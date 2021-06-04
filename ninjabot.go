package ninjabot

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/olekukonko/tablewriter"

	"github.com/rodrigo-brito/ninjabot/pkg/notification"

	"github.com/rodrigo-brito/ninjabot/pkg/ent"
	"github.com/rodrigo-brito/ninjabot/pkg/exchange"
	"github.com/rodrigo-brito/ninjabot/pkg/model"
	"github.com/rodrigo-brito/ninjabot/pkg/order"
	"github.com/rodrigo-brito/ninjabot/pkg/storage"
	"github.com/rodrigo-brito/ninjabot/pkg/strategy"

	log "github.com/sirupsen/logrus"
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
	storage  *ent.Client
	settings model.Settings
	exchange exchange.Exchange
	strategy strategy.Strategy
	notifier notification.Notifier

	orderController *order.Controller
	orderFeed       *order.Feed
	dataFeed        *exchange.DataFeedSubscription
}

type Option func(*NinjaBot)

func NewBot(ctx context.Context, settings model.Settings, exch exchange.Exchange, str strategy.Strategy,
	options ...Option) (*NinjaBot, error) {

	bot := &NinjaBot{
		settings:  settings,
		exchange:  exch,
		strategy:  str,
		orderFeed: order.NewOrderFeed(),
		dataFeed:  exchange.NewDataFeed(exch),
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

	return bot, nil
}

func WithStorage(storage *ent.Client) Option {
	return func(bot *NinjaBot) {
		bot.storage = storage
	}
}

func WithLogLevel(level log.Level) Option {
	return func(bot *NinjaBot) {
		log.SetLevel(level)
	}
}

func WithNotifier(notifier notification.Notifier) Option {
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

func (n *NinjaBot) SubscribeCandle(subscriptions ...CandleSubscriber) {
	for _, symbol := range n.settings.Pairs {
		for _, subscription := range subscriptions {
			n.dataFeed.Subscribe(symbol, n.strategy.Timeframe(), subscription.OnCandle, false)
		}
	}
}

func WithOrderSubscription(subscriber OrderSubscriber) Option {
	return func(bot *NinjaBot) {
		bot.SubscribeOrder(subscriber)
	}
}

func (n *NinjaBot) SubscribeOrder(subscriptions ...OrderSubscriber) {
	for _, symbol := range n.settings.Pairs {
		for _, subscription := range subscriptions {
			n.orderFeed.Subscribe(symbol, subscription.OnOrder, false)
		}
	}
}

func (n *NinjaBot) Summary() {
	var (
		total float64
		wins  int
		loses int
	)
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Pair", "Trades", "Win", "Loss", "% Win", "Payoff", "Profit"})
	table.SetFooterAlignment(tablewriter.ALIGN_RIGHT)
	avgPayoff := 0.0
	for _, summary := range n.orderController.Results {
		avgPayoff += summary.Payoff() * float64(len(summary.Win)+len(summary.Lose))
		table.Append([]string{
			summary.Symbol,
			strconv.Itoa(len(summary.Win) + len(summary.Lose)),
			strconv.Itoa(len(summary.Win)),
			strconv.Itoa(len(summary.Lose)),
			fmt.Sprintf("%.1f %%", float64(len(summary.Win))/float64(len(summary.Win)+len(summary.Lose))*100),
			fmt.Sprintf("%.3f", summary.Payoff()),
			fmt.Sprintf("%.4f", summary.Profit()),
		})
		total += summary.Profit()
		wins += len(summary.Win)
		loses += len(summary.Lose)
	}
	table.SetFooter([]string{
		"TOTAL",
		strconv.Itoa(wins + loses),
		strconv.Itoa(wins),
		strconv.Itoa(loses),
		fmt.Sprintf("%.1f %%", float64(wins)/float64(wins+loses)*100),
		fmt.Sprintf("%.3f", avgPayoff/float64(wins+loses)),
		fmt.Sprintf("%.4f", total),
	})
	table.Render()
}

func (n *NinjaBot) Run(ctx context.Context) error {
	for _, pair := range n.settings.Pairs {
		// setup and subscribe strategy to data feed (candles)
		strategyController := strategy.NewStrategyController(pair, n.settings, n.strategy, n.orderController)
		n.dataFeed.Subscribe(pair, n.strategy.Timeframe(), strategyController.OnCandle, true)

		// preload candles to warmup strategy
		candles, err := n.exchange.CandlesByLimit(ctx, pair, n.strategy.Timeframe(), n.strategy.WarmupPeriod()+1)
		if err != nil {
			return err
		}
		n.dataFeed.Preload(pair, n.strategy.Timeframe(), candles)
		strategyController.Start()
	}

	n.orderFeed.Start()
	n.orderController.Start()
	defer n.orderController.Stop()
	n.dataFeed.Start()
	return nil
}
