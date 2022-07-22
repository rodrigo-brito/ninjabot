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
func MAMA(input []float64, fastLimit float64, slowLimit float64) ([]float64, []float64) {
	return talib.Mama(input, fastLimit, slowLimit)
}

func MaVp(input []float64, periods []float64, minPeriod int, maxPeriod int, maType MaType) []float64 {
	return talib.MaVp(input, periods, minPeriod, maxPeriod, maType)
}

func MidPoint(input []float64, period int) []float64 {
	return talib.MidPoint(input, period)
}

func MidPrice(high []float64, low []float64, period int) []float64 {
	return talib.MidPrice(high, low, period)
}

// SAR - parabolic SAR
func SAR(high []float64, low []float64, inAcceleration float64, inMaximum float64) []float64 {
	return talib.Sar(high, low, inAcceleration, inMaximum)
}

func SARExt(high []float64, low []float64,
	startValue float64,
	offsetOnReverse float64,
	accelerationInitLong float64,
	accelerationLong float64,
	accelerationMaxLong float64,
	accelerationInitShort float64,
	accelerationShort float64,
	accelerationMaxShort float64) []float64 {
	return talib.SarExt(high, low, startValue, offsetOnReverse, accelerationInitLong, accelerationLong,
		accelerationMaxLong, accelerationInitShort, accelerationShort, accelerationMaxShort)
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
func ADX(high []float64, low []float64, close []float64, period int) []float64 {
	return talib.Adx(high, low, close, period)
}

// ADXR - Average Directional Movement Index Rating
func ADXR(high []float64, low []float64, close []float64, period int) []float64 {
	return talib.AdxR(high, low, close, period)
}

// APO - Absolute Price Oscillator
func APO(input []float64, fastPeriod int, slowPeriod int, maType MaType) []float64 {
	return talib.Apo(input, fastPeriod, slowPeriod, maType)
}

func Aroon(high []float64, low []float64, period int) ([]float64, []float64) {
	return talib.Aroon(high, low, period)
}

func AroonOsc(high []float64, low []float64, period int) []float64 {
	return talib.AroonOsc(high, low, period)
}

// BOP - Balance Of Power
func BOP(inOpen []float64, high []float64, low []float64, close []float64) []float64 {
	return talib.Bop(inOpen, high, low, close)
}

// CMO - Chande Momentum Oscillator
func CMO(input []float64, period int) []float64 {
	return talib.Cmo(input, period)
}

// CCI - commodity channel index
func CCI(high []float64, low []float64, close []float64, period int) []float64 {
	return talib.Cci(high, low, close, period)
}

// DX - Directional Movement Index
func DX(high []float64, low []float64, close []float64, period int) []float64 {
	return talib.Dx(high, low, close, period)
}

// MACD - moving average convergence/divergence
func MACD(input []float64, fastPeriod int, slowPeriod int, signalPeriod int) ([]float64, []float64, []float64) {
	return talib.Macd(input, fastPeriod, slowPeriod, signalPeriod)
}

func MACDExt(input []float64, fastPeriod int, fastMAType MaType, slowPeriod int, inSlowMAType MaType,
	signalPeriod int, signalMAType MaType) ([]float64, []float64, []float64) {
	return talib.MacdExt(input, fastPeriod, fastMAType, slowPeriod, inSlowMAType, signalPeriod, signalMAType)
}

func MACDFix(input []float64, signalPeriod int) ([]float64, []float64, []float64) {
	return talib.MacdFix(input, signalPeriod)
}

func MinusDI(high []float64, low []float64, close []float64, period int) []float64 {
	return talib.MinusDI(high, low, close, period)
}

func MinusDM(high []float64, low []float64, period int) []float64 {
	return talib.MinusDM(high, low, period)
}

// MFI - money flow index
func MFI(high []float64, low []float64, close []float64, volume []float64, period int) []float64 {
	return talib.Mfi(high, low, close, volume, period)
}

func Momentum(input []float64, period int) []float64 {
	return talib.Mom(input, period)
}

func PlusDI(high []float64, low []float64, close []float64, period int) []float64 {
	return talib.PlusDI(high, low, close, period)
}

func PlusDM(high []float64, low []float64, period int) []float64 {
	return talib.PlusDM(high, low, period)
}

// PPO - Percentage Price Oscillator
func PPO(input []float64, fastPeriod int, slowPeriod int, maType MaType) []float64 {
	return talib.Ppo(input, fastPeriod, slowPeriod, maType)
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
func Stoch(high []float64, low []float64, close []float64, fastKPeriod int, slowKPeriod int,
	slowKMAType MaType, slowDPeriod int, slowDMAType MaType) ([]float64, []float64) {

	return talib.Stoch(high, low, close, fastKPeriod, slowKPeriod, slowKMAType, slowDPeriod, slowDMAType)
}

// StochF is fast stochastic indicator.
func StochF(high []float64, low []float64, close []float64, fastKPeriod int, fastDPeriod int,
	fastDMAType MaType) ([]float64, []float64) {

	return talib.StochF(high, low, close, fastKPeriod, fastDPeriod, fastDMAType)
}

// StochRSI is stochastic RSI indicator.
func StochRSI(input []float64, period int, fastKPeriod int, fastDPeriod int, fastDMAType MaType) ([]float64,
	[]float64) {

	return talib.StochRsi(input, period, fastKPeriod, fastDPeriod, fastDMAType)
}

func Trix(input []float64, period int) []float64 {
	return talib.Trix(input, period)
}

func UltOsc(high []float64, low []float64, close []float64, period1 int, period2 int, period3 int) []float64 {
	return talib.UltOsc(high, low, close, period1, period2, period3)
}

// WilliamsR - Williams %R indicator.
func WilliamsR(high []float64, low []float64, close []float64, period int) []float64 {
	return talib.WillR(high, low, close, period)
}

func Ad(high []float64, low []float64, close []float64, volume []float64) []float64 {
	return talib.Ad(high, low, close, volume)
}

func AdOsc(high []float64, low []float64, close []float64, volume []float64, fastPeriod int,
	slowPeriod int) []float64 {
	return talib.AdOsc(high, low, close, volume, fastPeriod, slowPeriod)
}

// OBV is the On Balance Volume indicator.
func OBV(input []float64, volume []float64) []float64 {
	return talib.Obv(input, volume)
}

// ATR is the Average True Range indicator.
func ATR(high []float64, low []float64, close []float64, period int) []float64 {
	return talib.Atr(high, low, close, period)
}

// NATR is the normalized Average True Range indicator.
func NATR(high []float64, low []float64, close []float64, period int) []float64 {
	return talib.Natr(high, low, close, period)
}

// TRANGE is the True Range indicator.
func TRANGE(high []float64, low []float64, close []float64) []float64 {
	return talib.TRange(high, low, close)
}

func AvgPrice(inOpen []float64, high []float64, low []float64, close []float64) []float64 {
	return talib.AvgPrice(inOpen, high, low, close)
}

func MedPrice(high []float64, low []float64) []float64 {
	return talib.MedPrice(high, low)
}

func TypPrice(high []float64, low []float64, close []float64) []float64 {
	return talib.TypPrice(high, low, close)
}

func WCLPrice(high []float64, low []float64, close []float64) []float64 {
	return talib.WclPrice(high, low, close)
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

func Add(input0, input1 []float64) []float64 {
	return talib.Add(input0, input1)
}

func Div(input0, input1 []float64) []float64 {
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

func Mult(input0, input1 []float64) []float64 {
	return talib.Mult(input0, input1)
}

func Sub(input0, input1 []float64) []float64 {
	return talib.Sub(input0, input1)
}

func Sum(input []float64, period int) []float64 {
	return talib.Sum(input, period)
}
