package indicator

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/ui"
)

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
