package indicator

import (
	"math"
	"testing"
	"time"

	"github.com/markcheno/go-talib"
	"github.com/stretchr/testify/require"

	ninjaindicator "github.com/rodrigo-brito/ninjabot/indicator"
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/ui"
)

func dataframe(n int) *model.Dataframe {
	df := &model.Dataframe{Pair: "BTCUSDT", Metadata: map[string]model.Series[float64]{}}
	start := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		price := 100 + 10*math.Sin(float64(i)/7) + float64(i%5)
		df.Time = append(df.Time, start.Add(time.Duration(i)*time.Hour))
		df.Open = append(df.Open, price-0.5)
		df.High = append(df.High, price+1+float64(i%3))
		df.Low = append(df.Low, price-1-float64(i%2))
		df.Close = append(df.Close, price)
		df.Volume = append(df.Volume, 100+float64(i%11))
	}
	return df
}

func requireClose(t *testing.T, name string, expected, actual []float64) {
	t.Helper()
	require.Len(t, actual, len(expected), name)
	for i := range expected {
		require.InDelta(t, expected[i], actual[i], 1e-9*math.Max(1, math.Abs(expected[i])), "%s at %d", name, i)
	}
}

// TestIndicators_MatchTalib checks every dashboard indicator against the
// talib computation it replaced, including the leading values it skips.
func TestIndicators_MatchTalib(t *testing.T) {
	df := dataframe(300)
	ppo := func() [][]float64 {
		fast, slow := talib.Ema(df.Close, 12), talib.Ema(df.Close, 26)
		values := make([]float64, len(slow))
		for i := range slow {
			if slow[i] != 0 {
				values[i] = (fast[i] - slow[i]) / slow[i] * 100
			}
		}
		signal := talib.Ema(values, 9)
		hist := make([]float64, len(values))
		for i := range values {
			hist[i] = values[i] - signal[i]
		}
		return [][]float64{values[35:], signal[35:], hist[35:]}
	}
	macd, macdSignal, macdHist := talib.Macd(df.Close, 12, 26, 9)
	upper, middle, lower := talib.BBands(df.Close, 20, 2, 2, talib.EMA)
	k, d := talib.Stoch(df.High, df.Low, df.Close, 14, 3, talib.SMA, 3, talib.SMA)

	for name, tc := range map[string]struct {
		indicator ui.CustomIndicator
		expected  [][]float64
	}{
		"ema":        {EMA(9, "red"), [][]float64{talib.Ema(df.Close, 9)[9:]}},
		"sma":        {SMA(9, "red"), [][]float64{talib.Sma(df.Close, 9)[9:]}},
		"rsi":        {RSI(14, "red"), [][]float64{talib.Rsi(df.Close, 14)[14:]}},
		"cci":        {CCI(14, "red"), [][]float64{talib.Cci(df.High, df.Low, df.Close, 14)[14:]}},
		"willr":      {WillR(14, "red"), [][]float64{talib.WillR(df.High, df.Low, df.Close, 14)[14:]}},
		"obv":        {OBV("red"), [][]float64{talib.Obv(df.Close, df.Volume)}},
		"supertrend": {Spertrend(10, 3, "red"), [][]float64{ninjaindicator.SuperTrend(df.High, df.Low, df.Close, 10, 3)[10:]}},
		"macd":       {MACD(12, 26, 9, "a", "b", "c"), [][]float64{macd[35:], macdSignal[35:], macdHist[35:]}},
		"bb":         {BollingerBands(20, 2, "a", "b"), [][]float64{upper[20:], middle[20:], lower[20:]}},
		"stoch":      {Stoch(14, 3, 3, "a", "b"), [][]float64{k, d}},
		"ppo":        {PPO(12, 26, 9, "a", "b", "c"), ppo()},
	} {
		t.Run(name, func(t *testing.T) {
			tc.indicator.Load(df)
			metrics := tc.indicator.Metrics()
			require.Len(t, metrics, len(tc.expected))
			for i, metric := range metrics {
				requireClose(t, name, tc.expected[i], metric.Values)
				require.Len(t, metric.Time, len(metric.Values))
			}

			// the stream yields the same values, candle by candle
			s := tc.indicator.(ui.StreamIndicator)
			update := s.NewStream()
			for i := range df.Close {
				values := update(model.Candle{Time: df.Time[i], Open: df.Open[i], High: df.High[i],
					Low: df.Low[i], Close: df.Close[i], Volume: df.Volume[i], Complete: true})
				if j := i - tc.indicator.Warmup(); j >= 0 {
					for m := range metrics {
						require.InDelta(t, metrics[m].Values[j], values[m], 1e-12)
					}
				}
			}
			require.NotEmpty(t, tc.indicator.Name())
			_ = tc.indicator.Overlay()
		})
	}
}
