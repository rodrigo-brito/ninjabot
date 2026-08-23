package indicator

import (
	"math"
	"testing"

	"github.com/markcheno/go-talib"
	"github.com/stretchr/testify/require"
)

// Sample OHLCV series shared by every wrapper test. The series is long enough
// (100 bars) for the Hilbert Transform indicators, which need a large warmup.
var (
	sampleOpen   = sampleSeries(0.0)
	sampleHigh   = sampleSeries(1.0)
	sampleLow    = sampleSeries(-1.0)
	sampleClose  = sampleSeries(0.5)
	sampleVolume = sampleVolumes()
	samplePeriod = samplePeriods()
)

const sampleSize = 100

// sampleSeries builds a deterministic oscillating price series shifted by
// offset, so that high > close > open > low holds on every bar.
func sampleSeries(offset float64) []float64 {
	values := make([]float64, sampleSize)
	for i := range values {
		values[i] = 100 + 10*math.Sin(float64(i)/5) + float64(i)/4 + offset
	}
	return values
}

func sampleVolumes() []float64 {
	values := make([]float64, sampleSize)
	for i := range values {
		values[i] = 1000 + 300*math.Cos(float64(i)/3)
	}
	return values
}

// samplePeriods feeds MaVp, which takes a per-bar period.
func samplePeriods() []float64 {
	values := make([]float64, sampleSize)
	for i := range values {
		values[i] = float64(2 + i%4)
	}
	return values
}

// oneSeries asserts that a wrapper returning a single series delegates to the
// expected go-talib function.
func oneSeries(t *testing.T, got, want []float64) {
	t.Helper()
	require.Len(t, got, len(sampleClose))
	require.Equal(t, want, got)
}

func TestTalibWrappersSingleSeries(t *testing.T) {
	tests := []struct {
		name string
		got  func() []float64
		want func() []float64
	}{
		// Overlap studies
		{"DEMA", func() []float64 { return DEMA(sampleClose, 3) }, func() []float64 { return talib.Dema(sampleClose, 3) }},
		{"EMA", func() []float64 { return EMA(sampleClose, 3) }, func() []float64 { return talib.Ema(sampleClose, 3) }},
		{"HTTrendline", func() []float64 { return HTTrendline(sampleClose) }, func() []float64 { return talib.HtTrendline(sampleClose) }},
		{"KAMA", func() []float64 { return KAMA(sampleClose, 3) }, func() []float64 { return talib.Kama(sampleClose, 3) }},
		{"MA", func() []float64 { return MA(sampleClose, 3, TypeSMA) }, func() []float64 { return talib.Ma(sampleClose, 3, talib.SMA) }},
		{"MaVp", func() []float64 { return MaVp(sampleClose, samplePeriod, 2, 5, TypeSMA) },
			func() []float64 { return talib.MaVp(sampleClose, samplePeriod, 2, 5, talib.SMA) }},
		{"MidPoint", func() []float64 { return MidPoint(sampleClose, 3) }, func() []float64 { return talib.MidPoint(sampleClose, 3) }},
		{"MidPrice", func() []float64 { return MidPrice(sampleHigh, sampleLow, 3) },
			func() []float64 { return talib.MidPrice(sampleHigh, sampleLow, 3) }},
		{"SAR", func() []float64 { return SAR(sampleHigh, sampleLow, 0.02, 0.2) },
			func() []float64 { return talib.Sar(sampleHigh, sampleLow, 0.02, 0.2) }},
		{"SARExt", func() []float64 { return SARExt(sampleHigh, sampleLow, 0, 0, 0.02, 0.02, 0.2, 0.02, 0.02, 0.2) },
			func() []float64 { return talib.SarExt(sampleHigh, sampleLow, 0, 0, 0.02, 0.02, 0.2, 0.02, 0.02, 0.2) }},
		{"SMA", func() []float64 { return SMA(sampleClose, 3) }, func() []float64 { return talib.Sma(sampleClose, 3) }},
		{"T3", func() []float64 { return T3(sampleClose, 3, 0.7) }, func() []float64 { return talib.T3(sampleClose, 3, 0.7) }},
		{"TEMA", func() []float64 { return TEMA(sampleClose, 3) }, func() []float64 { return talib.Tema(sampleClose, 3) }},
		{"TRIMA", func() []float64 { return TRIMA(sampleClose, 3) }, func() []float64 { return talib.Trima(sampleClose, 3) }},
		{"WMA", func() []float64 { return WMA(sampleClose, 3) }, func() []float64 { return talib.Wma(sampleClose, 3) }},

		// Momentum indicators
		{"ADX", func() []float64 { return ADX(sampleHigh, sampleLow, sampleClose, 3) },
			func() []float64 { return talib.Adx(sampleHigh, sampleLow, sampleClose, 3) }},
		{"ADXR", func() []float64 { return ADXR(sampleHigh, sampleLow, sampleClose, 3) },
			func() []float64 { return talib.AdxR(sampleHigh, sampleLow, sampleClose, 3) }},
		{"APO", func() []float64 { return APO(sampleClose, 3, 6, TypeSMA) },
			func() []float64 { return talib.Apo(sampleClose, 3, 6, talib.SMA) }},
		{"AroonOsc", func() []float64 { return AroonOsc(sampleHigh, sampleLow, 3) },
			func() []float64 { return talib.AroonOsc(sampleHigh, sampleLow, 3) }},
		{"BOP", func() []float64 { return BOP(sampleOpen, sampleHigh, sampleLow, sampleClose) },
			func() []float64 { return talib.Bop(sampleOpen, sampleHigh, sampleLow, sampleClose) }},
		{"CMO", func() []float64 { return CMO(sampleClose, 3) }, func() []float64 { return talib.Cmo(sampleClose, 3) }},
		{"CCI", func() []float64 { return CCI(sampleHigh, sampleLow, sampleClose, 3) },
			func() []float64 { return talib.Cci(sampleHigh, sampleLow, sampleClose, 3) }},
		{"DX", func() []float64 { return DX(sampleHigh, sampleLow, sampleClose, 3) },
			func() []float64 { return talib.Dx(sampleHigh, sampleLow, sampleClose, 3) }},
		{"MinusDI", func() []float64 { return MinusDI(sampleHigh, sampleLow, sampleClose, 3) },
			func() []float64 { return talib.MinusDI(sampleHigh, sampleLow, sampleClose, 3) }},
		{"MinusDM", func() []float64 { return MinusDM(sampleHigh, sampleLow, 3) },
			func() []float64 { return talib.MinusDM(sampleHigh, sampleLow, 3) }},
		{"MFI", func() []float64 { return MFI(sampleHigh, sampleLow, sampleClose, sampleVolume, 3) },
			func() []float64 { return talib.Mfi(sampleHigh, sampleLow, sampleClose, sampleVolume, 3) }},
		{"Momentum", func() []float64 { return Momentum(sampleClose, 3) }, func() []float64 { return talib.Mom(sampleClose, 3) }},
		{"PlusDI", func() []float64 { return PlusDI(sampleHigh, sampleLow, sampleClose, 3) },
			func() []float64 { return talib.PlusDI(sampleHigh, sampleLow, sampleClose, 3) }},
		{"PlusDM", func() []float64 { return PlusDM(sampleHigh, sampleLow, 3) },
			func() []float64 { return talib.PlusDM(sampleHigh, sampleLow, 3) }},
		{"PPO", func() []float64 { return PPO(sampleClose, 3, 6, TypeSMA) },
			func() []float64 { return talib.Ppo(sampleClose, 3, 6, talib.SMA) }},
		{"ROCP", func() []float64 { return ROCP(sampleClose, 3) }, func() []float64 { return talib.Rocp(sampleClose, 3) }},
		{"ROC", func() []float64 { return ROC(sampleClose, 3) }, func() []float64 { return talib.Roc(sampleClose, 3) }},
		{"ROCR", func() []float64 { return ROCR(sampleClose, 3) }, func() []float64 { return talib.Rocr(sampleClose, 3) }},
		{"ROCR100", func() []float64 { return ROCR100(sampleClose, 3) }, func() []float64 { return talib.Rocr100(sampleClose, 3) }},
		{"RSI", func() []float64 { return RSI(sampleClose, 3) }, func() []float64 { return talib.Rsi(sampleClose, 3) }},
		{"Trix", func() []float64 { return Trix(sampleClose, 3) }, func() []float64 { return talib.Trix(sampleClose, 3) }},
		{"UltOsc", func() []float64 { return UltOsc(sampleHigh, sampleLow, sampleClose, 2, 3, 4) },
			func() []float64 { return talib.UltOsc(sampleHigh, sampleLow, sampleClose, 2, 3, 4) }},
		{"WilliamsR", func() []float64 { return WilliamsR(sampleHigh, sampleLow, sampleClose, 3) },
			func() []float64 { return talib.WillR(sampleHigh, sampleLow, sampleClose, 3) }},

		// Volume indicators
		{"Ad", func() []float64 { return Ad(sampleHigh, sampleLow, sampleClose, sampleVolume) },
			func() []float64 { return talib.Ad(sampleHigh, sampleLow, sampleClose, sampleVolume) }},
		{"AdOsc", func() []float64 { return AdOsc(sampleHigh, sampleLow, sampleClose, sampleVolume, 3, 6) },
			func() []float64 { return talib.AdOsc(sampleHigh, sampleLow, sampleClose, sampleVolume, 3, 6) }},
		{"OBV", func() []float64 { return OBV(sampleClose, sampleVolume) },
			func() []float64 { return talib.Obv(sampleClose, sampleVolume) }},

		// Volatility indicators
		{"ATR", func() []float64 { return ATR(sampleHigh, sampleLow, sampleClose, 3) },
			func() []float64 { return talib.Atr(sampleHigh, sampleLow, sampleClose, 3) }},
		{"NATR", func() []float64 { return NATR(sampleHigh, sampleLow, sampleClose, 3) },
			func() []float64 { return talib.Natr(sampleHigh, sampleLow, sampleClose, 3) }},
		{"TRANGE", func() []float64 { return TRANGE(sampleHigh, sampleLow, sampleClose) },
			func() []float64 { return talib.TRange(sampleHigh, sampleLow, sampleClose) }},

		// Price transforms
		{"AvgPrice", func() []float64 { return AvgPrice(sampleOpen, sampleHigh, sampleLow, sampleClose) },
			func() []float64 { return talib.AvgPrice(sampleOpen, sampleHigh, sampleLow, sampleClose) }},
		{"MedPrice", func() []float64 { return MedPrice(sampleHigh, sampleLow) },
			func() []float64 { return talib.MedPrice(sampleHigh, sampleLow) }},
		{"TypPrice", func() []float64 { return TypPrice(sampleHigh, sampleLow, sampleClose) },
			func() []float64 { return talib.TypPrice(sampleHigh, sampleLow, sampleClose) }},
		{"WCLPrice", func() []float64 { return WCLPrice(sampleHigh, sampleLow, sampleClose) },
			func() []float64 { return talib.WclPrice(sampleHigh, sampleLow, sampleClose) }},

		// Cycle indicators
		{"HTDcPeriod", func() []float64 { return HTDcPeriod(sampleClose) }, func() []float64 { return talib.HtDcPeriod(sampleClose) }},
		{"HTDcPhase", func() []float64 { return HTDcPhase(sampleClose) }, func() []float64 { return talib.HtDcPhase(sampleClose) }},
		{"HTTrendMode", func() []float64 { return HTTrendMode(sampleClose) }, func() []float64 { return talib.HtTrendMode(sampleClose) }},

		// Statistic functions
		{"Beta", func() []float64 { return Beta(sampleClose, sampleOpen, 3) },
			func() []float64 { return talib.Beta(sampleClose, sampleOpen, 3) }},
		{"Correl", func() []float64 { return Correl(sampleClose, sampleOpen, 3) },
			func() []float64 { return talib.Correl(sampleClose, sampleOpen, 3) }},
		{"LinearReg", func() []float64 { return LinearReg(sampleClose, 3) }, func() []float64 { return talib.LinearReg(sampleClose, 3) }},
		{"LinearRegAngle", func() []float64 { return LinearRegAngle(sampleClose, 3) },
			func() []float64 { return talib.LinearRegAngle(sampleClose, 3) }},
		{"LinearRegIntercept", func() []float64 { return LinearRegIntercept(sampleClose, 3) },
			func() []float64 { return talib.LinearRegIntercept(sampleClose, 3) }},
		{"LinearRegSlope", func() []float64 { return LinearRegSlope(sampleClose, 3) },
			func() []float64 { return talib.LinearRegSlope(sampleClose, 3) }},
		{"StdDev", func() []float64 { return StdDev(sampleClose, 3, 1) }, func() []float64 { return talib.StdDev(sampleClose, 3, 1) }},
		{"TSF", func() []float64 { return TSF(sampleClose, 3) }, func() []float64 { return talib.Tsf(sampleClose, 3) }},
		{"Var", func() []float64 { return Var(sampleClose, 3) }, func() []float64 { return talib.Var(sampleClose, 3) }},

		// Math transforms
		{"Acos", func() []float64 { return Acos(sampleUnit()) }, func() []float64 { return talib.Acos(sampleUnit()) }},
		{"Asin", func() []float64 { return Asin(sampleUnit()) }, func() []float64 { return talib.Asin(sampleUnit()) }},
		{"Atan", func() []float64 { return Atan(sampleClose) }, func() []float64 { return talib.Atan(sampleClose) }},
		{"Ceil", func() []float64 { return Ceil(sampleClose) }, func() []float64 { return talib.Ceil(sampleClose) }},
		{"Cos", func() []float64 { return Cos(sampleClose) }, func() []float64 { return talib.Cos(sampleClose) }},
		{"Cosh", func() []float64 { return Cosh(sampleUnit()) }, func() []float64 { return talib.Cosh(sampleUnit()) }},
		{"Exp", func() []float64 { return Exp(sampleUnit()) }, func() []float64 { return talib.Exp(sampleUnit()) }},
		{"Floor", func() []float64 { return Floor(sampleClose) }, func() []float64 { return talib.Floor(sampleClose) }},
		{"Ln", func() []float64 { return Ln(sampleClose) }, func() []float64 { return talib.Ln(sampleClose) }},
		{"Log10", func() []float64 { return Log10(sampleClose) }, func() []float64 { return talib.Log10(sampleClose) }},
		{"Sin", func() []float64 { return Sin(sampleClose) }, func() []float64 { return talib.Sin(sampleClose) }},
		{"Sinh", func() []float64 { return Sinh(sampleUnit()) }, func() []float64 { return talib.Sinh(sampleUnit()) }},
		{"Sqrt", func() []float64 { return Sqrt(sampleClose) }, func() []float64 { return talib.Sqrt(sampleClose) }},
		{"Tan", func() []float64 { return Tan(sampleClose) }, func() []float64 { return talib.Tan(sampleClose) }},
		{"Tanh", func() []float64 { return Tanh(sampleClose) }, func() []float64 { return talib.Tanh(sampleClose) }},

		// Math operators
		{"Add", func() []float64 { return Add(sampleHigh, sampleLow) }, func() []float64 { return talib.Add(sampleHigh, sampleLow) }},
		{"Div", func() []float64 { return Div(sampleHigh, sampleLow) }, func() []float64 { return talib.Div(sampleHigh, sampleLow) }},
		{"Max", func() []float64 { return Max(sampleClose, 3) }, func() []float64 { return talib.Max(sampleClose, 3) }},
		{"MaxIndex", func() []float64 { return MaxIndex(sampleClose, 3) }, func() []float64 { return talib.MaxIndex(sampleClose, 3) }},
		{"Min", func() []float64 { return Min(sampleClose, 3) }, func() []float64 { return talib.Min(sampleClose, 3) }},
		{"MinIndex", func() []float64 { return MinIndex(sampleClose, 3) }, func() []float64 { return talib.MinIndex(sampleClose, 3) }},
		{"Mult", func() []float64 { return Mult(sampleHigh, sampleLow) }, func() []float64 { return talib.Mult(sampleHigh, sampleLow) }},
		{"Sub", func() []float64 { return Sub(sampleHigh, sampleLow) }, func() []float64 { return talib.Sub(sampleHigh, sampleLow) }},
		{"Sum", func() []float64 { return Sum(sampleClose, 3) }, func() []float64 { return talib.Sum(sampleClose, 3) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oneSeries(t, tt.got(), tt.want())
		})
	}
}

func TestTalibWrappersMultiSeries(t *testing.T) {
	t.Run("BB returns upper, middle and lower bands", func(t *testing.T) {
		upper, middle, lower := BB(sampleClose, 5, 2, TypeSMA)
		wantUpper, wantMiddle, wantLower := talib.BBands(sampleClose, 5, 2, 2, talib.SMA)

		require.Equal(t, wantUpper, upper)
		require.Equal(t, wantMiddle, middle)
		require.Equal(t, wantLower, lower)
		require.Greater(t, upper[len(upper)-1], lower[len(lower)-1])
	})

	t.Run("MAMA", func(t *testing.T) {
		mama, fama := MAMA(sampleClose, 0.5, 0.05)
		wantMama, wantFama := talib.Mama(sampleClose, 0.5, 0.05)

		require.Equal(t, wantMama, mama)
		require.Equal(t, wantFama, fama)
	})

	t.Run("Aroon", func(t *testing.T) {
		down, up := Aroon(sampleHigh, sampleLow, 3)
		wantDown, wantUp := talib.Aroon(sampleHigh, sampleLow, 3)

		require.Equal(t, wantDown, down)
		require.Equal(t, wantUp, up)
	})

	t.Run("MACD", func(t *testing.T) {
		macd, signal, hist := MACD(sampleClose, 3, 6, 3)
		wantMACD, wantSignal, wantHist := talib.Macd(sampleClose, 3, 6, 3)

		require.Equal(t, wantMACD, macd)
		require.Equal(t, wantSignal, signal)
		require.Equal(t, wantHist, hist)
	})

	t.Run("MACDExt", func(t *testing.T) {
		macd, signal, hist := MACDExt(sampleClose, 3, TypeSMA, 6, TypeSMA, 3, TypeSMA)
		wantMACD, wantSignal, wantHist := talib.MacdExt(sampleClose, 3, talib.SMA, 6, talib.SMA, 3, talib.SMA)

		require.Equal(t, wantMACD, macd)
		require.Equal(t, wantSignal, signal)
		require.Equal(t, wantHist, hist)
	})

	t.Run("MACDFix", func(t *testing.T) {
		macd, signal, hist := MACDFix(sampleClose, 3)
		wantMACD, wantSignal, wantHist := talib.MacdFix(sampleClose, 3)

		require.Equal(t, wantMACD, macd)
		require.Equal(t, wantSignal, signal)
		require.Equal(t, wantHist, hist)
	})

	t.Run("Stoch", func(t *testing.T) {
		slowK, slowD := Stoch(sampleHigh, sampleLow, sampleClose, 5, 3, TypeSMA, 3, TypeSMA)
		wantK, wantD := talib.Stoch(sampleHigh, sampleLow, sampleClose, 5, 3, talib.SMA, 3, talib.SMA)

		require.Equal(t, wantK, slowK)
		require.Equal(t, wantD, slowD)
	})

	t.Run("StochF", func(t *testing.T) {
		fastK, fastD := StochF(sampleHigh, sampleLow, sampleClose, 5, 3, TypeSMA)
		wantK, wantD := talib.StochF(sampleHigh, sampleLow, sampleClose, 5, 3, talib.SMA)

		require.Equal(t, wantK, fastK)
		require.Equal(t, wantD, fastD)
	})

	t.Run("StochRSI", func(t *testing.T) {
		fastK, fastD := StochRSI(sampleClose, 5, 5, 3, TypeSMA)
		wantK, wantD := talib.StochRsi(sampleClose, 5, 5, 3, talib.SMA)

		require.Equal(t, wantK, fastK)
		require.Equal(t, wantD, fastD)
	})

	t.Run("HTPhasor", func(t *testing.T) {
		inPhase, quadrature := HTPhasor(sampleClose)
		wantInPhase, wantQuadrature := talib.HtPhasor(sampleClose)

		require.Equal(t, wantInPhase, inPhase)
		require.Equal(t, wantQuadrature, quadrature)
	})

	t.Run("HTSine", func(t *testing.T) {
		sine, leadSine := HTSine(sampleClose)
		wantSine, wantLeadSine := talib.HtSine(sampleClose)

		require.Equal(t, wantSine, sine)
		require.Equal(t, wantLeadSine, leadSine)
	})

	t.Run("MinMax", func(t *testing.T) {
		min, max := MinMax(sampleClose, 3)
		wantMin, wantMax := talib.MinMax(sampleClose, 3)

		require.Equal(t, wantMin, min)
		require.Equal(t, wantMax, max)
	})

	t.Run("MinMaxIndex", func(t *testing.T) {
		minIdx, maxIdx := MinMaxIndex(sampleClose, 3)
		wantMin, wantMax := talib.MinMaxIndex(sampleClose, 3)

		require.Equal(t, wantMin, minIdx)
		require.Equal(t, wantMax, maxIdx)
	})
}

// sampleUnit returns the close series normalized to [-1, 1], required by the
// inverse trigonometric wrappers.
func sampleUnit() []float64 {
	values := make([]float64, len(sampleClose))
	for i, v := range sampleClose {
		values[i] = v/200 - 0.5
	}
	return values
}
