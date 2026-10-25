package backtest

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	str2duration "github.com/xhit/go-str2duration/v2"

	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/order"
	"github.com/rodrigo-brito/ninjabot/storage"
	"github.com/rodrigo-brito/ninjabot/strategy"
)

// SharedStrategy is one strategy trading a shared account.
type SharedStrategy struct {
	// Tag names the strategy; it is stamped on its orders and trades.
	Tag      string
	Strategy strategy.Strategy
	Pairs    []string
	// Timeframe defaults to Strategy.Timeframe().
	Timeframe string
	// Budget is the cash attributed to the strategy: it sees it as its
	// quote balance, moved by its own fills.
	Budget float64
}

// SharedConfig describes strategies trading one paper wallet together.
type SharedConfig struct {
	Quote              string
	Capital            float64
	MakerFee, TakerFee float64
	Strategies         []SharedStrategy
	NoStatistics       bool
	Progress           func(done, total int)
}

// TagResult is the share of one strategy in a shared run.
type TagResult struct {
	Tag    string  `json:"tag"`
	Result *Result `json:"result"`
}

// SharedResult is the outcome of a shared run: the whole account and the
// attribution to every strategy.
type SharedResult struct {
	Account *Result     `json:"account"`
	Tags    []TagResult `json:"tags"`
}

// stream is a candle series of a pair in a timeframe, with the controllers
// that consume it.
type stream struct {
	pair, timeframe string
	candles         []model.Candle
	pricing         bool // the finest timeframe of the pair prices the wallet
	controllers     []*strategy.Controller
}

// RunShared runs several strategies on one paper wallet. Each strategy
// trades through a broker scoped to its tag, so its orders, positions and
// results are attributed to it while the cash and the fills are shared.
//
// The wallet is priced by the finest timeframe traded on each pair: limit and
// stop orders fill on those candles.
func RunShared(ctx context.Context, ds *Dataset, cfg SharedConfig) (*SharedResult, error) {
	if cfg.Quote == "" || cfg.Capital <= 0 || len(cfg.Strategies) == 0 {
		return nil, errors.New("backtest: a shared run needs a quote, a positive capital and strategies")
	}

	f := newFeeder(ds)
	wallet := exchange.NewPaperWallet(context.Background(), cfg.Quote,
		exchange.WithPaperAsset(cfg.Quote, cfg.Capital),
		exchange.WithPaperFee(cfg.MakerFee, cfg.TakerFee),
		exchange.WithDataFeed(f),
	)
	orders := order.NewController(context.Background(), wallet, storage.NewMemory(), order.NewSyncFeed())

	streams := map[string]*stream{}
	finest := map[string]time.Duration{}
	tags := make([]string, 0, len(cfg.Strategies))
	seen := map[string]bool{}
	for _, s := range cfg.Strategies {
		if s.Tag == "" || seen[s.Tag] {
			return nil, fmt.Errorf("backtest: strategy tag %q is empty or repeated", s.Tag)
		}
		seen[s.Tag] = true
		tags = append(tags, s.Tag)
		orders.SetBudget(s.Tag, s.Budget)

		timeframe := s.Timeframe
		if timeframe == "" {
			timeframe = s.Strategy.Timeframe()
		}
		duration, err := str2duration.ParseDuration(timeframe)
		if err != nil {
			return nil, fmt.Errorf("backtest: %s: %w", s.Tag, err)
		}
		pairs := s.Pairs
		if len(pairs) == 0 {
			pairs = ds.Pairs()
		}
		broker := orders.Scoped(s.Tag)
		for _, pair := range pairs {
			if _, quote := exchange.SplitAssetQuote(pair); quote != cfg.Quote {
				return nil, fmt.Errorf("backtest: pair %s is not quoted in %s", pair, cfg.Quote)
			}
			key := pair + "--" + timeframe
			st, ok := streams[key]
			if !ok {
				candles, err := ds.Candles(pair, timeframe)
				if err != nil {
					return nil, err
				}
				st = &stream{pair: pair, timeframe: timeframe, candles: candles}
				streams[key] = st
			}
			st.controllers = append(st.controllers, strategy.NewStrategyController(pair, s.Strategy, broker))
			if current, ok := finest[pair]; !ok || duration < current {
				finest[pair] = duration
			}
		}
	}

	ordered := make([]*stream, 0, len(streams))
	for _, st := range streams {
		duration, _ := str2duration.ParseDuration(st.timeframe)
		st.pricing = duration == finest[st.pair]
		ordered = append(ordered, st)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].pair != ordered[j].pair {
			return ordered[i].pair < ordered[j].pair
		}
		return ordered[i].timeframe < ordered[j].timeframe
	})

	run := &sharedRun{wallet: wallet, orders: orders, feeder: f, tags: tags, curves: map[string][]Point{}}
	if err := run.loop(ctx, ordered, cfg.Progress); err != nil {
		return nil, err
	}
	return run.results(cfg, ordered)
}

type sharedRun struct {
	wallet      *exchange.PaperWallet
	orders      *order.Controller
	feeder      *feeder
	tags        []string
	equity      []Point
	curves      map[string][]Point
	first, last time.Time
	candles     int
}

func appendPointOrReplace(points []Point, p Point) []Point {
	if n := len(points); n > 0 && points[n-1].Time == p.Time {
		points[n-1] = p
		return points
	}
	return append(points, p)
}

func (r *sharedRun) loop(ctx context.Context, streams []*stream, progress func(done, total int)) error {
	for _, st := range streams {
		for _, controller := range st.controllers {
			controller.Start()
		}
		r.candles += len(st.candles)
	}

	cursor := make([]int, len(streams))
	for done := 0; done < r.candles; done++ {
		if done%progressStep == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
			if progress != nil && done > 0 {
				progress(done, r.candles)
			}
		}

		best := -1
		for i, st := range streams {
			if cursor[i] >= len(st.candles) {
				continue
			}
			if best < 0 || st.candles[cursor[i]].Less(streams[best].candles[cursor[best]]) {
				best = i
			}
		}
		st := streams[best]
		candle := st.candles[cursor[best]]
		cursor[best]++
		if r.first.IsZero() {
			r.first = candle.Time
		}
		r.last = candle.Time

		if st.pricing {
			r.feeder.observe(candle)
			r.wallet.OnCandle(candle)
			if len(r.wallet.TakeFills()) > 0 {
				r.orders.Sync()
			}
			// the ledgers price holdings like the wallet: at the last candle,
			// partial or not
			r.orders.OnCandle(candle)
		}
		for _, controller := range st.controllers {
			controller.OnPartialCandle(candle)
			if candle.Complete {
				controller.OnCandle(candle)
			}
		}
		if st.pricing && candle.Complete {
			// sample the account and every tag at the same moment
			t := candle.Time.Unix()
			r.equity = appendPointOrReplace(r.equity, Point{Time: t, Value: r.wallet.Equity()})
			for _, tag := range r.tags {
				r.curves[tag] = appendPointOrReplace(r.curves[tag], Point{Time: t, Value: r.orders.TagEquity(tag)})
			}
		}
	}

	r.wallet.TakeFills()
	r.orders.Sync()
	if progress != nil {
		progress(r.candles, r.candles)
	}
	return nil
}

func (r *sharedRun) results(cfg SharedConfig, streams []*stream) (*SharedResult, error) {
	allOrders, err := r.orders.Orders()
	if err != nil {
		return nil, err
	}
	trades := r.orders.Trades()
	period := Period{Start: r.first, End: r.last, Candles: r.candles}

	pricing := make([][]model.Candle, 0)
	for _, st := range streams {
		if st.pricing {
			pricing = append(pricing, st.candles)
		}
	}

	pairsOf := func(tag string) ([]string, string, string) {
		pairs := map[string]bool{}
		var timeframe, name string
		for _, s := range cfg.Strategies {
			if s.Tag != tag {
				continue
			}
			name = strategyName(s.Strategy)
			timeframe = s.Timeframe
			if timeframe == "" {
				timeframe = s.Strategy.Timeframe()
			}
			for _, st := range streams {
				if st.timeframe == timeframe && (len(s.Pairs) == 0 || contains(s.Pairs, st.pair)) {
					pairs[st.pair] = true
				}
			}
		}
		out := make([]string, 0, len(pairs))
		for pair := range pairs {
			out = append(out, pair)
		}
		sort.Strings(out)
		return out, timeframe, name
	}

	allPairs := map[string]bool{}
	for _, st := range streams {
		allPairs[st.pair] = true
	}
	accountPairs := make([]string, 0, len(allPairs))
	for pair := range allPairs {
		accountPairs = append(accountPairs, pair)
	}
	sort.Strings(accountPairs)

	account := assemble(resultInput{
		Config: ConfigEcho{Strategy: "shared", Pairs: accountPairs, Quote: cfg.Quote,
			Balances: map[string]float64{cfg.Quote: cfg.Capital}, MakerFee: cfg.MakerFee, TakerFee: cfg.TakerFee,
			Timeframe: finestTimeframe(streams)},
		Period: period, Trades: trades, Orders: allOrders, Equity: r.equity,
		Initial: cfg.Capital, Final: r.wallet.Equity(), Fees: r.wallet.FeesPaid(),
		Volume: r.wallet.Volume(), Streams: pricing, NoStatistics: cfg.NoStatistics,
	})

	result := &SharedResult{Account: account}
	for _, s := range cfg.Strategies {
		pairs, timeframe, name := pairsOf(s.Tag)
		tagTrades := make([]order.Result, 0)
		for _, trade := range trades {
			if trade.Tag == s.Tag {
				tagTrades = append(tagTrades, trade)
			}
		}
		tagOrders := make([]model.Order, 0)
		for _, o := range allOrders {
			if o.Tag == s.Tag {
				tagOrders = append(tagOrders, o)
			}
		}
		tagStreams := make([][]model.Candle, 0)
		for _, st := range streams {
			if st.timeframe == timeframe && contains(pairs, st.pair) {
				tagStreams = append(tagStreams, st.candles)
			}
		}
		curve := r.curves[s.Tag]
		final := s.Budget
		if len(curve) > 0 {
			final = curve[len(curve)-1].Value
		}
		tag := assemble(resultInput{
			Config: ConfigEcho{Strategy: name, Pairs: pairs, Timeframe: timeframe, Quote: cfg.Quote,
				Balances: map[string]float64{cfg.Quote: s.Budget}, MakerFee: cfg.MakerFee, TakerFee: cfg.TakerFee},
			Period: period, Trades: tagTrades, Orders: tagOrders, Equity: curve,
			Initial: s.Budget, Final: final, Fees: r.orders.TagFees(s.Tag),
			Streams: tagStreams, NoStatistics: cfg.NoStatistics,
		})
		result.Tags = append(result.Tags, TagResult{Tag: s.Tag, Result: tag})
	}
	return result, nil
}

func finestTimeframe(streams []*stream) string {
	var best time.Duration
	timeframe := ""
	for _, st := range streams {
		duration, err := str2duration.ParseDuration(st.timeframe)
		if err == nil && (timeframe == "" || duration < best) {
			best, timeframe = duration, st.timeframe
		}
	}
	return timeframe
}

func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
