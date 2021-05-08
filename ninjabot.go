package ninjabot

import (
	"context"

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

	orderController *order.Controller
	orderFeed       *order.FeedSubscription
	dataFeed        *exchange.DataFeedSubscription
}

type Option func(*NinjaBot)

func NewBot(ctx context.Context, settings model.Settings, exc exchange.Exchange, str strategy.Strategy, options ...Option) (*NinjaBot, error) {
	bot := &NinjaBot{
		settings: settings,
		exchange: exc,
		strategy: str,
	}

	for _, option := range options {
		option(bot)
	}

	var err error
	if bot.storage == nil {
		bot.storage, err = storage.New(defaultDatabase)
		if err != nil {
			return nil, err
		}
	}

	if bot.orderFeed == nil {
		bot.orderFeed = order.NewOrderFeed()
	}

	if bot.dataFeed == nil {
		bot.dataFeed = exchange.NewDataFeed(exc)
	}

	if bot.orderController == nil {
		bot.orderController = order.NewController(ctx, exc, bot.storage, bot.orderFeed)
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

func (n *NinjaBot) SubscribeCandle(subscriptions ...CandleSubscriber) {
	for _, symbol := range n.settings.Pairs {
		for _, subscription := range subscriptions {
			n.dataFeed.Subscribe(symbol, n.strategy.Timeframe(), subscription.OnCandle, false)
		}
	}
}

func (n *NinjaBot) SubscribeOrder(subscriptions ...OrderSubscriber) {
	for _, symbol := range n.settings.Pairs {
		for _, subscription := range subscriptions {
			n.orderFeed.Subscribe(symbol, subscription.OnOrder, false)
		}
	}
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
	<-n.dataFeed.Start(ctx)
	return nil
}
