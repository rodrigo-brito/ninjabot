package ninjabot

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/rodrigo-brito/ninjabot/backtest"
	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/notification"
	"github.com/rodrigo-brito/ninjabot/order"
	"github.com/rodrigo-brito/ninjabot/service"
	"github.com/rodrigo-brito/ninjabot/storage"
	"github.com/rodrigo-brito/ninjabot/strategy"
	"github.com/rodrigo-brito/ninjabot/tools/log"
	"github.com/rodrigo-brito/ninjabot/tools/metrics"

	"github.com/aybabtme/uniplot/histogram"
	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
	"github.com/schollz/progressbar/v3"
)

const defaultDatabase = "ninjabot.db"

func init() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04",
	})
}

type OrderSubscriber interface {
	OnOrder(model.Order)
}

type CandleSubscriber interface {
	OnCandle(model.Candle)
}

type NinjaBot struct {
	storage  storage.Storage
	settings model.Settings
	exchange service.Exchange
	strategy strategy.Strategy
	notifier service.Notifier
	telegram service.Telegram

	orderController       *order.Controller
	priorityQueueCandle   *model.PriorityQueue
	strategiesControllers map[string]*strategy.Controller
	orderFeed             *order.Feed
	dataFeed              *exchange.DataFeedSubscription
	paperWallet           *exchange.PaperWallet
	candleSubscribers     []CandleSubscriber
	controllerOptions     []strategy.ControllerOption

	backtest       bool
	backtestResult *backtest.Result
}

type Option func(*NinjaBot)

func NewBot(ctx context.Context, settings model.Settings, exch service.Exchange, str strategy.Strategy,
	options ...Option) (*NinjaBot, error) {

	bot := &NinjaBot{
		settings:              settings,
		exchange:              exch,
		strategy:              str,
		orderFeed:             order.NewOrderFeed(),
		dataFeed:              exchange.NewDataFeed(exch),
		strategiesControllers: make(map[string]*strategy.Controller),
		priorityQueueCandle:   model.NewPriorityQueue(nil),
	}

	for _, pair := range settings.Pairs {
		asset, quote := exchange.SplitAssetQuote(pair)
		if asset == "" || quote == "" {
			return nil, fmt.Errorf("invalid pair: %s", pair)
		}
	}

	for _, option := range options {
		option(bot)
	}

	var err error
	if bot.storage == nil {
		bot.storage, err = storage.FromFile(defaultDatabase)
		if err != nil {
			return nil, err
		}
	}

	if bot.backtest {
		// keep orders in step with candles: subscribers receive them inline
		bot.orderFeed.SetSynchronous(true)
	}
	bot.orderController = order.NewController(ctx, exch, bot.storage, bot.orderFeed)

	if settings.Telegram.Enabled {
		bot.telegram, err = notification.NewTelegram(bot.orderController, settings)
		if err != nil {
			return nil, err
		}
		// register telegram as notifier
		bot.notifier = bot.telegram
	}

	// the notifier, from WithNotifier or from telegram, can only be wired once
	// the order controller exists
	if bot.notifier != nil {
		bot.orderController.SetNotifier(bot.notifier)
		bot.SubscribeOrder(bot.notifier)
	}

	return bot, nil
}

// WithBacktest sets the bot to run in backtest mode, it is required for backtesting environments
// Backtest mode optimize the input read for CSV and deal with race conditions
func WithBacktest(wallet *exchange.PaperWallet) Option {
	return func(bot *NinjaBot) {
		bot.backtest = true
		opt := WithPaperWallet(wallet)
		opt(bot)
	}
}

// WithStorage sets the storage for the bot, by default it uses a local file called ninjabot.db
func WithStorage(storage storage.Storage) Option {
	return func(bot *NinjaBot) {
		bot.storage = storage
	}
}

// WithLogLevel sets the log level. eg: log.DebugLevel, log.InfoLevel, log.WarnLevel, log.ErrorLevel, log.FatalLevel
func WithLogLevel(level log.Level) Option {
	return func(_ *NinjaBot) {
		log.SetLevel(level)
	}
}

// WithNotifier registers a notifier to the bot, currently only email and telegram are supported.
// The notifier is wired to the order controller by NewBot, once it exists.
func WithNotifier(notifier service.Notifier) Option {
	return func(bot *NinjaBot) {
		bot.notifier = notifier
	}
}

// WithStrategyOptions customizes the strategy controller of every pair, e.g.
// strategy.WithMaxHistory to bound the memory of a long-running bot.
func WithStrategyOptions(options ...strategy.ControllerOption) Option {
	return func(bot *NinjaBot) {
		bot.controllerOptions = append(bot.controllerOptions, options...)
	}
}

// WithCandleSubscription subscribes a given struct to the candle feed
func WithCandleSubscription(subscriber CandleSubscriber) Option {
	return func(bot *NinjaBot) {
		bot.SubscribeCandle(subscriber)
	}
}

// WithPaperWallet sets the paper wallet for the bot (used for backtesting and live simulation)
func WithPaperWallet(wallet *exchange.PaperWallet) Option {
	return func(bot *NinjaBot) {
		bot.paperWallet = wallet
	}
}

func (n *NinjaBot) SubscribeCandle(subscriptions ...CandleSubscriber) {
	n.candleSubscribers = append(n.candleSubscribers, subscriptions...)
	for _, pair := range n.settings.Pairs {
		for _, subscription := range subscriptions {
			n.dataFeed.Subscribe(pair, n.strategy.Timeframe(), subscription.OnCandle, false)
		}
	}
}

func WithOrderSubscription(subscriber OrderSubscriber) Option {
	return func(bot *NinjaBot) {
		bot.SubscribeOrder(subscriber)
	}
}

func (n *NinjaBot) SubscribeOrder(subscriptions ...OrderSubscriber) {
	for _, pair := range n.settings.Pairs {
		for _, subscription := range subscriptions {
			n.orderFeed.Subscribe(pair, subscription.OnOrder, false)
		}
	}
}

func (n *NinjaBot) Controller() *order.Controller {
	return n.orderController
}

// Summary function displays all trades, accuracy and some bot metrics in stdout
// To access the raw data, you may access `bot.Controller().Results`
func (n *NinjaBot) Summary() {
	var (
		total  float64
		wins   int
		loses  int
		volume float64
		sqn    float64
	)

	buffer := bytes.NewBuffer(nil)
	table := tablewriter.NewTable(buffer,
		tablewriter.WithRenderer(renderer.NewBlueprint(tw.Rendition{
			Symbols: tw.NewSymbols(tw.StyleASCII),
		})),
		tablewriter.WithHeaderAutoFormat(tw.Off),
		tablewriter.WithHeaderAlignment(tw.AlignCenter),
		tablewriter.WithRowAlignmentConfig(tw.CellAlignment{
			PerColumn: []tw.Align{
				tw.AlignLeft, tw.AlignRight, tw.AlignRight, tw.AlignRight, tw.AlignRight,
				tw.AlignRight, tw.AlignRight, tw.AlignRight, tw.AlignRight, tw.AlignRight,
			},
		}),
		tablewriter.WithFooterAlignmentConfig(tw.CellAlignment{Global: tw.AlignRight}),
	)
	table.Header([]string{"PAIR", "TRADES", "WIN", "LOSS", "% WIN", "PAYOFF", "PR FACT.", "SQN", "PROFIT", "VOLUME"})
	avgPayoff := 0.0
	avgProfitFactor := 0.0

	// Pre-calculate total returns size to avoid reallocations
	totalReturns := 0
	for _, summary := range n.orderController.Results {
		totalReturns += len(summary.WinPercent()) + len(summary.LosePercent())
	}
	returns := make([]float64, 0, totalReturns)

	for _, summary := range n.orderController.Results {
		avgPayoff += summary.Payoff() * float64(len(summary.Win())+len(summary.Lose()))
		avgProfitFactor += summary.ProfitFactor() * float64(len(summary.Win())+len(summary.Lose()))
		err := table.Append([]string{
			summary.Pair,
			strconv.Itoa(len(summary.Win()) + len(summary.Lose())),
			strconv.Itoa(len(summary.Win())),
			strconv.Itoa(len(summary.Lose())),
			fmt.Sprintf("%.1f %%", summary.WinPercentage()),
			fmt.Sprintf("%.3f", summary.Payoff()),
			fmt.Sprintf("%.3f", summary.ProfitFactor()),
			fmt.Sprintf("%.1f", summary.SQN()),
			fmt.Sprintf("%.2f", summary.Profit()),
			fmt.Sprintf("%.2f", summary.Volume),
		})
		if err != nil {
			log.Error(err)
		}

		total += summary.Profit()
		sqn += summary.SQN()
		wins += len(summary.Win())
		loses += len(summary.Lose())
		volume += summary.Volume

		returns = append(returns, summary.WinPercent()...)
		returns = append(returns, summary.LosePercent()...)
	}

	// averages over an empty run are reported as 0 instead of NaN
	trades := math.Max(float64(wins+loses), 1)
	pairs := math.Max(float64(len(n.orderController.Results)), 1)
	table.Footer([]string{
		"TOTAL",
		strconv.Itoa(wins + loses),
		strconv.Itoa(wins),
		strconv.Itoa(loses),
		fmt.Sprintf("%.1f %%", float64(wins)/trades*100),
		fmt.Sprintf("%.3f", avgPayoff/trades),
		fmt.Sprintf("%.3f", avgProfitFactor/trades),
		fmt.Sprintf("%.1f", sqn/pairs),
		fmt.Sprintf("%.2f", total),
		fmt.Sprintf("%.2f", volume),
	})
	if err := table.Render(); err != nil {
		log.Error(err)
	}

	fmt.Println(buffer.String())
	fmt.Println("------ RETURN -------")
	totalReturn := 0.0
	returnsPercent := make([]float64, 0, len(returns))
	for _, p := range returns {
		returnsPercent = append(returnsPercent, p*100)
		totalReturn += p
	}
	hist := histogram.Hist(15, returnsPercent)
	histogram.Fprint(os.Stdout, hist, histogram.Linear(10))
	fmt.Println()

	fmt.Println("------ CONFIDENCE INTERVAL (95%) -------")
	for pair, summary := range n.orderController.Results {
		fmt.Printf("| %s |\n", pair)
		returns := append(summary.WinPercent(), summary.LosePercent()...)
		returnsInterval := metrics.Bootstrap(returns, metrics.Mean, 10000, 0.95)
		payoffInterval := metrics.Bootstrap(returns, metrics.Payoff, 10000, 0.95)
		profitFactorInterval := metrics.Bootstrap(returns, metrics.ProfitFactor, 10000, 0.95)

		fmt.Printf("RETURN:      %.2f%% (%.2f%% ~ %.2f%%)\n",
			returnsInterval.Mean*100, returnsInterval.Lower*100, returnsInterval.Upper*100)
		fmt.Printf("PAYOFF:      %.2f (%.2f ~ %.2f)\n",
			payoffInterval.Mean, payoffInterval.Lower, payoffInterval.Upper)
		fmt.Printf("PROF.FACTOR: %.2f (%.2f ~ %.2f)\n",
			profitFactorInterval.Mean, profitFactorInterval.Lower, profitFactorInterval.Upper)
	}

	fmt.Println()

	if n.paperWallet != nil {
		n.paperWallet.Summary()
	}

}

func (n NinjaBot) SaveReturns(outputDir string) error {
	for _, summary := range n.orderController.Results {
		outputFile := fmt.Sprintf("%s/%s.csv", outputDir, summary.Pair)
		if err := summary.SaveReturns(outputFile); err != nil {
			return err
		}
	}
	return nil
}

func (n *NinjaBot) onCandle(candle model.Candle) {
	n.priorityQueueCandle.Push(candle)
}

func (n *NinjaBot) processCandle(candle model.Candle) {
	if n.paperWallet != nil {
		n.paperWallet.OnCandle(candle)
	}

	n.strategiesControllers[candle.Pair].OnPartialCandle(candle)
	if candle.Complete {
		n.strategiesControllers[candle.Pair].OnCandle(candle)
		n.orderController.OnCandle(candle)
	}
}

// Process pending candles in buffer
func (n *NinjaBot) processCandles() {
	for item := range n.priorityQueueCandle.PopLock() {
		n.processCandle(item.(model.Candle))
	}
}

// Before Ninjabot start, we need to load the necessary data to fill strategy indicators
// Then, we need to get the time frame and warmup period to fetch the necessary candles
func (n *NinjaBot) preload(ctx context.Context, pair string) error {
	candles, err := n.exchange.CandlesByLimit(ctx, pair, n.strategy.Timeframe(), n.strategy.WarmupPeriod())
	if err != nil {
		return err
	}

	for _, candle := range candles {
		n.processCandle(candle)
	}

	n.dataFeed.Preload(pair, n.strategy.Timeframe(), candles)

	return nil
}

// Result returns the outcome of the last backtest run, or nil when the bot
// is not in backtest mode or has not run yet.
func (n *NinjaBot) Result() *backtest.Result {
	return n.backtestResult
}

// observeIndicators registers the candle subscribers that want the plotted
// indicators of the strategy (the dashboard) with the controller of a pair.
func (n *NinjaBot) observeIndicators(controller *strategy.Controller) {
	for _, subscriber := range n.candleSubscribers {
		if observer, ok := subscriber.(strategy.IndicatorObserver); ok {
			controller.Subscribe(observer)
		}
	}
}

// runBacktest replays the candles of the paper wallet's feed with the
// deterministic backtest engine.
func (n *NinjaBot) runBacktest(ctx context.Context) error {
	if n.paperWallet == nil {
		return fmt.Errorf("backtest mode requires a paper wallet")
	}

	timeframe := n.strategy.Timeframe()
	end := time.Date(9999, time.December, 31, 0, 0, 0, 0, time.UTC)
	streams := make([][]model.Candle, 0, len(n.settings.Pairs))
	total := 0
	for _, pair := range n.settings.Pairs {
		n.strategiesControllers[pair] = strategy.NewStrategyController(pair, n.strategy, n.orderController, n.controllerOptions...)
		n.observeIndicators(n.strategiesControllers[pair])
		candles, err := n.exchange.CandlesByPeriod(ctx, pair, timeframe, time.Time{}, end)
		if err != nil {
			return err
		}
		streams = append(streams, candles)
		total += len(candles)
	}

	subscribers := make([]func(model.Candle), len(n.candleSubscribers))
	for i, subscriber := range n.candleSubscribers {
		subscribers[i] = subscriber.OnCandle
	}

	log.Info("[SETUP] Starting backtesting")
	progressBar := progressbar.Default(int64(total))
	engine := backtest.NewWithComponents(backtest.Components{
		Wallet:            n.paperWallet,
		Orders:            n.orderController,
		Controllers:       n.strategiesControllers,
		Streams:           streams,
		CandleSubscribers: subscribers,
		// the robustness reports cost more than the run itself; the dashboard
		// and the CLI compute them on demand
		NoStatistics: true,
		Config: backtest.ConfigEcho{
			Strategy:  fmt.Sprintf("%T", n.strategy),
			Pairs:     n.settings.Pairs,
			Timeframe: timeframe,
		},
		Progress: func(done, _ int) {
			if err := progressBar.Set(done); err != nil {
				log.Warnf("update progressbar fail: %v", err)
			}
		},
	})

	if n.telegram != nil {
		n.telegram.Start()
	}

	result, err := engine.Run(ctx)
	if err != nil {
		return err
	}
	n.backtestResult = result
	return nil
}

// Run will initialize the strategy controller, order controller, preload data and start the bot
func (n *NinjaBot) Run(ctx context.Context) error {
	if n.backtest {
		return n.runBacktest(ctx)
	}

	for _, pair := range n.settings.Pairs {
		// setup and subscribe strategy to data feed (candles)
		n.strategiesControllers[pair] = strategy.NewStrategyController(pair, n.strategy, n.orderController, n.controllerOptions...)
		n.observeIndicators(n.strategiesControllers[pair])

		// preload candles for warmup period
		err := n.preload(ctx, pair)
		if err != nil {
			return err
		}

		// link to ninja bot controller
		n.dataFeed.Subscribe(pair, n.strategy.Timeframe(), n.onCandle, false)

		// start strategy controller
		n.strategiesControllers[pair].Start()
	}

	// start order feed and controller
	n.orderFeed.Start()
	n.orderController.Start()
	defer n.orderController.Stop()
	if n.telegram != nil {
		n.telegram.Start()
	}

	// start data feed and process new candles
	n.dataFeed.Start(false)
	n.processCandles()

	return nil
}
