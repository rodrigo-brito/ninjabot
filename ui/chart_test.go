package ui

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/service"
	"github.com/rodrigo-brito/ninjabot/strategy"
)

func newTestChart(t *testing.T, options ...Option) *Chart {
	t.Helper()
	c, err := New(options...)
	require.NoError(t, err)
	return c
}

func candleAt(pair string, ts time.Time, close float64) model.Candle {
	return model.Candle{
		Pair:     pair,
		Time:     ts,
		Open:     close - 1,
		Close:    close,
		High:     close + 2,
		Low:      close - 2,
		Volume:   100,
		Complete: true,
	}
}

func TestChart_OnCandle(t *testing.T) {
	c := newTestChart(t)
	base := time.Date(2021, 9, 26, 20, 0, 0, 0, time.UTC)

	c.OnCandle(candleAt("ETHUSDT", base, 3000))
	c.OnCandle(model.Candle{Pair: "ETHUSDT", Time: base.Add(time.Hour), Close: 1, Complete: false})
	c.OnCandle(candleAt("ETHUSDT", base, 3000)) // duplicated
	c.OnCandle(candleAt("ETHUSDT", base.Add(time.Hour), 3010))
	c.OnCandle(candleAt("BTCUSDT", base, 40000))

	require.Len(t, c.candles["ETHUSDT"], 2)
	require.Len(t, c.candles["BTCUSDT"], 1)
	assert.Equal(t, Candle{Time: base.Unix(), Open: 2999, High: 3002, Low: 2998, Close: 3000, Volume: 100}, c.candles["ETHUSDT"][0])
	assert.Equal(t, []string{"BTCUSDT", "ETHUSDT"}, c.pairs())
	assert.Equal(t, []float64{3000, 3010}, []float64(c.dataframe["ETHUSDT"].Close))
	assert.WithinDuration(t, time.Now(), c.lastUpdate, time.Minute)
}

func TestChart_OnOrderAndSnapshot(t *testing.T) {
	c := newTestChart(t)
	base := time.Date(2021, 9, 26, 20, 0, 0, 0, time.UTC)

	c.OnCandle(candleAt("ETHUSDT", base, 3000))
	c.OnCandle(candleAt("ETHUSDT", base.Add(time.Hour), 3010))
	c.OnCandle(candleAt("ETHUSDT", base.Add(2*time.Hour), 3020))

	stop := 2900.0
	c.OnOrder(model.Order{
		ID: 1, Pair: "ETHUSDT", Side: model.SideTypeBuy, Type: model.OrderTypeMarket,
		Status: model.OrderStatusTypeFilled, Price: 3000, Quantity: 1,
		CreatedAt: base, UpdatedAt: base.Add(30 * time.Minute),
	})
	c.OnOrder(model.Order{
		ID: 2, Pair: "ETHUSDT", Side: model.SideTypeSell, Type: model.OrderTypeStopLoss,
		Status: model.OrderStatusTypeFilled, Price: 3020, Quantity: 1, Stop: &stop,
		CreatedAt: base.Add(time.Hour), UpdatedAt: base.Add(5 * time.Hour), // after last candle
		Profit: 0.1, ProfitValue: 20, RefPrice: 3000,
	})
	// order for a pair without candles must not panic
	c.OnOrder(model.Order{ID: 3, Pair: "SOLUSDT", UpdatedAt: base})

	snap := c.snapshot("ETHUSDT")
	assert.Equal(t, "ETHUSDT", snap.Pair)
	assert.Equal(t, "ETH", snap.Asset)
	assert.Equal(t, "USDT", snap.Quote)
	require.Len(t, snap.Candles, 3)
	require.Len(t, snap.Orders, 2)
	assert.Nil(t, snap.MaxDrawdown)
	assert.Empty(t, snap.EquityValues)
	assert.NotNil(t, snap.Indicators)

	first, second := snap.Orders[0], snap.Orders[1]
	assert.Equal(t, int64(1), first.ID)
	assert.Equal(t, "BUY", first.Side)
	assert.Equal(t, base.Unix(), first.CandleTime, "order inside first candle")
	assert.Equal(t, int64(2), second.ID)
	assert.Equal(t, "STOP_LOSS", second.Type)
	assert.Equal(t, base.Add(2*time.Hour).Unix(), second.CandleTime, "order after last candle snaps to it")
	require.NotNil(t, second.Stop)
	assert.Equal(t, stop, *second.Stop)
	assert.Equal(t, 0.1, second.Profit)

	rows := c.orderRowsByPair("ETHUSDT")
	require.Len(t, rows, 2)
	assert.Equal(t, []string{base.Format(time.RFC3339), "FILLED", "BUY", "1", "MARKET", "1.000000", "3000.000000", "3000.00", ""}, rows[0])
	assert.Equal(t, "0.10", rows[1][8])

	assert.Empty(t, c.orderRowsByPair("UNKNOWN"))
	assert.Empty(t, c.ordersByPair("UNKNOWN"))
}

func TestCandleTimeFor(t *testing.T) {
	candles := []Candle{{Time: 100}, {Time: 200}, {Time: 300}}
	assert.Equal(t, int64(0), candleTimeFor(nil, time.Unix(150, 0)))
	assert.Equal(t, int64(100), candleTimeFor(candles, time.Unix(50, 0)))
	assert.Equal(t, int64(100), candleTimeFor(candles, time.Unix(100, 0)))
	assert.Equal(t, int64(100), candleTimeFor(candles, time.Unix(199, 0)))
	assert.Equal(t, int64(200), candleTimeFor(candles, time.Unix(200, 0)))
	assert.Equal(t, int64(300), candleTimeFor(candles, time.Unix(999, 0)))
}

type fakeIndicator struct {
	loaded bool
}

func (f *fakeIndicator) Name() string  { return "FAKE" }
func (f *fakeIndicator) Overlay() bool { return true }
func (f *fakeIndicator) Warmup() int   { return 0 }
func (f *fakeIndicator) Load(df *model.Dataframe) {
	f.loaded = true
}
func (f *fakeIndicator) Metrics() []IndicatorMetric {
	return []IndicatorMetric{{
		Name:   "fake",
		Color:  "red",
		Style:  "line",
		Values: []float64{1, 2},
		Time:   []time.Time{time.Unix(10, 0), time.Unix(20, 0)},
	}}
}

type fakeStrategy struct{}

func (fakeStrategy) Timeframe() string                         { return "1h" }
func (fakeStrategy) WarmupPeriod() int                         { return 1 }
func (fakeStrategy) OnCandle(*model.Dataframe, service.Broker) {}
func (fakeStrategy) Indicators(df *model.Dataframe) []strategy.ChartIndicator {
	return []strategy.ChartIndicator{
		{
			GroupName: "Close",
			Overlay:   true,
			Warmup:    1,
			Time:      df.Time,
			Metrics: []strategy.IndicatorMetric{
				{Name: "close", Color: "blue", Style: strategy.StyleLine, Values: df.Close},
				{Name: "short", Color: "blue", Style: strategy.StyleLine, Values: nil}, // below warmup, skipped
			},
		},
	}
}

func TestChart_Indicators(t *testing.T) {
	fake := &fakeIndicator{}
	c := newTestChart(t, WithCustomIndicators(fake), WithStrategyIndicators(fakeStrategy{}))
	base := time.Unix(1_700_000_000, 0).UTC()

	assert.Empty(t, c.indicatorsByPair("ETHUSDT"), "no dataframe yet")

	c.OnCandle(candleAt("ETHUSDT", base, 10))
	c.OnCandle(candleAt("ETHUSDT", base.Add(time.Hour), 11))

	indicators := c.indicatorsByPair("ETHUSDT")
	require.Len(t, indicators, 2)
	assert.True(t, fake.loaded)

	assert.Equal(t, "FAKE", indicators[0].Name)
	assert.Equal(t, []Point{{Time: 10, Value: 1}, {Time: 20, Value: 2}}, indicators[0].Metrics[0].Points)

	assert.Equal(t, "Close", indicators[1].Name)
	require.Len(t, indicators[1].Metrics, 1, "metric below warmup is skipped")
	assert.Equal(t, []Point{{Time: base.Add(time.Hour).Unix(), Value: 11}}, indicators[1].Metrics[0].Points)

	event := c.candleEvent("ETHUSDT", c.candles["ETHUSDT"][1])
	require.Len(t, event.Indicators, 2)
	assert.Equal(t, Point{Time: 20, Value: 2}, event.Indicators[0].Metrics[0].Point)
	assert.Nil(t, event.Equity)
}

func TestChart_WithPaperWallet(t *testing.T) {
	wallet := exchange.NewPaperWallet(context.Background(), "USDT", exchange.WithPaperAsset("USDT", 1000))
	c := newTestChart(t, WithPaperWallet(wallet), WithPort(9999))
	assert.Equal(t, 9999, c.port)

	base := time.Unix(1_700_000_000, 0).UTC()
	c.OnCandle(candleAt("ETHUSDT", base, 10))
	wallet.OnCandle(candleAt("ETHUSDT", base, 10))
	wallet.OnCandle(candleAt("ETHUSDT", base.Add(time.Hour), 12))

	snap := c.snapshot("ETHUSDT")
	require.NotEmpty(t, snap.EquityValues)
	assert.Equal(t, 1000.0, snap.EquityValues[0].Value)
	assert.NotNil(t, snap.MaxDrawdown)

	event := c.candleEvent("ETHUSDT", c.candles["ETHUSDT"][0])
	require.NotNil(t, event.Equity)
}

func TestToPoints(t *testing.T) {
	points := toPoints([]time.Time{time.Unix(1, 0), time.Unix(2, 0)}, []float64{1})
	assert.Equal(t, []Point{{Time: 1, Value: 1}}, points)
	assert.Empty(t, toPoints(nil, []float64{1, 2}))
}
