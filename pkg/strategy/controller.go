package strategy

import (
	"github.com/rodrigo-brito/ninjabot/pkg/exchange"
	"github.com/rodrigo-brito/ninjabot/pkg/model"
	"github.com/rodrigo-brito/ninjabot/pkg/series"
)

type Controller struct {
	strategy  Strategy
	dataframe *model.Dataframe
	broker    exchange.Broker
	started   bool
}

func NewStrategyController(pair string, settings model.Settings, strategy Strategy,
	broker exchange.Broker) *Controller {

	strategy.Init(settings)
	dataframe := &model.Dataframe{
		Pair:     pair,
		Metadata: make(map[string]series.Series),
	}

	return &Controller{
		dataframe: dataframe,
		strategy:  strategy,
		broker:    broker,
	}
}

func (s *Controller) Start() {
	s.started = true
}

func (s *Controller) OnCandle(candle model.Candle) {
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
