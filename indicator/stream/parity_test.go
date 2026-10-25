package stream

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/markcheno/go-talib"
	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/indicator"
	"github.com/rodrigo-brito/ninjabot/model"
)

var (
	sizes   = []int{30, 100, 1000, 5000}
	periods = []int{2, 3, 5, 9, 14, 21, 50}
)

// randomCandles is a seeded random walk with consistent OHLC bars. Some bars
// repeat the previous close, to exercise the equality branches.
func randomCandles(n int, seed int64) []model.Candle {
	random := rand.New(rand.NewSource(seed)) //nolint:gosec // reproducible test data
	candles := make([]model.Candle, n)
	price := 100.0
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range candles {
		open := price
		if random.Intn(10) > 0 {
			price *= 1 + random.NormFloat64()*0.02
		}
		high := math.Max(open, price) * (1 + random.Float64()*0.01)
		low := math.Min(open, price) * (1 - random.Float64()*0.01)
		candles[i] = model.Candle{
			Time: start.Add(time.Duration(i) * time.Hour), Open: open, High: high, Low: low, Close: price,
			Volume: 100 + random.Float64()*1000, Complete: true,
		}
	}
	return candles
}

type series struct{ open, high, low, close, volume []float64 }

func split(candles []model.Candle) series {
	var s series
	for _, c := range candles {
		s.open = append(s.open, c.Open)
		s.high = append(s.high, c.High)
		s.low = append(s.low, c.Low)
		s.close = append(s.close, c.Close)
		s.volume = append(s.volume, c.Volume)
	}
	return s
}

// requireSeries compares an incremental series with talib's, value by value.
// The indicators replicate talib's arithmetic and are bit-exact in a regular
// build; Go may fuse multiply-adds differently in other build modes (-race
// on arm64), hence the relative tolerance of 1e-12.
func requireSeries(t *testing.T, name string, expected, actual []float64) {
	t.Helper()
	require.Len(t, actual, len(expected), name)
	for i := range expected {
		if math.Abs(expected[i]-actual[i]) > 1e-12*math.Max(1, math.Abs(expected[i])) {
			t.Fatalf("%s: index %d: talib %v, stream %v", name, i, expected[i], actual[i])
		}
	}
}

func runLine(l Line, values []float64) []float64 {
	out := make([]float64, len(values))
	for i, v := range values {
		preview := l.Preview(v)
		out[i] = l.Update(v)
		if preview != out[i] {
			panic(fmt.Sprintf("preview %v differs from update %v at %d", preview, out[i], i))
		}
	}
	return out
}

func runBar(b Bar, candles []model.Candle) []float64 {
	out := make([]float64, len(candles))
	for i, c := range candles {
		preview := b.Preview(&c)
		out[i] = b.Update(&c)
		if preview != out[i] {
			panic(fmt.Sprintf("preview %v differs from update %v at %d", preview, out[i], i))
		}
	}
	return out
}

func runMulti(m Multi, candles []model.Candle) [][]float64 {
	out := make([][]float64, len(m.Outputs()))
	for _, c := range candles {
		preview := m.Preview(&c)
		values := m.Update(&c)
		for j, v := range values {
			if preview[j] != v {
				panic(fmt.Sprintf("%s preview %v differs from update %v", m.Outputs()[j], preview[j], v))
			}
			out[j] = append(out[j], v)
		}
	}
	return out
}

func eachCase(t *testing.T, minSize func(period int) int, test func(t *testing.T, candles []model.Candle, s series, period int)) {
	for _, size := range sizes {
		for _, period := range periods {
			if size < minSize(period) {
				continue
			}
			t.Run(fmt.Sprintf("n=%d/p=%d", size, period), func(t *testing.T) {
				candles := randomCandles(size, int64(size*100+period))
				test(t, candles, split(candles), period)
			})
		}
	}
}

func same(p int) int { return p + 1 }

func TestParity_SMA(t *testing.T) {
	eachCase(t, same, func(t *testing.T, _ []model.Candle, s series, p int) {
		requireSeries(t, "SMA", talib.Sma(s.close, p), runLine(NewSMA(p), s.close))
	})
}

func TestParity_EMA(t *testing.T) {
	eachCase(t, same, func(t *testing.T, _ []model.Candle, s series, p int) {
		requireSeries(t, "EMA", talib.Ema(s.close, p), runLine(NewEMA(p), s.close))
	})
}

func TestParity_RSI(t *testing.T) {
	eachCase(t, func(p int) int { return p + 2 }, func(t *testing.T, _ []model.Candle, s series, p int) {
		requireSeries(t, "RSI", talib.Rsi(s.close, p), runLine(NewRSI(p), s.close))
	})
}

func TestParity_ATR(t *testing.T) {
	eachCase(t, func(p int) int { return p + 2 }, func(t *testing.T, candles []model.Candle, s series, p int) {
		requireSeries(t, "ATR", talib.Atr(s.high, s.low, s.close, p), runBar(NewATR(p), candles))
	})
	candles := randomCandles(100, 1)
	s := split(candles)
	requireSeries(t, "ATR(1)", talib.Atr(s.high, s.low, s.close, 1), runBar(NewATR(1), candles))
}

func TestParity_MaxMin(t *testing.T) {
	eachCase(t, same, func(t *testing.T, _ []model.Candle, s series, p int) {
		requireSeries(t, "Max", talib.Max(s.close, p), runLine(NewMax(p), s.close))
		requireSeries(t, "Min", talib.Min(s.close, p), runLine(NewMin(p), s.close))
	})
}

func TestParity_StdDev(t *testing.T) {
	eachCase(t, same, func(t *testing.T, _ []model.Candle, s series, p int) {
		requireSeries(t, "Var", talib.Var(s.close, p), runLine(NewVariance(p), s.close))
		requireSeries(t, "StdDev", talib.StdDev(s.close, p, 1), runLine(NewStdDev(p, 1), s.close))
		requireSeries(t, "StdDev2", talib.StdDev(s.close, p, 2), runLine(NewStdDev(p, 2), s.close))
	})
}

func TestParity_OBV(t *testing.T) {
	for _, size := range sizes {
		candles := randomCandles(size, int64(size))
		s := split(candles)
		requireSeries(t, "OBV", talib.Obv(s.close, s.volume), runBar(NewOBV(), candles))
	}
}

func TestParity_MACD(t *testing.T) {
	for _, size := range sizes {
		for _, params := range [][3]int{{12, 26, 9}, {5, 13, 4}, {26, 12, 9}, {3, 10, 2}} {
			if size < params[0]+params[1]+params[2] {
				continue
			}
			candles := randomCandles(size, int64(size+params[1]))
			s := split(candles)
			macd, signal, hist := talib.Macd(s.close, params[0], params[1], params[2])
			out := runMulti(NewMACD(params[0], params[1], params[2]), candles)
			name := fmt.Sprintf("MACD%v/n=%d", params, size)
			requireSeries(t, name+"/macd", macd, out[0])
			requireSeries(t, name+"/signal", signal, out[1])
			requireSeries(t, name+"/hist", hist, out[2])
		}
	}
}

func TestParity_BB(t *testing.T) {
	eachCase(t, same, func(t *testing.T, candles []model.Candle, s series, p int) {
		for _, deviation := range []float64{1, 2} {
			upper, middle, lower := talib.BBands(s.close, p, deviation, deviation, talib.SMA)
			out := runMulti(NewBB(p, deviation), candles)
			requireSeries(t, "BB/upper", upper, out[0])
			requireSeries(t, "BB/middle", middle, out[1])
			requireSeries(t, "BB/lower", lower, out[2])
		}
	})
}

func TestParity_BBEMA(t *testing.T) {
	eachCase(t, same, func(t *testing.T, candles []model.Candle, s series, p int) {
		upper, middle, lower := talib.BBands(s.close, p, 2, 2, talib.EMA)
		out := runMulti(NewBBEMA(p, 2), candles)
		requireSeries(t, "BBEMA/upper", upper, out[0])
		requireSeries(t, "BBEMA/middle", middle, out[1])
		requireSeries(t, "BBEMA/lower", lower, out[2])
	})
}

func TestParity_CCI(t *testing.T) {
	eachCase(t, same, func(t *testing.T, candles []model.Candle, s series, p int) {
		requireSeries(t, "CCI", talib.Cci(s.high, s.low, s.close, p), runBar(NewCCI(p), candles))
	})
}

func TestParity_WillR(t *testing.T) {
	eachCase(t, same, func(t *testing.T, candles []model.Candle, s series, p int) {
		requireSeries(t, "WillR", talib.WillR(s.high, s.low, s.close, p), runBar(NewWillR(p), candles))
	})
}

func TestParity_Stoch(t *testing.T) {
	for _, size := range sizes {
		for _, params := range [][3]int{{14, 3, 3}, {8, 3, 3}, {5, 1, 1}, {9, 5, 2}} {
			if size < params[0]+params[1]+params[2] {
				continue
			}
			candles := randomCandles(size, int64(size+params[0]))
			s := split(candles)
			k, d := talib.Stoch(s.high, s.low, s.close, params[0], params[1], talib.SMA, params[2], talib.SMA)
			out := runMulti(NewStoch(params[0], params[1], params[2]), candles)
			name := fmt.Sprintf("Stoch%v/n=%d", params, size)
			requireSeries(t, name+"/k", k, out[0])
			requireSeries(t, name+"/d", d, out[1])
		}
	}
}

func TestParity_SuperTrend(t *testing.T) {
	eachCase(t, func(p int) int { return p + 2 }, func(t *testing.T, candles []model.Candle, s series, p int) {
		expected := indicator.SuperTrend(s.high, s.low, s.close, p, 3)
		requireSeries(t, "SuperTrend", expected, runBar(NewSuperTrend(p, 3), candles))
	})
}

func TestFromLineAndSources(t *testing.T) {
	candles := randomCandles(50, 7)
	s := split(candles)
	bar := FromLine(NewSMA(5), Close)
	requireSeries(t, "SMA bar", talib.Sma(s.close, 5), runBar(bar, candles))
	require.True(t, bar.Ready())
	require.Equal(t, 4, bar.Warmup())
	bar.Reset()
	require.False(t, bar.Ready())

	c := model.Candle{Open: 1, High: 4, Low: 2, Close: 3, Volume: 9}
	require.Equal(t, 1.0, Open(&c))
	require.Equal(t, 4.0, High(&c))
	require.Equal(t, 2.0, Low(&c))
	require.Equal(t, 9.0, Volume(&c))
	require.Equal(t, 3.0, HL2(&c))
	require.Equal(t, 3.0, HLC3(&c))
	require.Equal(t, 2.5, OHLC4(&c))
}

func TestReadyAndReset(t *testing.T) {
	candles := randomCandles(60, 3)
	indicators := []struct {
		name   string
		update func(model.Candle)
		ready  func() bool
		warmup int
		reset  func()
	}{}
	add := func(name string, b Bar) {
		indicators = append(indicators, struct {
			name   string
			update func(model.Candle)
			ready  func() bool
			warmup int
			reset  func()
		}{name, func(c model.Candle) { b.Update(&c) }, b.Ready, b.Warmup(), b.Reset})
	}
	addMulti := func(name string, m Multi) {
		indicators = append(indicators, struct {
			name   string
			update func(model.Candle)
			ready  func() bool
			warmup int
			reset  func()
		}{name, func(c model.Candle) { m.Update(&c) }, m.Ready, m.Warmup(), m.Reset})
	}
	add("sma", FromLine(NewSMA(5), Close))
	add("ema", FromLine(NewEMA(5), Close))
	add("rsi", FromLine(NewRSI(5), Close))
	add("max", FromLine(NewMax(5), Close))
	add("min", FromLine(NewMin(5), Close))
	add("std", FromLine(NewStdDev(5, 2), Close))
	add("atr", NewATR(5))
	add("obv", NewOBV())
	add("supertrend", NewSuperTrend(5, 3))
	add("cci", NewCCI(5))
	add("willr", NewWillR(5))
	addMulti("macd", NewMACD(3, 6, 2))
	addMulti("bb", NewBB(5, 2))
	addMulti("bbema", NewBBEMA(5, 2))
	addMulti("stoch", NewStoch(5, 3, 3))

	for _, ind := range indicators {
		t.Run(ind.name, func(t *testing.T) {
			for i := 0; i < 2; i++ { // the second pass checks Reset
				for j, c := range candles {
					ind.update(c)
					require.Equal(t, j >= ind.warmup, ind.ready(), "bar %d", j)
				}
				ind.reset()
				require.False(t, ind.ready())
			}
		})
	}
}

func TestDegeneratePeriods(t *testing.T) {
	require.Panics(t, func() { NewSMA(0) })
	require.Panics(t, func() { NewEMA(0) })
	require.Panics(t, func() { NewVariance(0) })
	require.Panics(t, func() { NewCCI(0) })
	require.Panics(t, func() { NewWillR(0) })

	values := split(randomCandles(20, 1)).close
	requireSeries(t, "RSI(1)", talib.Rsi(values, 1), runLine(NewRSI(1), values))
	requireSeries(t, "Max(1)", talib.Max(values, 1), runLine(NewMax(1), values))
	require.False(t, NewRSI(1).Ready())
}
