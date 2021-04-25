package main

import (
	"context"
	"fmt"
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
	strategyController := NewStrategyController(n.settings, n.strategy, n.exchange)
	for _, pair := range n.settings.Pairs {
		dataFeed.Register(pair, n.strategy.Timeframe(), strategyController.OnCandle)
	}
	strategyController.Live()
	fmt.Println("live.")
	<-dataFeed.Start()
	return nil
}
