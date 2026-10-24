package strategy

import (
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/service"
)

// ControllerOption customizes a strategy controller.
type ControllerOption func(*Controller)

// WithFullHistory makes the controller hand the whole history to a legacy
// strategy (Indicators and OnCandle) instead of a window of WarmupPeriod
// candles. Indicators then match the values of streaming strategies, at the
// cost of a computation that grows with the history.
func WithFullHistory() ControllerOption {
	return func(c *Controller) { c.fullHistory = true }
}

// WithMaxHistory bounds the memory of the controller: when the history
// reaches twice n candles, only the last n are kept. n must cover the
// warmup period and any lookback the strategy reads.
func WithMaxHistory(n int) ControllerOption {
	return func(c *Controller) { c.maxHistory = n }
}

// WithCapacity pre-allocates the history for n candles. A backtest knows
// how many candles each pair has; reserving them up front avoids growing
// every series as the history fills.
func WithCapacity(n int) ControllerOption {
	return func(c *Controller) { c.capacity = n }
}

type Controller struct {
	strategy     Strategy
	dataframe    *model.Dataframe
	broker       service.Broker
	started      bool
	warmupPeriod int // Cache warmup period

	// streaming strategies
	indicators    *IndicatorSet
	lastCommitted time.Time
	current       model.Candle // reused, so indicators get a pointer without an allocation

	sample      model.Dataframe // reused window handed to the strategy
	fullHistory bool
	maxHistory  int
	capacity    int

	observers []IndicatorObserver
}

func NewStrategyController(pair string, strategy Strategy, broker service.Broker,
	options ...ControllerOption) *Controller {
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

	controller := &Controller{
		dataframe:    dataframe,
		strategy:     strategy,
		broker:       broker,
		warmupPeriod: warmupPeriod,
		sample:       model.Dataframe{Pair: pair, Metadata: make(map[string]model.Series[float64])},
	}
	for _, option := range options {
		option(controller)
	}
	if controller.capacity > cap(dataframe.Close) {
		n := controller.capacity
		if controller.maxHistory > 0 && n > 2*controller.maxHistory {
			n = 2 * controller.maxHistory
		}
		dataframe.Close = make(model.Series[float64], 0, n)
		dataframe.Open = make(model.Series[float64], 0, n)
		dataframe.High = make(model.Series[float64], 0, n)
		dataframe.Low = make(model.Series[float64], 0, n)
		dataframe.Volume = make(model.Series[float64], 0, n)
		dataframe.Time = make([]time.Time, 0, n)
	}

	if streaming, ok := strategy.(StreamingStrategy); ok {
		controller.indicators = newIndicatorSet()
		streaming.Setup(pair, controller.indicators)
		for _, name := range controller.indicators.Names() {
			dataframe.Metadata[name] = make(model.Series[float64], 0, cap(dataframe.Close))
		}
	}

	return controller
}

func (s *Controller) Start() {
	s.started = true
}

// Dataframe returns the history of the controller. It is owned by the
// controller: read it only from the goroutine that feeds candles.
func (s *Controller) Dataframe() *model.Dataframe {
	return s.dataframe
}

func (s *Controller) OnPartialCandle(candle model.Candle) {
	if candle.Complete || len(s.dataframe.Close) < s.strategy.WarmupPeriod() {
		return
	}
	str, ok := s.strategy.(HighFrequencyStrategy)
	if !ok {
		return
	}

	s.updateDataFrame(candle)
	if s.indicators != nil {
		// a partial candle never advances the indicators: it previews them
		s.indicators.update(s.dataframe, s.load(candle), true)
	} else {
		str.Indicators(s.dataframe)
	}
	str.OnPartialCandle(s.dataframe, s.broker)
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

// load copies candle to the reused buffer handed to the indicators.
func (s *Controller) load(candle model.Candle) *model.Candle {
	s.current = candle
	return &s.current
}

func (s *Controller) OnCandle(candle model.Candle) {
	if len(s.dataframe.Time) > 0 && candle.Time.Before(s.dataframe.Time[len(s.dataframe.Time)-1]) {
		log.Errorf("late candle received: %#v", candle)
		return
	}

	s.updateDataFrame(candle)

	var charts []ChartIndicator
	if s.indicators != nil {
		if candle.Time.Equal(s.lastCommitted) && !s.lastCommitted.IsZero() {
			// a repeated candle does not advance the indicators twice
			s.indicators.update(s.dataframe, s.load(candle), true)
		} else {
			s.indicators.update(s.dataframe, s.load(candle), false)
			s.lastCommitted = candle.Time
		}
	}

	s.compact()

	if len(s.dataframe.Close) >= s.warmupPeriod {
		df := s.window()
		if s.indicators != nil {
			if len(s.observers) > 0 {
				charts = s.indicators.chart(s.dataframe)
			}
		} else {
			charts = s.strategy.Indicators(df)
		}
		if s.started {
			s.strategy.OnCandle(df, s.broker)
		}
	}

	s.notify(candle, charts)
}

// window is the view of the history handed to a legacy strategy: the last
// WarmupPeriod candles, or the whole history with WithFullHistory. Streaming
// strategies always get the whole history. The view
// shares the series of the history and reuses one map.
func (s *Controller) window() *model.Dataframe {
	size := len(s.dataframe.Time)
	// streaming indicators already cover the whole history: the strategy
	// reads the history itself, with no window to rebuild
	if s.indicators != nil || s.fullHistory || size <= s.warmupPeriod {
		return s.dataframe
	}

	start := size - s.warmupPeriod
	sample := &s.sample
	sample.Close = s.dataframe.Close[start:]
	sample.Open = s.dataframe.Open[start:]
	sample.High = s.dataframe.High[start:]
	sample.Low = s.dataframe.Low[start:]
	sample.Volume = s.dataframe.Volume[start:]
	sample.Time = s.dataframe.Time[start:]
	sample.LastUpdate = s.dataframe.LastUpdate
	for key, series := range s.dataframe.Metadata {
		sample.Metadata[key] = series.LastValues(s.warmupPeriod)
	}
	if len(sample.Metadata) != len(s.dataframe.Metadata) {
		// the strategy added keys to the window: start over from the history
		clear(sample.Metadata)
		for key, series := range s.dataframe.Metadata {
			sample.Metadata[key] = series.LastValues(s.warmupPeriod)
		}
	}
	return sample
}

// compact drops the oldest candles when the history passes twice the
// configured maximum.
func (s *Controller) compact() {
	if s.maxHistory <= 0 || len(s.dataframe.Time) < 2*s.maxHistory {
		return
	}
	keep := s.maxHistory
	df := s.dataframe
	df.Close = compactSeries(df.Close, keep)
	df.Open = compactSeries(df.Open, keep)
	df.High = compactSeries(df.High, keep)
	df.Low = compactSeries(df.Low, keep)
	df.Volume = compactSeries(df.Volume, keep)
	df.Time = append(df.Time[:0], df.Time[len(df.Time)-keep:]...)
	for key, series := range df.Metadata {
		if len(series) > keep {
			df.Metadata[key] = compactSeries(series, keep)
		}
	}
	if s.indicators != nil {
		s.indicators.resync(df)
	}
}

func compactSeries(series model.Series[float64], keep int) model.Series[float64] {
	return append(series[:0], series[len(series)-keep:]...)
}
