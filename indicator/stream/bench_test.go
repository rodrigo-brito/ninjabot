package stream

import "testing"

func benchBar(b *testing.B, bar Bar) {
	candles := randomCandles(1024, 1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bar.Update(&candles[i%len(candles)])
	}
}

func benchMulti(b *testing.B, m Multi) {
	candles := randomCandles(1024, 1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Update(&candles[i%len(candles)])
	}
}

func BenchmarkStream_SMA(b *testing.B)        { benchBar(b, FromLine(NewSMA(50), Close)) }
func BenchmarkStream_EMA(b *testing.B)        { benchBar(b, FromLine(NewEMA(50), Close)) }
func BenchmarkStream_RSI(b *testing.B)        { benchBar(b, FromLine(NewRSI(14), Close)) }
func BenchmarkStream_Max(b *testing.B)        { benchBar(b, FromLine(NewMax(50), Close)) }
func BenchmarkStream_StdDev(b *testing.B)     { benchBar(b, FromLine(NewStdDev(20, 2), Close)) }
func BenchmarkStream_ATR(b *testing.B)        { benchBar(b, NewATR(14)) }
func BenchmarkStream_OBV(b *testing.B)        { benchBar(b, NewOBV()) }
func BenchmarkStream_SuperTrend(b *testing.B) { benchBar(b, NewSuperTrend(10, 3)) }
func BenchmarkStream_MACD(b *testing.B)       { benchMulti(b, NewMACD(12, 26, 9)) }
func BenchmarkStream_BB(b *testing.B)         { benchMulti(b, NewBB(20, 2)) }
func BenchmarkStream_Stoch(b *testing.B)      { benchMulti(b, NewStoch(14, 3, 3)) }
