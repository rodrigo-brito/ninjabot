package ninjabot

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/notification"
	"github.com/rodrigo-brito/ninjabot/order"
	"github.com/rodrigo-brito/ninjabot/service"
	"github.com/rodrigo-brito/ninjabot/storage"
	"github.com/rodrigo-brito/ninjabot/strategy"

	"github.com/olekukonko/tablewriter"
	log "github.com/sirupsen/logrus"
)

const (
	defaultDatabase = "ninjabot.db"
)

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

	backtest       bool
	pendingCandles int64
	startBacktest  sync.WaitGroup
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

	if bot.orderController == nil {
		bot.orderController = order.NewController(ctx, exch, bot.storage, bot.orderFeed, bot.notifier)
	}

	if settings.Telegram.Enabled {
		bot.telegram, err = notification.NewTelegram(bot.orderController, settings)
		if err != nil {
			return nil, err
		}
		// register telegram as notifier
		WithNotifier(bot.telegram)(bot)
	}

	return bot, nil
}

func WithBacktest(wallet *exchange.PaperWallet) Option {
	return func(bot *NinjaBot) {
		bot.backtest = true
		bot.startBacktest.Add(1)

		// load paper wallet
		opt := WithPaperWallet(wallet)
		opt(bot)
	}
}

func WithStorage(storage storage.Storage) Option {
	return func(bot *NinjaBot) {
		bot.storage = storage
	}
}

func WithLogLevel(level log.Level) Option {
	return func(bot *NinjaBot) {
		log.SetLevel(level)
	}
}

func WithNotifier(notifier service.Notifier) Option {
	return func(bot *NinjaBot) {
		bot.notifier = notifier
		bot.SubscribeOrder(notifier)
	}
}

func WithCandleSubscription(subscriber CandleSubscriber) Option {
	return func(bot *NinjaBot) {
		bot.SubscribeCandle(subscriber)
	}
}

func WithPaperWallet(wallet *exchange.PaperWallet) Option {
	return func(bot *NinjaBot) {
		bot.paperWallet = wallet
	}
}

func (n *NinjaBot) SubscribeCandle(subscriptions ...CandleSubscriber) {
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

func (n *NinjaBot) Summary() string {
	var (
		total  float64
		wins   int
		loses  int
		volume float64
	)

	buffer := bytes.NewBuffer(nil)
	table := tablewriter.NewWriter(buffer)
	table.SetHeader([]string{"Pair", "Trades", "Win", "Loss", "% Win", "Payoff", "Profit", "Volume"})
	table.SetFooterAlignment(tablewriter.ALIGN_RIGHT)
	avgPayoff := 0.0

	for _, summary := range n.orderController.Results {
		avgPayoff += summary.Payoff() * float64(len(summary.Win)+len(summary.Lose))
		table.Append([]string{
			summary.Pair,
			strconv.Itoa(len(summary.Win) + len(summary.Lose)),
			strconv.Itoa(len(summary.Win)),
			strconv.Itoa(len(summary.Lose)),
			fmt.Sprintf("%.1f %%", float64(len(summary.Win))/float64(len(summary.Win)+len(summary.Lose))*100),
			fmt.Sprintf("%.3f", summary.Payoff()),
			fmt.Sprintf("%.2f", summary.Profit()),
			fmt.Sprintf("%.2f", summary.Volume),
		})
		total += summary.Profit()
		wins += len(summary.Win)
		loses += len(summary.Lose)
		volume += summary.Volume
	}

	table.SetFooter([]string{
		"TOTAL",
		strconv.Itoa(wins + loses),
		strconv.Itoa(wins),
		strconv.Itoa(loses),
		fmt.Sprintf("%.1f %%", float64(wins)/float64(wins+loses)*100),
		fmt.Sprintf("%.3f", avgPayoff/float64(wins+loses)),
		fmt.Sprintf("%.2f", total),
		fmt.Sprintf("%.2f", volume),
	})
	table.Render()

	return buffer.String()
}

func (n *NinjaBot) onCandle(candle model.Candle) {
	n.priorityQueueCandle.Push(candle)
	atomic.AddInt64(&n.pendingCandles, 1)
}

func (n *NinjaBot) processCandles() {
	// when backtesting, we need to wait all candles load
	// to avoid sync issues between multiple coins
	if n.backtest {
		n.startBacktest.Wait()
	}

	for atomic.AddInt64(&n.pendingCandles, -1) >= 0 {
		item := n.priorityQueueCandle.Pop()

		candle := item.(model.Candle)
		if n.paperWallet != nil {
			n.paperWallet.OnCandle(candle)
		}

		if candle.Complete {
			n.strategiesControllers[candle.Pair].OnCandle(candle)
		}
	}
}

func (n *NinjaBot) Run(ctx context.Context) error {
	for _, pair := range n.settings.Pairs {
		pair := pair
		// setup and subscribe strategy to data feed (candles)
		strategyController := strategy.NewStrategyController(pair, n.strategy, n.orderController)
		strategyController.Start()
		n.strategiesControllers[pair] = strategyController

		// link to ninja bot controller
		n.dataFeed.Subscribe(pair, n.strategy.Timeframe(), n.onCandle, false)

		if !n.backtest {
			// preload candles to warmup strategy
			candles, err := n.exchange.CandlesByLimit(ctx, pair, n.strategy.Timeframe(), n.strategy.WarmupPeriod()+1)
			if err != nil {
				return err
			}
			n.dataFeed.Preload(pair, n.strategy.Timeframe(), candles)
		}
	}

	n.orderFeed.Start()
	n.orderController.Start()
	defer n.orderController.Stop()
	if n.telegram != nil {
		n.telegram.Start()
	}

	n.dataFeed.OnFinish(func() {
		if n.backtest {
			n.startBacktest.Done()
		}
	})
	go n.dataFeed.Start()
	n.processCandles()
	return nil
}
