package indicator

import "github.com/markcheno/go-talib"

type MaType = talib.MaType

// Kinds of moving averages
const (
	TypeSMA   = talib.SMA
	TypeEMA   = talib.EMA
	TypeWMA   = talib.WMA
	TypeDEMA  = talib.DEMA
	TypeTEMA  = talib.TEMA
	TypeTRIMA = talib.TRIMA
	TypeKAMA  = talib.KAMA
	TypeMAMA  = talib.MAMA
	TypeT3MA  = talib.T3MA
)

// BB - Bollinger Bands
func BB(input []float64, period int, deviation float64, maType MaType) ([]float64, []float64, []float64) {
	return talib.BBands(input, period, deviation, deviation, maType)
}

// DEMA - double exponential moving average
func DEMA(input []float64, period int) []float64 {
	return talib.Dema(input, period)
}

// EMA - exponential moving average
func EMA(input []float64, period int) []float64 {
	return talib.Ema(input, period)
}

func HTTrendline(input []float64) []float64 {
	return talib.HtTrendline(input)
}

// KAMA - Kaufman Adaptive Moving Average
func KAMA(input []float64, period int) []float64 {
	return talib.Kama(input, period)
}

// MA - moving average
func MA(input []float64, period int, maType MaType) []float64 {
	return talib.Ma(input, period, maType)
}

// MAMA - moving average convergence/divergence
func MAMA(input []float64, inFastLimit float64, inSlowLimit float64) ([]float64, []float64) {
	return talib.Mama(input, inFastLimit, inSlowLimit)
}

func MaVp(input []float64, inPeriods []float64, inMinPeriod int, inMaxPeriod int, maType MaType) []float64 {
	return talib.MaVp(input, inPeriods, inMinPeriod, inMaxPeriod, maType)
}

func MidPoint(input []float64, period int) []float64 {
	return talib.MidPoint(input, period)
}

func MidPrice(inHigh []float64, inLow []float64, period int) []float64 {
	return talib.MidPrice(inHigh, inLow, period)
}

// SAR - parabolic SAR
func SAR(inHigh []float64, inLow []float64, inAcceleration float64, inMaximum float64) []float64 {
	return talib.Sar(inHigh, inLow, inAcceleration, inMaximum)
}

func SARExt(inHigh []float64, inLow []float64,
	inStartValue float64,
	inOffsetOnReverse float64,
	inAccelerationInitLong float64,
	inAccelerationLong float64,
	inAccelerationMaxLong float64,
	inAccelerationInitShort float64,
	inAccelerationShort float64,
	inAccelerationMaxShort float64) []float64 {
	return talib.SarExt(inHigh, inLow, inStartValue, inOffsetOnReverse, inAccelerationInitLong, inAccelerationLong,
		inAccelerationMaxLong, inAccelerationInitShort, inAccelerationShort, inAccelerationMaxShort)
}

// SMA - simple moving average
func SMA(input []float64, period int) []float64 {
	return talib.Sma(input, period)
}

// T3 - Triple Exponential Moving Average (T3)
func T3(input []float64, period int, inVFactor float64) []float64 {
	return talib.T3(input, period, inVFactor)
}

// TEMA - triple exponential moving average
func TEMA(input []float64, period int) []float64 {
	return talib.Tema(input, period)
}

// TRIMA - Triangular Moving Average
func TRIMA(input []float64, period int) []float64 {
	return talib.Trima(input, period)
}

// WMA - weighted moving average
func WMA(input []float64, period int) []float64 {
	return talib.Wma(input, period)
}

// ADX - relative strength index
func ADX(inHigh []float64, inLow []float64, inClose []float64, period int) []float64 {
	return talib.Adx(inHigh, inLow, inClose, period)
}

// ADXR - Average Directional Movement Index Rating
func ADXR(inHigh []float64, inLow []float64, inClose []float64, period int) []float64 {
	return talib.AdxR(inHigh, inLow, inClose, period)
}

// APO - Absolute Price Oscillator
func APO(input []float64, inFastPeriod int, inSlowPeriod int, maType MaType) []float64 {
	return talib.Apo(input, inFastPeriod, inSlowPeriod, maType)
}

func Aroon(inHigh []float64, inLow []float64, period int) ([]float64, []float64) {
	return talib.Aroon(inHigh, inLow, period)
}

func AroonOsc(inHigh []float64, inLow []float64, period int) []float64 {
	return talib.AroonOsc(inHigh, inLow, period)
}

// BOP - Balance Of Power
func BOP(inOpen []float64, inHigh []float64, inLow []float64, inClose []float64) []float64 {
	return talib.Bop(inOpen, inHigh, inLow, inClose)
}

// CMO - Chande Momentum Oscillator
func CMO(input []float64, period int) []float64 {
	return talib.Cmo(input, period)
}

// CCI - commodity channel index
func CCI(inHigh []float64, inLow []float64, inClose []float64, period int) []float64 {
	return talib.Cci(inHigh, inLow, inClose, period)
}

// DX - Directional Movement Index
func DX(inHigh []float64, inLow []float64, inClose []float64, period int) []float64 {
	return talib.Dx(inHigh, inLow, inClose, period)
}

// MACD - moving average convergence/divergence
func MACD(input []float64, inFastPeriod int, inSlowPeriod int, inSignalPeriod int) ([]float64, []float64, []float64) {
	return talib.Macd(input, inFastPeriod, inSlowPeriod, inSignalPeriod)
}

func MACDExt(input []float64, inFastPeriod int, inFastMAType MaType, inSlowPeriod int, inSlowMAType MaType,
	inSignalPeriod int, inSignalMAType MaType) ([]float64, []float64, []float64) {
	return talib.MacdExt(input, inFastPeriod, inFastMAType, inSlowPeriod, inSlowMAType, inSignalPeriod, inSignalMAType)
}

func MACDFix(input []float64, inSignalPeriod int) ([]float64, []float64, []float64) {
	return talib.MacdFix(input, inSignalPeriod)
}

func MinusDI(inHigh []float64, inLow []float64, inClose []float64, period int) []float64 {
	return talib.MinusDI(inHigh, inLow, inClose, period)
}

func MinusDM(inHigh []float64, inLow []float64, period int) []float64 {
	return talib.MinusDM(inHigh, inLow, period)
}

// MFI - money flow index
func MFI(inHigh []float64, inLow []float64, inClose []float64, inVolume []float64, period int) []float64 {
	return talib.Mfi(inHigh, inLow, inClose, inVolume, period)
}

func Momentum(input []float64, period int) []float64 {
	return talib.Mom(input, period)
}

func PlusDI(inHigh []float64, inLow []float64, inClose []float64, period int) []float64 {
	return talib.PlusDI(inHigh, inLow, inClose, period)
}

func PlusDM(inHigh []float64, inLow []float64, period int) []float64 {
	return talib.PlusDM(inHigh, inLow, period)
}

// PPO - Percentage Price Oscillator
func PPO(input []float64, inFastPeriod int, inSlowPeriod int, maType MaType) []float64 {
	return talib.Ppo(input, inFastPeriod, inSlowPeriod, maType)
}

// ROCP - Rate of change Percentage: (price-prevPrice)/prevPrice
func ROCP(input []float64, period int) []float64 {
	return talib.Rocp(input, period)
}

// ROC - Rate of change : ((price/prevPrice)-1)*100
func ROC(input []float64, period int) []float64 {
	return talib.Roc(input, period)
}

// ROCR - Rate of change ratio: (price/prevPrice)
func ROCR(input []float64, period int) []float64 {
	return talib.Rocr(input, period)
}

// ROCR100 - Rate of change ratio 100 scale: (price/prevPrice)*100
func ROCR100(input []float64, period int) []float64 {
	return talib.Rocr100(input, period)
}

// RSI - relative strength index.
func RSI(input []float64, period int) []float64 {
	return talib.Rsi(input, period)
}

// Stoch is slow stochastic indicator.
func Stoch(inHigh []float64, inLow []float64, inClose []float64, inFastKPeriod int, inSlowKPeriod int,
	inSlowKMAType MaType, inSlowDPeriod int, inSlowDMAType MaType) ([]float64, []float64) {

	return talib.Stoch(inHigh, inLow, inClose, inFastKPeriod, inSlowKPeriod, inSlowKMAType, inSlowDPeriod, inSlowDMAType)
}

// StochF is fast stochastic indicator.
func StochF(inHigh []float64, inLow []float64, inClose []float64, inFastKPeriod int, inFastDPeriod int,
	inFastDMAType MaType) ([]float64, []float64) {

	return talib.StochF(inHigh, inLow, inClose, inFastKPeriod, inFastDPeriod, inFastDMAType)
}

// StochRSI is stochastic RSI indicator.
func StochRSI(input []float64, period int, inFastKPeriod int, inFastDPeriod int, inFastDMAType MaType) ([]float64,
	[]float64) {

	return talib.StochRsi(input, period, inFastKPeriod, inFastDPeriod, inFastDMAType)
}

func Trix(input []float64, period int) []float64 {
	return talib.Trix(input, period)
}

func UltOsc(inHigh []float64, inLow []float64, inClose []float64, period1 int, period2 int, period3 int) []float64 {
	return talib.UltOsc(inHigh, inLow, inClose, period1, period2, period3)
}

// WilliamsR - Williams %R indicator.
func WilliamsR(inHigh []float64, inLow []float64, inClose []float64, period int) []float64 {
	return talib.WillR(inHigh, inLow, inClose, period)
}

func Ad(inHigh []float64, inLow []float64, inClose []float64, inVolume []float64) []float64 {
	return talib.Ad(inHigh, inLow, inClose, inVolume)
}

func AdOsc(inHigh []float64, inLow []float64, inClose []float64, inVolume []float64, inFastPeriod int, inSlowPeriod int) []float64 {
	return talib.AdOsc(inHigh, inLow, inClose, inVolume, inFastPeriod, inSlowPeriod)
}

// OBV is the On Balance Volume indicator.
func OBV(input []float64, inVolume []float64) []float64 {
	return talib.Obv(input, inVolume)
}

// ATR is the Average True Range indicator.
func ATR(inHigh []float64, inLow []float64, inClose []float64, period int) []float64 {
	return talib.Atr(inHigh, inLow, inClose, period)
}

// NATR is the normalized Average True Range indicator.
func Natr(inHigh []float64, inLow []float64, inClose []float64, period int) []float64 {
	return talib.Natr(inHigh, inLow, inClose, period)
}

// TRANGE is the True Range indicator.
func TRANGE(inHigh []float64, inLow []float64, inClose []float64) []float64 {
	return talib.TRange(inHigh, inLow, inClose)
}

func AvgPrice(inOpen []float64, inHigh []float64, inLow []float64, inClose []float64) []float64 {
	return talib.AvgPrice(inOpen, inHigh, inLow, inClose)
}

func MedPrice(inHigh []float64, inLow []float64) []float64 {
	return talib.MedPrice(inHigh, inLow)
}

func TypPrice(inHigh []float64, inLow []float64, inClose []float64) []float64 {
	return talib.TypPrice(inHigh, inLow, inClose)
}

func WCLPrice(inHigh []float64, inLow []float64, inClose []float64) []float64 {
	return talib.WclPrice(inHigh, inLow, inClose)
}

func HTDcPeriod(input []float64) []float64 {
	return talib.HtDcPeriod(input)
}

func HTDcPhase(input []float64) []float64 {
	return talib.HtDcPhase(input)
}

func HTPhasor(input []float64) ([]float64, []float64) {
	return talib.HtPhasor(input)
}

func HTSine(input []float64) ([]float64, []float64) {
	return talib.HtSine(input)
}

func HTTrendMode(input []float64) []float64 {
	return talib.HtTrendMode(input)
}

func Beta(input0 []float64, input1 []float64, period int) []float64 {
	return talib.Beta(input0, input1, period)
}

func Correl(input0 []float64, input1 []float64, period int) []float64 {
	return talib.Correl(input0, input1, period)
}

func LinearReg(input []float64, period int) []float64 {
	return talib.LinearReg(input, period)
}

func LinearRegAngle(input []float64, period int) []float64 {
	return talib.LinearRegAngle(input, period)
}

func LinearRegIntercept(input []float64, period int) []float64 {
	return talib.LinearRegIntercept(input, period)
}

func LinearRegSlope(input []float64, period int) []float64 {
	return talib.LinearRegSlope(input, period)
}

func StdDev(input []float64, period int, inNbDev float64) []float64 {
	return talib.StdDev(input, period, inNbDev)
}

// TSF - Time Series Forecast
func TSF(input []float64, period int) []float64 {
	return talib.Tsf(input, period)
}

func Var(input []float64, period int) []float64 {
	return talib.Var(input, period)
}

/* Math Transform Functions */

func Acos(input []float64) []float64 {
	return talib.Acos(input)
}

func Asin(input []float64) []float64 {
	return talib.Asin(input)
}

func Atan(input []float64) []float64 {
	return talib.Atan(input)
}

func Ceil(input []float64) []float64 {
	return talib.Ceil(input)
}

func Cos(input []float64) []float64 {
	return talib.Cos(input)
}

func Cosh(input []float64) []float64 {
	return talib.Cosh(input)
}

func Exp(input []float64) []float64 {
	return talib.Exp(input)
}

func Floor(input []float64) []float64 {
	return talib.Floor(input)
}

func Ln(input []float64) []float64 {
	return talib.Ln(input)
}

func Log10(input []float64) []float64 {
	return talib.Log10(input)
}

func Sin(input []float64) []float64 {
	return talib.Sin(input)
}

func Sinh(input []float64) []float64 {
	return talib.Sinh(input)
}

func Sqrt(input []float64) []float64 {
	return talib.Sqrt(input)
}

func Tan(input []float64) []float64 {
	return talib.Tan(input)
}

func Tanh(input []float64) []float64 {
	return talib.Tanh(input)
}

/* Math Operator Functions */

func Add(input0 []float64, input1 []float64) []float64 {
	return talib.Add(input0, input1)
}

func Div(input0 []float64, input1 []float64) []float64 {
	return talib.Div(input0, input1)
}

func Max(input []float64, period int) []float64 {
	return talib.Max(input, period)
}

func MaxIndex(input []float64, period int) []float64 {
	return talib.MaxIndex(input, period)
}

func Min(input []float64, period int) []float64 {
	return talib.Min(input, period)
}

func MinIndex(input []float64, period int) []float64 {
	return talib.MinIndex(input, period)
}

func MinMax(input []float64, period int) ([]float64, []float64) {
	return talib.MinMax(input, period)
}

func MinMaxIndex(input []float64, period int) ([]float64, []float64) {
	return talib.MinMaxIndex(input, period)
}

func Mult(input0 []float64, input1 []float64) []float64 {
	return talib.Mult(input0, input1)
}

func Sub(input0 []float64, input1 []float64) []float64 {
	return talib.Sub(input0, input1)
}

func Sum(input []float64, period int) []float64 {
	return talib.Sum(input, period)
}
