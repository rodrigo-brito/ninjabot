package strategy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/service"
	"github.com/rodrigo-brito/ninjabot/testdata/mocks"
)

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
