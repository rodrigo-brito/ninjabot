package strategy

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/indicator"
	"github.com/rodrigo-brito/ninjabot/indicator/stream"
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/service"
	"github.com/rodrigo-brito/ninjabot/testdata/mocks"
)

// recordingBroker records the market orders it receives. It always reports
// a position on both sides, so the strategies decide on indicators only.
type recordingBroker struct {
	service.Broker
	calls []string
}

func (b *recordingBroker) Position(string) (float64, float64, error) { return 1, 1000, nil }

func (b *recordingBroker) CreateOrderMarket(side model.SideType, pair string, size float64) (model.Order, error) {
	b.calls = append(b.calls, fmt.Sprintf("%s %s %.8f", side, pair, size))
	return model.Order{}, nil
}

// crossDecision is the trading logic shared by the legacy and the streaming
// strategy: trade the crosses of the close over two moving averages.
func crossDecision(df *model.Dataframe, broker service.Broker) {
	if df.Metadata["fast"].Crossover(df.Metadata["slow"]) {
		_, _ = broker.CreateOrderMarket(model.SideTypeBuy, df.Pair, df.Close.Last(0))
	}
	if df.Metadata["fast"].Crossunder(df.Metadata["slow"]) {
		_, _ = broker.CreateOrderMarket(model.SideTypeSell, df.Pair, df.Close.Last(0))
	}
}

type legacyCross struct{ warmup int }

func (s legacyCross) Timeframe() string { return "1h" }
func (s legacyCross) WarmupPeriod() int { return s.warmup }
func (s legacyCross) Indicators(df *model.Dataframe) []ChartIndicator {
	df.Metadata["fast"] = indicator.EMA(df.Close, 9)
	df.Metadata["slow"] = indicator.SMA(df.Close, 21)
	df.Metadata["rsi"] = indicator.RSI(df.Close, 14)
	return []ChartIndicator{{GroupName: "MA", Time: df.Time, Metrics: []IndicatorMetric{{Name: "fast", Values: df.Metadata["fast"]}}}}
}
func (s legacyCross) OnCandle(df *model.Dataframe, broker service.Broker) { crossDecision(df, broker) }

type streamingCross struct{ legacyCross }

func (s streamingCross) Indicators(*model.Dataframe) []ChartIndicator { return nil }
func (s streamingCross) Setup(_ string, set *IndicatorSet) {
	set.Line("fast", stream.NewEMA(9), stream.Close)
	set.Line("slow", stream.NewSMA(21), stream.Close)
	set.Line("rsi", stream.NewRSI(14), stream.Close)
	set.Plot(ChartSpec{GroupName: "MA", Overlay: true, Metrics: []MetricSpec{{Key: "fast", Name: "EMA 9", Color: "red"}}})
}

func feed(controller *Controller, candles []model.Candle) {
	controller.Start()
	for _, candle := range candles {
		controller.OnPartialCandle(candle)
		if candle.Complete {
			controller.OnCandle(candle)
		}
	}
}

func TestController_StreamingMatchesFullHistory(t *testing.T) {
	candles := syntheticCandles(2000)

	legacyBroker := &recordingBroker{}
	legacy := NewStrategyController("BTCUSDT", legacyCross{warmup: 30}, legacyBroker, WithFullHistory())
	feed(legacy, candles)

	streamingBroker := &recordingBroker{}
	streaming := NewStrategyController("BTCUSDT", streamingCross{legacyCross{warmup: 30}}, streamingBroker)
	feed(streaming, candles)

	require.NotEmpty(t, legacyBroker.calls)
	require.Equal(t, legacyBroker.calls, streamingBroker.calls)

	for _, key := range []string{"fast", "slow", "rsi"} {
		expected, actual := legacy.Dataframe().Metadata[key], streaming.Dataframe().Metadata[key]
		require.Len(t, actual, len(expected), key)
		for i := range expected {
			// bit-exact in a regular build, see indicator/stream parity tests
			require.InDelta(t, expected[i], actual[i], 1e-9, "%s at %d", key, i)
		}
	}
}

func TestController_LegacyWindowDiffersFromFullHistory(t *testing.T) {
	// the legacy default computes recursive indicators over a window of
	// WarmupPeriod candles, so its EMA carries warmup bias
	candles := syntheticCandles(500)
	window := &lastValue{}
	controller := NewStrategyController("BTCUSDT", legacyCross{warmup: 22}, &recordingBroker{})
	controller.Subscribe(window)
	feed(controller, candles)

	full := &lastValue{}
	controller = NewStrategyController("BTCUSDT", legacyCross{warmup: 22}, &recordingBroker{}, WithFullHistory())
	controller.Subscribe(full)
	feed(controller, candles)

	require.NotZero(t, window.value)
	require.Greater(t, math.Abs(window.value-full.value), 1e-6)
}

// lastValue keeps the last plotted value of the first metric.
type lastValue struct{ value float64 }

func (l *lastValue) OnIndicators(_ model.Candle, indicators []ChartIndicator) {
	l.value = indicators[0].Metrics[0].Values.Last(0)
}

// countingLine counts the calls an indicator receives.
type countingLine struct {
	stream.Line
	updates, previews int
}

func (c *countingLine) Update(v float64) float64  { c.updates++; return c.Line.Update(v) }
func (c *countingLine) Preview(v float64) float64 { c.previews++; return c.Line.Preview(v) }

type highFrequency struct {
	counter  *countingLine
	partials []float64
}

func (s *highFrequency) Timeframe() string                            { return "4h" }
func (s *highFrequency) WarmupPeriod() int                            { return 3 }
func (s *highFrequency) Indicators(*model.Dataframe) []ChartIndicator { return nil }
func (s *highFrequency) OnCandle(*model.Dataframe, service.Broker)    {}
func (s *highFrequency) Setup(_ string, set *IndicatorSet) {
	set.Line("sma", s.counter, stream.Close)
}
func (s *highFrequency) OnPartialCandle(df *model.Dataframe, _ service.Broker) {
	s.partials = append(s.partials, df.Metadata["sma"].Last(0))
	if len(df.Metadata["sma"]) != len(df.Close) {
		panic("metadata out of step with the history")
	}
}

func TestController_PartialCandlesPreview(t *testing.T) {
	strategy := &highFrequency{counter: &countingLine{Line: stream.NewSMA(2)}}
	controller := NewStrategyController("BTCUSDT", strategy, nil)
	controller.Start()

	base := syntheticCandles(1)[0]
	complete := 0
	for period := 0; period < 6; period++ {
		for part := 0; part < 4; part++ {
			candle := base
			candle.Time = base.Time.Add(4 * time.Hour * time.Duration(period))
			candle.Close = float64(100 + period*10 + part)
			candle.Complete = part == 3
			controller.OnPartialCandle(candle)
			if candle.Complete {
				controller.OnCandle(candle)
				complete++
			}
		}
	}

	require.Equal(t, complete, strategy.counter.updates, "one update per closed candle")
	require.Len(t, controller.Dataframe().Close, 6)
	require.Len(t, controller.Dataframe().Metadata["sma"], 6)
	// SMA(2) of the closes 100+10p+3: the last value averages periods 4 and 5
	require.Equal(t, (143.0+153.0)/2, controller.Dataframe().Metadata["sma"].Last(0))
	require.NotEmpty(t, strategy.partials)
	// a partial of period 5 previews the SMA with its own close
	require.Contains(t, strategy.partials, (143.0+150.0)/2)

	// a repeated closed candle does not advance the indicators again
	last := base
	last.Time = base.Time.Add(4 * time.Hour * 5)
	last.Close = 153
	last.Complete = true
	controller.OnCandle(last)
	require.Equal(t, complete, strategy.counter.updates)
}

func TestController_MaxHistory(t *testing.T) {
	candles := syntheticCandles(1000)
	broker := &recordingBroker{}
	controller := NewStrategyController("BTCUSDT", streamingCross{legacyCross{warmup: 30}}, broker, WithMaxHistory(100))
	feed(controller, candles)

	df := controller.Dataframe()
	require.Less(t, len(df.Close), 200)
	require.GreaterOrEqual(t, len(df.Close), 100)
	require.Len(t, df.Time, len(df.Close))
	require.Len(t, df.Metadata["fast"], len(df.Close))
	require.Equal(t, candles[len(candles)-1].Time, df.Time[len(df.Time)-1])

	reference := &recordingBroker{}
	feed(NewStrategyController("BTCUSDT", streamingCross{legacyCross{warmup: 30}}, reference), candles)
	require.Equal(t, reference.calls, broker.calls, "compaction does not change decisions")
}

type observer struct{ calls, metrics int }

func (o *observer) OnIndicators(_ model.Candle, indicators []ChartIndicator) {
	o.calls++
	o.metrics = len(indicators[0].Metrics)
}

func TestController_Observers(t *testing.T) {
	candles := syntheticCandles(100)
	for name, strategy := range map[string]Strategy{
		"legacy":    legacyCross{warmup: 30},
		"streaming": streamingCross{legacyCross{warmup: 30}},
	} {
		t.Run(name, func(t *testing.T) {
			o := &observer{}
			controller := NewStrategyController("BTCUSDT", strategy, &recordingBroker{})
			controller.Subscribe(o)
			feed(controller, candles)
			require.Equal(t, 100-30+1, o.calls)
			require.Equal(t, 1, o.metrics)
		})
	}
}

func TestIndicatorSet(t *testing.T) {
	set := newIndicatorSet()
	set.Line("ema", stream.NewEMA(3), stream.Close)
	set.Bar("atr", stream.NewATR(3))
	set.Multi("macd", stream.NewMACD(3, 6, 2))
	require.Equal(t, []string{"ema", "atr", "macd_macd", "macd_signal", "macd_hist"}, set.Names())
	require.Panics(t, func() { set.Line("ema", stream.NewEMA(3), stream.Close) })
}

func TestController_Capacity(t *testing.T) {
	controller := NewStrategyController("BTCUSDT", streamingCross{legacyCross{warmup: 30}}, &recordingBroker{},
		WithCapacity(500))
	require.Equal(t, 500, cap(controller.Dataframe().Close))
	require.Equal(t, 500, cap(controller.Dataframe().Metadata["fast"]))

	bounded := NewStrategyController("BTCUSDT", legacyCross{warmup: 30}, &recordingBroker{},
		WithCapacity(500), WithMaxHistory(100))
	require.Equal(t, 200, cap(bounded.Dataframe().Close), "never more than the bounded history")
}

// fakeStrategy records every call made by the controller so the tests can
// assert on what the strategy actually observed.
type fakeStrategy struct {
	warmupPeriod int

	indicatorCalls []model.Dataframe
	onCandleCalls  []model.Dataframe
}

func (f *fakeStrategy) Timeframe() string { return "1h" }

func (f *fakeStrategy) WarmupPeriod() int { return f.warmupPeriod }

func (f *fakeStrategy) Indicators(df *model.Dataframe) []ChartIndicator {
	f.indicatorCalls = append(f.indicatorCalls, *df)
	return nil
}

func (f *fakeStrategy) OnCandle(df *model.Dataframe, _ service.Broker) {
	f.onCandleCalls = append(f.onCandleCalls, *df)
}

// fakeHighFrequencyStrategy also reacts to partial candles.
type fakeHighFrequencyStrategy struct {
	fakeStrategy

	onPartialCalls []model.Dataframe
}

func (f *fakeHighFrequencyStrategy) OnPartialCandle(df *model.Dataframe, _ service.Broker) {
	f.onPartialCalls = append(f.onPartialCalls, *df)
}

var baseTime = time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

// candle builds a complete candle at baseTime + index hours.
func candle(index int, close float64) model.Candle {
	return model.Candle{
		Pair:     "BTCUSDT",
		Time:     baseTime.Add(time.Duration(index) * time.Hour),
		Open:     close - 1,
		Close:    close,
		Low:      close - 2,
		High:     close + 2,
		Volume:   100 + float64(index),
		Complete: true,
	}
}

func TestNewStrategyController(t *testing.T) {
	strategy := &fakeStrategy{warmupPeriod: 5}

	controller := NewStrategyController("BTCUSDT", strategy, &mocks.Broker{})

	require.Equal(t, "BTCUSDT", controller.dataframe.Pair)
	require.Equal(t, 5, controller.warmupPeriod)
	require.NotNil(t, controller.dataframe.Metadata)
	require.Empty(t, controller.dataframe.Close)
	require.False(t, controller.started, "the controller only trades after Start")
}

func TestControllerOnCandle(t *testing.T) {
	t.Run("fills indicators but does not trade before Start", func(t *testing.T) {
		strategy := &fakeStrategy{warmupPeriod: 2}
		controller := NewStrategyController("BTCUSDT", strategy, &mocks.Broker{})

		controller.OnCandle(candle(0, 100))
		controller.OnCandle(candle(1, 101))

		require.Len(t, strategy.indicatorCalls, 1, "indicators run as soon as the warmup is complete")
		require.Empty(t, strategy.onCandleCalls, "OnCandle is held back until Start")
	})

	t.Run("waits for the warmup period", func(t *testing.T) {
		strategy := &fakeStrategy{warmupPeriod: 3}
		controller := NewStrategyController("BTCUSDT", strategy, &mocks.Broker{})
		controller.Start()

		controller.OnCandle(candle(0, 100))
		controller.OnCandle(candle(1, 101))
		require.Empty(t, strategy.onCandleCalls)

		controller.OnCandle(candle(2, 102))
		require.Len(t, strategy.onCandleCalls, 1)
	})

	t.Run("passes a warmup-sized sample to the strategy", func(t *testing.T) {
		strategy := &fakeStrategy{warmupPeriod: 2}
		controller := NewStrategyController("BTCUSDT", strategy, &mocks.Broker{})
		controller.Start()

		for i := 0; i < 5; i++ {
			controller.OnCandle(candle(i, 100+float64(i)))
		}

		last := strategy.onCandleCalls[len(strategy.onCandleCalls)-1]
		require.Len(t, last.Close, 2)
		require.Equal(t, []float64{103, 104}, []float64(last.Close))
	})

	t.Run("appends each new candle to the dataframe", func(t *testing.T) {
		strategy := &fakeStrategy{warmupPeriod: 1}
		controller := NewStrategyController("BTCUSDT", strategy, &mocks.Broker{})

		controller.OnCandle(candle(0, 100))
		controller.OnCandle(candle(1, 101))

		df := controller.dataframe
		require.Equal(t, []float64{100, 101}, []float64(df.Close))
		require.Equal(t, []float64{99, 100}, []float64(df.Open))
		require.Equal(t, []float64{102, 103}, []float64(df.High))
		require.Equal(t, []float64{98, 99}, []float64(df.Low))
		require.Equal(t, []float64{100, 101}, []float64(df.Volume))
		require.Equal(t, candle(1, 101).Time, df.LastUpdate)
	})

	t.Run("overwrites the last candle when the timestamp repeats", func(t *testing.T) {
		strategy := &fakeStrategy{warmupPeriod: 1}
		controller := NewStrategyController("BTCUSDT", strategy, &mocks.Broker{})

		controller.OnCandle(candle(0, 100))
		updated := candle(0, 105)
		controller.OnCandle(updated)

		df := controller.dataframe
		require.Len(t, df.Close, 1, "an update must not append a new row")
		require.Equal(t, 105.0, df.Close[0])
		require.Equal(t, 107.0, df.High[0])
	})

	t.Run("keeps candle metadata in sync", func(t *testing.T) {
		strategy := &fakeStrategy{warmupPeriod: 1}
		controller := NewStrategyController("BTCUSDT", strategy, &mocks.Broker{})

		first := candle(0, 100)
		first.Metadata = map[string]float64{"funding": 0.01}
		controller.OnCandle(first)

		update := candle(0, 101)
		update.Metadata = map[string]float64{"funding": 0.02}
		controller.OnCandle(update)

		require.Equal(t, []float64{0.02}, []float64(controller.dataframe.Metadata["funding"]))
	})

	t.Run("drops late candles", func(t *testing.T) {
		strategy := &fakeStrategy{warmupPeriod: 1}
		controller := NewStrategyController("BTCUSDT", strategy, &mocks.Broker{})
		controller.Start()

		controller.OnCandle(candle(2, 102))
		controller.OnCandle(candle(1, 101)) // older than the last one

		require.Len(t, controller.dataframe.Close, 1)
		require.Equal(t, 102.0, controller.dataframe.Close[0])
	})
}

func TestControllerOnPartialCandle(t *testing.T) {
	partial := func(index int, close float64) model.Candle {
		c := candle(index, close)
		c.Complete = false
		return c
	}

	t.Run("notifies high frequency strategies after the warmup", func(t *testing.T) {
		strategy := &fakeHighFrequencyStrategy{fakeStrategy: fakeStrategy{warmupPeriod: 2}}
		controller := NewStrategyController("BTCUSDT", strategy, &mocks.Broker{})

		controller.OnCandle(candle(0, 100))
		controller.OnCandle(candle(1, 101))
		controller.OnPartialCandle(partial(2, 102))

		require.Len(t, strategy.onPartialCalls, 1)
		require.Equal(t, 102.0, strategy.onPartialCalls[0].Close.Last(0))
	})

	t.Run("ignores partial candles before the warmup", func(t *testing.T) {
		strategy := &fakeHighFrequencyStrategy{fakeStrategy: fakeStrategy{warmupPeriod: 5}}
		controller := NewStrategyController("BTCUSDT", strategy, &mocks.Broker{})

		controller.OnPartialCandle(partial(0, 100))

		require.Empty(t, strategy.onPartialCalls)
	})

	t.Run("ignores complete candles", func(t *testing.T) {
		strategy := &fakeHighFrequencyStrategy{fakeStrategy: fakeStrategy{warmupPeriod: 1}}
		controller := NewStrategyController("BTCUSDT", strategy, &mocks.Broker{})

		controller.OnCandle(candle(0, 100))
		controller.OnPartialCandle(candle(1, 101))

		require.Empty(t, strategy.onPartialCalls)
	})

	t.Run("is a no-op for regular strategies", func(t *testing.T) {
		strategy := &fakeStrategy{warmupPeriod: 1}
		controller := NewStrategyController("BTCUSDT", strategy, &mocks.Broker{})

		controller.OnCandle(candle(0, 100))
		controller.OnPartialCandle(partial(1, 101))

		require.Len(t, controller.dataframe.Close, 1, "a partial candle must not reach the dataframe")
	})
}
