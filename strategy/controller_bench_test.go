package strategy

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/rodrigo-brito/ninjabot/indicator"
	"github.com/rodrigo-brito/ninjabot/indicator/stream"
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/service"
)

// benchStrategy computes three common indicators and never trades. The
// slowest period fits the warmup window.
type benchStrategy struct{ warmup int }

func (s benchStrategy) Timeframe() string { return "1h" }
func (s benchStrategy) WarmupPeriod() int { return s.warmup }

func (s benchStrategy) Indicators(df *model.Dataframe) []ChartIndicator {
	df.Metadata["ema"] = indicator.EMA(df.Close, 9)
	df.Metadata["sma"] = indicator.SMA(df.Close, 21)
	df.Metadata["rsi"] = indicator.RSI(df.Close, 14)
	return nil
}

func (s benchStrategy) OnCandle(*model.Dataframe, service.Broker) {}

// syntheticCandles is a deterministic sine-shaped price series.
func syntheticCandles(n int) []model.Candle {
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]model.Candle, n)
	for i := range candles {
		price := 100 + 10*math.Sin(float64(i)/50) + float64(i%7)
		candles[i] = model.Candle{
			Pair: "BTCUSDT", Time: time.Unix(start.Unix()+int64(i)*3600, 0).UTC(),
			Open: price, High: price + 1, Low: price - 1, Close: price, Volume: 1, Complete: true,
		}
	}
	return candles
}

// BenchmarkController_OnCandle measures the per-candle cost of the legacy
// controller: one op is one closed candle, indicators recomputed over the
// warmup window.
func BenchmarkController_OnCandle(b *testing.B) {
	for _, warmup := range []int{22, 200} {
		b.Run(fmt.Sprintf("W=%d", warmup), func(b *testing.B) {
			candles := syntheticCandles(b.N + warmup)
			controller := NewStrategyController("BTCUSDT", benchStrategy{warmup: warmup}, nil, WithCapacity(len(candles)))
			controller.Start()
			for _, candle := range candles[:warmup] {
				controller.OnCandle(candle)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				controller.OnCandle(candles[warmup+i])
			}
		})
	}
}

// streamingBench registers the indicators of benchStrategy incrementally.
type streamingBench struct{ benchStrategy }

func (s streamingBench) Indicators(*model.Dataframe) []ChartIndicator { return nil }
func (s streamingBench) Setup(_ string, set *IndicatorSet) {
	set.Line("ema", stream.NewEMA(9), stream.Close)
	set.Line("sma", stream.NewSMA(21), stream.Close)
	set.Line("rsi", stream.NewRSI(14), stream.Close)
}

// BenchmarkController_Streaming is BenchmarkController_OnCandle for a
// streaming strategy: one op is one closed candle.
func BenchmarkController_Streaming(b *testing.B) {
	for _, warmup := range []int{22, 200} {
		b.Run(fmt.Sprintf("W=%d", warmup), func(b *testing.B) {
			candles := syntheticCandles(b.N + warmup)
			controller := NewStrategyController("BTCUSDT", streamingBench{benchStrategy{warmup: warmup}}, nil,
				WithCapacity(len(candles)))
			controller.Start()
			for _, candle := range candles[:warmup] {
				controller.OnCandle(candle)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				controller.OnCandle(candles[warmup+i])
			}
		})
		b.Run(fmt.Sprintf("W=%d/max-history", warmup), func(b *testing.B) {
			candles := syntheticCandles(b.N + warmup)
			controller := NewStrategyController("BTCUSDT", streamingBench{benchStrategy{warmup: warmup}}, nil,
				WithMaxHistory(1000))
			controller.Start()
			for _, candle := range candles[:warmup] {
				controller.OnCandle(candle)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				controller.OnCandle(candles[warmup+i])
			}
		})
	}
}

type partialBench struct{ streamingBench }

func (partialBench) OnPartialCandle(*model.Dataframe, service.Broker) {}

type legacyPartialBench struct{ benchStrategy }

func (legacyPartialBench) OnPartialCandle(*model.Dataframe, service.Broker) {}

// BenchmarkOnPartialCandle measures a partial candle after a history of the
// given size: constant for streaming strategies, linear for legacy ones.
func BenchmarkOnPartialCandle(b *testing.B) {
	for _, history := range []int{10_000, 100_000} {
		for name, strategy := range map[string]Strategy{
			"streaming": partialBench{streamingBench{benchStrategy{warmup: 22}}},
			"legacy":    legacyPartialBench{benchStrategy{warmup: 22}},
		} {
			b.Run(fmt.Sprintf("%s/history=%d", name, history), func(b *testing.B) {
				candles := syntheticCandles(history + 1)
				controller := NewStrategyController("BTCUSDT", strategy, nil)
				controller.Start()
				for _, candle := range candles[:history] {
					controller.OnCandle(candle)
				}
				partial := candles[history]
				partial.Complete = false

				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					controller.OnPartialCandle(partial)
				}
			})
		}
	}
}
