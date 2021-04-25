package main

import "time"

type Strategy interface {
	Init(settings Settings)
	Timeframe() string
	WarmupPeriod() int
	Indicators(dataframe *Dataframe)
	OnCandle(dataframe *Dataframe, broker Broker)
}

type strategyController struct {
	strategy  Strategy
	dataframe *Dataframe
	broker    Broker
	live      bool
}

func NewStrategyController(settings Settings, strategy Strategy, broker Broker) strategyController {
	strategy.Init(settings)
	return strategyController{
		strategy: strategy,
		broker:   broker,
	}
}

func (s *strategyController) Live() {
	s.live = true
}

func (s strategyController) OnCandle(candle Candle) {
	s.dataframe.Time = append([]time.Time{candle.Time}, s.dataframe.Time...)
	s.dataframe.Close = append([]float64{candle.Close}, s.dataframe.Close...)
	s.dataframe.Open = append([]float64{candle.Open}, s.dataframe.Open...)
	s.dataframe.High = append([]float64{candle.High}, s.dataframe.High...)
	s.dataframe.Low = append([]float64{candle.Low}, s.dataframe.Low...)
	s.dataframe.Volume = append([]float64{candle.Volume}, s.dataframe.Volume...)
	s.strategy.Indicators(s.dataframe)
	if s.live {
		s.strategy.OnCandle(s.dataframe, s.broker)
	}
}
