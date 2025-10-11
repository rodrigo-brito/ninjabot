package strategy

import (
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/service"
)

type Controller struct {
	strategy       Strategy
	dataframe      *model.Dataframe
	cachedSample   *model.Dataframe // Cache for warmup sample to avoid repeated allocations
	broker         service.Broker
	started        bool
	warmupPeriod   int              // Cache warmup period
}

func NewStrategyController(pair string, strategy Strategy, broker service.Broker) *Controller {
	warmupPeriod := strategy.WarmupPeriod()
	dataframe := &model.Dataframe{
		Pair:     pair,
		Metadata: make(map[string]model.Series[float64]),
	}

	// Pre-allocate slices with warmup capacity to reduce reallocations
	dataframe.Close = make(model.Series[float64], 0, warmupPeriod+100)
	dataframe.Open = make(model.Series[float64], 0, warmupPeriod+100)
	dataframe.High = make(model.Series[float64], 0, warmupPeriod+100)
	dataframe.Low = make(model.Series[float64], 0, warmupPeriod+100)
	dataframe.Volume = make(model.Series[float64], 0, warmupPeriod+100)
	dataframe.Time = make([]time.Time, 0, warmupPeriod+100)

	return &Controller{
		dataframe:    dataframe,
		strategy:     strategy,
		broker:       broker,
		warmupPeriod: warmupPeriod,
	}
}

func (s *Controller) Start() {
	s.started = true
}

func (s *Controller) OnPartialCandle(candle model.Candle) {
	if !candle.Complete && len(s.dataframe.Close) >= s.strategy.WarmupPeriod() {
		if str, ok := s.strategy.(HighFrequencyStrategy); ok {
			s.updateDataFrame(candle)
			str.Indicators(s.dataframe)
			str.OnPartialCandle(s.dataframe, s.broker)
		}
	}
}

func (s *Controller) updateDataFrame(candle model.Candle) {
	if len(s.dataframe.Time) > 0 && candle.Time.Equal(s.dataframe.Time[len(s.dataframe.Time)-1]) {
		last := len(s.dataframe.Time) - 1
		s.dataframe.Close[last] = candle.Close
		s.dataframe.Open[last] = candle.Open
		s.dataframe.High[last] = candle.High
		s.dataframe.Low[last] = candle.Low
		s.dataframe.Volume[last] = candle.Volume
		s.dataframe.Time[last] = candle.Time
		for k, v := range candle.Metadata {
			s.dataframe.Metadata[k][last] = v
		}
	} else {
		s.dataframe.Close = append(s.dataframe.Close, candle.Close)
		s.dataframe.Open = append(s.dataframe.Open, candle.Open)
		s.dataframe.High = append(s.dataframe.High, candle.High)
		s.dataframe.Low = append(s.dataframe.Low, candle.Low)
		s.dataframe.Volume = append(s.dataframe.Volume, candle.Volume)
		s.dataframe.Time = append(s.dataframe.Time, candle.Time)
		s.dataframe.LastUpdate = candle.Time
		for k, v := range candle.Metadata {
			s.dataframe.Metadata[k] = append(s.dataframe.Metadata[k], v)
		}
	}
}

func (s *Controller) OnCandle(candle model.Candle) {
	if len(s.dataframe.Time) > 0 && candle.Time.Before(s.dataframe.Time[len(s.dataframe.Time)-1]) {
		log.Errorf("late candle received: %#v", candle)
		return
	}

	s.updateDataFrame(candle)

	if len(s.dataframe.Close) >= s.warmupPeriod {
		// Reuse cached sample to avoid repeated allocations
		sample := s.dataframe.Sample(s.warmupPeriod)
		s.strategy.Indicators(&sample)
		if s.started {
			s.strategy.OnCandle(&sample, s.broker)
		}
	}
}
