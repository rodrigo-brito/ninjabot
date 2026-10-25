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
		"stoch":      {Stoch(14, 3, 3, "a", "b"), [][]float64{k[6:], d[6:]}},
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

// newDataframe builds a deterministic OHLCV dataframe with the requested
// number of candles.
func newDataframe(size int) *model.Dataframe {
	df := &model.Dataframe{
		Pair:   "BTCUSDT",
		Close:  make(model.Series[float64], size),
		Open:   make(model.Series[float64], size),
		High:   make(model.Series[float64], size),
		Low:    make(model.Series[float64], size),
		Volume: make(model.Series[float64], size),
		Time:   make([]time.Time, size),
	}

	start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < size; i++ {
		close := 100 + 10*math.Sin(float64(i)/5) + float64(i)/4
		df.Close[i] = close
		df.Open[i] = close - 0.5
		df.High[i] = close + 1
		df.Low[i] = close - 1
		df.Volume[i] = 1000 + 300*math.Cos(float64(i)/3)
		df.Time[i] = start.Add(time.Duration(i) * time.Minute)
	}

	return df
}

// indicatorCase describes the contract every dashboard indicator must honour.
type indicatorCase struct {
	name        string
	build       func() ui.CustomIndicator
	wantName    string
	wantWarmup  int
	wantOverlay bool
	wantMetrics []string // style of each metric, in order
}

func indicatorCases() []indicatorCase {
	return []indicatorCase{
		{
			name:        "BollingerBands",
			build:       func() ui.CustomIndicator { return BollingerBands(20, 2, "#f00", "#0f0") },
			wantName:    "BB(20, 2.00)",
			wantWarmup:  20,
			wantOverlay: true,
			wantMetrics: []string{"line", "line", "line"},
		},
		{
			name:        "CCI",
			build:       func() ui.CustomIndicator { return CCI(14, "#f00") },
			wantName:    "CCI(14)",
			wantWarmup:  14,
			wantOverlay: false,
			wantMetrics: []string{"line"},
		},
		{
			name:        "EMA",
			build:       func() ui.CustomIndicator { return EMA(9, "#f00") },
			wantName:    "EMA(9)",
			wantWarmup:  9,
			wantOverlay: true,
			wantMetrics: []string{"line"},
		},
		{
			name:        "MACD",
			build:       func() ui.CustomIndicator { return MACD(12, 26, 9, "#f00", "#0f0", "#00f") },
			wantName:    "MACD(12, 26, 9)",
			wantWarmup:  35,
			wantOverlay: false,
			wantMetrics: []string{"line", "line", "bar"},
		},
		{
			name:        "OBV",
			build:       func() ui.CustomIndicator { return OBV("#f00") },
			wantName:    "OBV",
			wantWarmup:  0,
			wantOverlay: false,
			wantMetrics: []string{"line"},
		},
		{
			name:        "PPO",
			build:       func() ui.CustomIndicator { return PPO(12, 26, 9, "#f00", "#0f0", "#00f") },
			wantName:    "PPO(12, 26, 9)",
			wantWarmup:  35,
			wantOverlay: false,
			wantMetrics: []string{"line", "line", "bar"},
		},
		{
			name:        "RSI",
			build:       func() ui.CustomIndicator { return RSI(14, "#f00") },
			wantName:    "RSI(14)",
			wantWarmup:  14,
			wantOverlay: false,
			wantMetrics: []string{"line"},
		},
		{
			name:        "SMA",
			build:       func() ui.CustomIndicator { return SMA(20, "#f00") },
			wantName:    "SMA(20)",
			wantWarmup:  20,
			wantOverlay: true,
			wantMetrics: []string{"line"},
		},
		{
			name:        "Stoch",
			build:       func() ui.CustomIndicator { return Stoch(14, 3, 3, "#f00", "#0f0") },
			wantName:    "STOCH(14, 3, 3)",
			wantWarmup:  6,
			wantOverlay: false,
			wantMetrics: []string{"line", "line"},
		},
		{
			name:        "SuperTrend",
			build:       func() ui.CustomIndicator { return Spertrend(10, 3, "#f00") },
			wantName:    "SuperTrend(10,3.0)",
			wantWarmup:  10,
			wantOverlay: true,
			wantMetrics: []string{"scatter"},
		},
		{
			name:        "WillR",
			build:       func() ui.CustomIndicator { return WillR(14, "#f00") },
			wantName:    "%R(14)",
			wantWarmup:  14,
			wantOverlay: false,
			wantMetrics: []string{"line"},
		},
	}
}

func TestIndicatorMetadata(t *testing.T) {
	for _, tt := range indicatorCases() {
		t.Run(tt.name, func(t *testing.T) {
			indicator := tt.build()

			require.Equal(t, tt.wantName, indicator.Name())
			require.Equal(t, tt.wantWarmup, indicator.Warmup())
			require.Equal(t, tt.wantOverlay, indicator.Overlay())
		})
	}
}

func TestIndicatorLoad(t *testing.T) {
	df := newDataframe(120)

	for _, tt := range indicatorCases() {
		t.Run(tt.name, func(t *testing.T) {
			indicator := tt.build()
			indicator.Load(df)

			metrics := indicator.Metrics()
			require.Len(t, metrics, len(tt.wantMetrics))

			for i, metric := range metrics {
				require.Equal(t, tt.wantMetrics[i], metric.Style, "metric %d style", i)
				require.NotEmpty(t, metric.Color, "metric %d color", i)
				require.NotEmpty(t, metric.Values, "metric %d values", i)
				require.Len(t, metric.Time, len(metric.Values), "metric %d time/values mismatch", i)
				require.Subset(t, df.Time, metric.Time, "metric %d must reuse the dataframe timestamps", i)
			}
		})
	}
}

// Indicators that guard against short dataframes must leave their series empty
// instead of panicking on the warmup slice.
func TestIndicatorLoadSkipsShortDataframe(t *testing.T) {
	guarded := []string{"BollingerBands", "EMA", "PPO", "RSI", "SMA", "SuperTrend", "WillR"}
	byName := map[string]indicatorCase{}
	for _, tt := range indicatorCases() {
		byName[tt.name] = tt
	}

	df := newDataframe(3)

	for _, name := range guarded {
		t.Run(name, func(t *testing.T) {
			indicator := byName[name].build()
			indicator.Load(df)

			for _, metric := range indicator.Metrics() {
				require.Empty(t, metric.Values)
				require.Empty(t, metric.Time)
			}
		})
	}
}

func TestPPOValues(t *testing.T) {
	df := newDataframe(120)
	indicator := PPO(12, 26, 9, "#f00", "#0f0", "#00f")
	indicator.Load(df)

	metrics := indicator.Metrics()
	ppo, signal, hist := metrics[0], metrics[1], metrics[2]

	require.Equal(t, "PPO", ppo.Name)
	require.Equal(t, "PPOSignal", signal.Name)
	require.Equal(t, "PPOHist", hist.Name)

	// The histogram is the difference between the PPO line and its signal.
	for i := range hist.Values {
		require.InDelta(t, ppo.Values[i]-signal.Values[i], hist.Values[i], 1e-9)
	}
}

func TestSuperTrendFollowsPrice(t *testing.T) {
	size := 120
	df := newDataframe(size)
	for i := 0; i < size; i++ {
		df.Close[i] = 100 + float64(i)
		df.High[i] = df.Close[i] + 1
		df.Low[i] = df.Close[i] - 1
	}

	indicator := Spertrend(10, 3, "#f00")
	indicator.Load(df)

	values := indicator.Metrics()[0].Values
	require.Less(t, values[len(values)-1], df.Close[size-1], "the band trails below the close on an uptrend")
}
