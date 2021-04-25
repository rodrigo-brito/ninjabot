package ninjabot

import (
	"context"
)

type NinjaBot struct {
	settings Settings
	exchange Exchange
	strategy Strategy
}

func NewBot(settings Settings, exchange Exchange, strategy Strategy) *NinjaBot {
	return &NinjaBot{
		settings: settings,
		exchange: exchange,
		strategy: strategy,
	}
}

func (n *NinjaBot) Run(ctx context.Context) error {
	dataFeed := NewDataFeed(n.exchange)
	for _, pair := range n.settings.Pairs {
		strategyController := NewStrategyController(pair, n.settings, n.strategy, n.exchange)
		dataFeed.Register(pair, n.strategy.Timeframe(), strategyController.OnCandle, true)
		candles, err := n.exchange.LoadCandlesByLimit(ctx, pair, n.strategy.Timeframe(), n.strategy.WarmupPeriod()+1)
		if err != nil {
			return err
		}
		dataFeed.Preload(pair, n.strategy.Timeframe(), candles)
		strategyController.Start()
	}
	<-dataFeed.Start(ctx)
	return nil
}
