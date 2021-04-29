package strategy

import (
	"github.com/rodrigo-brito/ninjabot/pkg/exchange"
	"github.com/rodrigo-brito/ninjabot/pkg/model"
	"github.com/rodrigo-brito/ninjabot/pkg/order"
)

type Strategy interface {
	Init(settings model.Settings)
	Timeframe() string
	WarmupPeriod() int
	Indicators(dataframe *model.Dataframe)
	OnCandle(dataframe *model.Dataframe, broker exchange.Broker)
}

type strategyController struct {
	strategy        Strategy
	dataframe       *model.Dataframe
	broker          exchange.Broker
	orderController order.Controller
	started         bool
}

func NewStrategyController(pair string, settings model.Settings, strategy Strategy, orderController order.Controller) *strategyController {
	strategy.Init(settings)
	dataframe := &model.Dataframe{
		Pair:     pair,
		Metadata: make(map[string][]float64),
	}

	return &strategyController{
		dataframe: dataframe,
		strategy:  strategy,
		broker:    orderController,
	}
}

func (s *strategyController) Start() {
	s.started = true
}

func (s *strategyController) OnCandle(candle model.Candle) {
	s.dataframe.Close = append(s.dataframe.Close, candle.Close)
	s.dataframe.Open = append(s.dataframe.Open, candle.Open)
	s.dataframe.High = append(s.dataframe.High, candle.High)
	s.dataframe.Low = append(s.dataframe.Low, candle.Low)
	s.dataframe.Volume = append(s.dataframe.Volume, candle.Volume)
	s.dataframe.Time = append(s.dataframe.Time, candle.Time)
	s.dataframe.LastUpdate = candle.Time

	if len(s.dataframe.Close) > s.strategy.WarmupPeriod() {
		s.strategy.Indicators(s.dataframe)
		if s.started {
			s.strategy.OnCandle(s.dataframe, s.broker)
		}
	}
}
