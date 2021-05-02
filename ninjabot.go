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

type NinjaBot struct {
	settings model.Settings
	exchange exchange.Exchange
	strategy strategy.Strategy
	storage  *ent.Client
}

type Option func(*NinjaBot)

func NewBot(settings model.Settings, exchange exchange.Exchange, strategy strategy.Strategy, options ...Option) (*NinjaBot, error) {
	bot := &NinjaBot{
		settings: settings,
		exchange: exchange,
		strategy: strategy,
	}

	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04",
	})

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

func (n *NinjaBot) Run(ctx context.Context) error {
	oderFeed := order.NewOrderFeed()
	dataFeed := exchange.NewDataFeed(n.exchange)
	orderController := order.NewController(ctx, n.exchange, n.storage, oderFeed)

	for _, pair := range n.settings.Pairs {
		// setup and subscribe strategy to data feed (candles)
		strategyController := strategy.NewStrategyController(pair, n.settings, n.strategy, orderController)
		dataFeed.Subscribe(pair, n.strategy.Timeframe(), strategyController.OnCandle, true)

		// preload candles to warmup strategy
		candles, err := n.exchange.LoadCandlesByLimit(ctx, pair, n.strategy.Timeframe(), n.strategy.WarmupPeriod()+1)
		if err != nil {
			return err
		}
		dataFeed.Preload(pair, n.strategy.Timeframe(), candles)
		strategyController.Start()
	}

	oderFeed.Start()
	orderController.Start()
	<-dataFeed.Start(ctx)
	return nil
}
