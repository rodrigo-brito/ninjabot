package ui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/strategy"
)

// counterIndicator streams the number of candles seen and the close.
type counterIndicator struct{ fakeIndicator }

func (c *counterIndicator) Name() string { return "COUNTER" }
func (c *counterIndicator) Warmup() int  { return 2 }
func (c *counterIndicator) Metrics() []IndicatorMetric {
	return []IndicatorMetric{{Name: "count", Color: "red", Style: "line"}, {Name: "close", Color: "blue", Style: "line"}}
}
func (c *counterIndicator) NewStream() func(model.Candle) []float64 {
	count := 0.0
	return func(candle model.Candle) []float64 {
		count++
		return []float64{count, candle.Close}
	}
}

func TestChart_StreamIndicators(t *testing.T) {
	counter := &counterIndicator{}
	c := newTestChart(t, WithCustomIndicators(counter))
	base := time.Unix(1_700_000_000, 0).UTC()

	for i := 0; i < 5; i++ {
		c.OnCandle(candleAt("ETHUSDT", base.Add(time.Duration(i)*time.Hour), 10+float64(i)))
		c.OnCandle(candleAt("BTCUSDT", base.Add(time.Duration(i)*time.Hour), 100))
	}
	c.OnCandle(candleAt("ETHUSDT", base.Add(4*time.Hour), 99)) // duplicated, ignored

	indicators := c.indicatorsByPair("ETHUSDT")
	require.Len(t, indicators, 1)
	assert.False(t, counter.loaded, "a stream indicator is never reloaded")
	require.Len(t, indicators[0].Metrics, 2)
	assert.Equal(t, []Point{
		{Time: base.Add(2 * time.Hour).Unix(), Value: 3},
		{Time: base.Add(3 * time.Hour).Unix(), Value: 4},
		{Time: base.Add(4 * time.Hour).Unix(), Value: 5},
	}, indicators[0].Metrics[0].Points, "one stream per pair, warmup skipped")
	assert.Equal(t, 14.0, indicators[0].Metrics[1].Points[2].Value)

	event := c.candleEvent("ETHUSDT", c.candles["ETHUSDT"][4])
	require.Len(t, event.Indicators, 1)
	assert.Equal(t, Point{Time: base.Add(4 * time.Hour).Unix(), Value: 5}, event.Indicators[0].Metrics[0].Point)
}

func TestChart_OnIndicators(t *testing.T) {
	c := newTestChart(t, WithStrategyIndicators(warmupStrategy{}))
	base := time.Unix(1_700_000_000, 0).UTC()

	// the controller reports from the candle that completes the warmup (3)
	for i := 2; i < 6; i++ {
		candle := candleAt("ETHUSDT", base.Add(time.Duration(i)*time.Hour), 10+float64(i))
		values := model.Series[float64]{float64(i - 1), float64(i)}
		c.OnIndicators(candle, []strategy.ChartIndicator{{
			GroupName: "Line", Overlay: true, Warmup: 4,
			Metrics: []strategy.IndicatorMetric{{Name: "value", Color: "red", Values: values}, {Name: "empty"}},
		}})
		c.OnCandle(candle)
	}

	indicators := c.indicatorsByPair("ETHUSDT")
	require.Len(t, indicators, 1)
	assert.Equal(t, "Line", indicators[0].Name)
	require.Len(t, indicators[0].Metrics, 1, "metrics without values are skipped")
	assert.Equal(t, []Point{
		{Time: base.Add(4 * time.Hour).Unix(), Value: 4},
		{Time: base.Add(5 * time.Hour).Unix(), Value: 5},
	}, indicators[0].Metrics[0].Points, "points start at the warmup of the indicator")

	event := c.candleEvent("ETHUSDT", c.candles["ETHUSDT"][len(c.candles["ETHUSDT"])-1])
	require.Len(t, event.Indicators, 1)
	assert.Equal(t, 5.0, event.Indicators[0].Metrics[0].Point.Value)
}

type warmupStrategy struct{ fakeStrategy }

func (warmupStrategy) WarmupPeriod() int { return 3 }
func (warmupStrategy) Indicators(*model.Dataframe) []strategy.ChartIndicator {
	panic("the chart never recomputes the indicators of an observed strategy")
}
