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
