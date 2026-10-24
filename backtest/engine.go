package backtest

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/order"
	"github.com/rodrigo-brito/ninjabot/storage"
	"github.com/rodrigo-brito/ninjabot/strategy"
)

// Config describes one backtest run.
type Config struct {
	// Pairs to trade. Defaults to every pair of the dataset.
	Pairs []string
	// Strategy to run. The same instance receives the candles of every pair,
	// so parallel runs need one instance each.
	Strategy strategy.Strategy
	// Timeframe of the candles fed to the strategy. Defaults to
	// Strategy.Timeframe().
	Timeframe string
	// Quote is the currency the wallet is valued in, e.g. "USDT".
	Quote string
	// Balances is the initial wallet, e.g. {"USDT": 10000}.
	Balances map[string]float64
	// MakerFee and TakerFee are charged on every fill, as a fraction of the
	// traded volume.
	MakerFee, TakerFee float64
	// Storage keeps the orders. Defaults to storage.NewMemory().
	Storage storage.Storage
	// Progress, when set, is called every progressStep candles and at the end.
	Progress func(done, total int)
	// CandleSubscribers receive every candle after the strategy, in order.
	CandleSubscribers []func(model.Candle)
	// OrderSubscribers receive every order event, in order.
	OrderSubscribers []func(model.Order)
	// NoStatistics skips the robustness reports of the result, which cost a
	// few thousand resamples per pair. Sweeps that only rank runs set it.
	NoStatistics bool
	// ControllerOptions customize the strategy controllers, e.g.
	// strategy.WithMaxHistory.
	ControllerOptions []strategy.ControllerOption
	// IndicatorObservers receive the plotted indicators of the strategy,
	// e.g. the dashboard.
	IndicatorObservers []strategy.IndicatorObserver
	// Seed of the randomized statistics; metrics.DefaultSeed when zero.
	Seed int64
	// TradeFrom, when set, makes the candles before it a warmup: they feed
	// the indicators of the strategy only. Trading, the wallet and every
	// measure of the result start at TradeFrom.
	TradeFrom time.Time
}

// completeCount is the number of closed candles of a stream: the size of
// the history a strategy controller will hold.
func completeCount(stream []model.Candle) int {
	count := 0
	for i := range stream {
		if stream[i].Complete {
			count++
		}
	}
	return count
}

// progressStep is how often, in candles, Config.Progress is called.
const progressStep = 1000

// Components are the parts of a run. A NinjaBot in backtest mode builds them
// itself and hands them to NewWithComponents.
type Components struct {
	Wallet      *exchange.PaperWallet
	Orders      *order.Controller
	Controllers map[string]*strategy.Controller // by pair
	// Streams are the candles of every pair, each sorted by time.
	Streams           [][]model.Candle
	CandleSubscribers []func(model.Candle)
	Progress          func(done, total int)
	// Config is echoed in the result.
	Config ConfigEcho
	// NoStatistics skips the robustness reports of the result.
	NoStatistics bool
	// TradeFrom ends the warmup, see Config.TradeFrom.
	TradeFrom time.Time
	// Seed of the randomized statistics.
	Seed int64
}

// Engine runs a strategy over a dataset in a single goroutine: no channels,
// no timers, no shared state with other engines. The same inputs always
// produce the same result.
type Engine struct {
	c      Components
	feeder *feeder
}

// New prepares a run of cfg over the dataset.
func New(ds *Dataset, cfg Config) (*Engine, error) {
	if cfg.Strategy == nil {
		return nil, errors.New("backtest: missing strategy")
	}
	if cfg.Quote == "" {
		return nil, errors.New("backtest: missing quote currency")
	}
	if len(cfg.Pairs) == 0 {
		cfg.Pairs = ds.Pairs()
	}
	if cfg.Timeframe == "" {
		cfg.Timeframe = cfg.Strategy.Timeframe()
	}
	if cfg.Storage == nil {
		cfg.Storage = storage.NewMemory()
	}

	streams := make([][]model.Candle, 0, len(cfg.Pairs))
	for _, pair := range cfg.Pairs {
		asset, quote := exchange.SplitAssetQuote(pair)
		if asset == "" || quote == "" {
			return nil, fmt.Errorf("backtest: invalid pair %s", pair)
		}
		if quote != cfg.Quote {
			return nil, fmt.Errorf("backtest: pair %s is not quoted in %s", pair, cfg.Quote)
		}
		candles, err := ds.Candles(pair, cfg.Timeframe)
		if err != nil {
			return nil, err
		}
		streams = append(streams, candles)
	}

	f := newFeeder(ds)
	options := []exchange.PaperWalletOption{
		exchange.WithPaperFee(cfg.MakerFee, cfg.TakerFee),
		exchange.WithDataFeed(f),
	}
	assets := make([]string, 0, len(cfg.Balances))
	for asset := range cfg.Balances {
		assets = append(assets, asset)
	}
	sort.Strings(assets)
	for _, asset := range assets {
		options = append(options, exchange.WithPaperAsset(asset, cfg.Balances[asset]))
	}
	ctx := context.Background()
	wallet := exchange.NewPaperWallet(ctx, cfg.Quote, options...)

	feed := order.NewSyncFeed()
	for _, pair := range cfg.Pairs {
		for _, subscriber := range cfg.OrderSubscribers {
			feed.Subscribe(pair, subscriber, false)
		}
	}
	orders := order.NewController(ctx, wallet, cfg.Storage, feed)

	controllers := make(map[string]*strategy.Controller, len(cfg.Pairs))
	for i, pair := range cfg.Pairs {
		options := append([]strategy.ControllerOption{strategy.WithCapacity(completeCount(streams[i]))},
			cfg.ControllerOptions...)
		controllers[pair] = strategy.NewStrategyController(pair, cfg.Strategy, orders, options...)
		for _, observer := range cfg.IndicatorObservers {
			controllers[pair].Subscribe(observer)
		}
	}

	engine := NewWithComponents(Components{
		Wallet:            wallet,
		Orders:            orders,
		Controllers:       controllers,
		Streams:           streams,
		CandleSubscribers: cfg.CandleSubscribers,
		Progress:          cfg.Progress,
		NoStatistics:      cfg.NoStatistics,
		TradeFrom:         cfg.TradeFrom,
		Seed:              cfg.Seed,
		Config: ConfigEcho{
			Strategy:  strategyName(cfg.Strategy),
			Pairs:     sortedPairs(cfg.Pairs),
			Timeframe: cfg.Timeframe,
			Quote:     cfg.Quote,
			Balances:  cfg.Balances,
			MakerFee:  cfg.MakerFee,
			TakerFee:  cfg.TakerFee,
		},
	})
	engine.feeder = f
	return engine, nil
}

// NewWithComponents creates an engine from parts built elsewhere. The order
// controller must not be started: the engine syncs it after every fill.
func NewWithComponents(c Components) *Engine {
	return &Engine{c: c}
}

// Wallet returns the paper wallet of the run.
func (e *Engine) Wallet() *exchange.PaperWallet { return e.c.Wallet }

// Orders returns the order controller of the run.
func (e *Engine) Orders() *order.Controller { return e.c.Orders }

// Run replays every candle in chronological order and returns the result.
// Candles of different pairs with the same time are ordered by update time
// and then by pair, like model.Candle.Less.
func (e *Engine) Run(ctx context.Context) (*Result, error) {
	trading := e.c.TradeFrom.IsZero()
	if trading {
		for _, controller := range e.c.Controllers {
			controller.Start()
		}
	}

	total := 0
	for _, stream := range e.c.Streams {
		total += len(stream)
	}

	cursor := make([]int, len(e.c.Streams))
	var first, last time.Time
	for done := 0; done < total; done++ {
		if done%progressStep == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if e.c.Progress != nil && done > 0 {
				e.c.Progress(done, total)
			}
		}

		// k-way merge: the smallest head among the streams
		best := -1
		for i, stream := range e.c.Streams {
			if cursor[i] >= len(stream) {
				continue
			}
			if best < 0 || stream[cursor[i]].Less(e.c.Streams[best][cursor[best]]) {
				best = i
			}
		}
		candle := e.c.Streams[best][cursor[best]]
		cursor[best]++

		if !trading {
			if candle.Time.Before(e.c.TradeFrom) {
				// warmup: indicators only, no trading, nothing measured
				if controller, ok := e.c.Controllers[candle.Pair]; ok && candle.Complete {
					controller.OnCandle(candle)
				}
				continue
			}
			trading = true
			for _, controller := range e.c.Controllers {
				controller.Start()
			}
		}

		if first.IsZero() {
			first = candle.Time
		}
		last = candle.Time
		e.step(candle)
	}

	// settle anything still reported by the wallet
	e.c.Wallet.TakeFills()
	e.c.Orders.Sync()

	if e.c.Progress != nil {
		e.c.Progress(total, total)
	}

	if !e.c.TradeFrom.IsZero() {
		e.c.Streams = clipStreams(e.c.Streams, e.c.TradeFrom)
		total = 0
		for _, stream := range e.c.Streams {
			total += len(stream)
		}
	}
	return e.result(first, last, total)
}

// clipStreams drops the candles before from: the warmup is not part of the
// result.
func clipStreams(streams [][]model.Candle, from time.Time) [][]model.Candle {
	clipped := make([][]model.Candle, len(streams))
	for i, stream := range streams {
		start := sort.Search(len(stream), func(j int) bool { return !stream[j].Time.Before(from) })
		clipped[i] = stream[start:]
	}
	return clipped
}

// step processes one candle.
func (e *Engine) step(candle model.Candle) {
	if e.feeder != nil {
		e.feeder.observe(candle)
	}

	e.c.Wallet.OnCandle(candle)
	if fills := e.c.Wallet.TakeFills(); len(fills) > 0 {
		e.c.Orders.Sync()
	}

	if controller, ok := e.c.Controllers[candle.Pair]; ok {
		controller.OnPartialCandle(candle)
		if candle.Complete {
			controller.OnCandle(candle)
		}
	}
	if candle.Complete {
		e.c.Orders.OnCandle(candle)
	}

	for _, subscriber := range e.c.CandleSubscribers {
		subscriber(candle)
	}
}
