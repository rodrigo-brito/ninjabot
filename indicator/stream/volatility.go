package stream

import (
	"math"

	"github.com/rodrigo-brito/ninjabot/model"
)

// trueRange is the true range of c given the previous close. Like talib, the
// first bar has a true range of zero.
func trueRange(c *model.Candle, previousClose float64) float64 {
	greatest := c.High - c.Low
	if value := math.Abs(previousClose - c.High); value > greatest {
		greatest = value
	}
	if value := math.Abs(previousClose - c.Low); value > greatest {
		greatest = value
	}
	return greatest
}

// ATR is the average true range with Wilder smoothing.
type ATR struct {
	period        int
	count         int
	previousClose float64
	sum           float64
	value         float64
}

// NewATR creates an average true range. A period of one yields the true
// range; a period below one always yields zero.
func NewATR(period int) *ATR { return &ATR{period: period} }

func (a *ATR) Update(c *model.Candle) float64 {
	index := a.count
	a.count++
	previousClose := a.previousClose
	a.previousClose = c.Close

	if a.period < 1 || index == 0 {
		return 0
	}
	tr := trueRange(c, previousClose)
	if a.period == 1 {
		a.value = tr
		return tr
	}

	n := float64(a.period)
	switch {
	case index < a.period:
		a.sum += tr
		return 0
	case index == a.period:
		a.sum += tr
		a.value = a.sum / n
	default:
		a.value *= n - 1.0
		a.value += tr
		a.value /= n
	}
	return a.value
}

func (a *ATR) Preview(c *model.Candle) float64 {
	next := *a
	return next.Update(c)
}

func (a *ATR) Value() float64 { return a.value }
func (a *ATR) Ready() bool    { return a.period >= 1 && a.count > a.Warmup() }

func (a *ATR) Warmup() int {
	if a.period == 1 {
		return 1
	}
	return a.period
}

func (a *ATR) Reset() { *a = ATR{period: a.period} }

// Variance is the population variance of the last period values.
type Variance struct {
	period     int
	window     ring
	sum, sumSq float64
	value      float64
}

// NewVariance creates a rolling population variance.
func NewVariance(period int) *Variance {
	if period < 1 {
		panic("stream: variance period must be positive")
	}
	return &Variance{period: period, window: newRing(period)}
}

func (v *Variance) Update(x float64) float64 {
	v.window.push(x)
	v.sum += x
	v.sumSq += x * x
	if v.window.count < v.period {
		return 0
	}
	n := float64(v.period)
	mean, meanSq := v.sum/n, v.sumSq/n
	trailing := v.window.at(v.window.count - v.period)
	v.sum -= trailing
	v.sumSq -= trailing * trailing
	v.value = meanSq - mean*mean
	return v.value
}

func (v *Variance) Preview(x float64) float64 {
	if v.window.count+1 < v.period {
		return 0
	}
	n := float64(v.period)
	mean, meanSq := (v.sum+x)/n, (v.sumSq+x*x)/n
	return meanSq - mean*mean
}

func (v *Variance) Value() float64 { return v.value }
func (v *Variance) Ready() bool    { return v.window.count >= v.period }
func (v *Variance) Warmup() int    { return v.period - 1 }

func (v *Variance) Reset() {
	v.window.reset()
	v.sum, v.sumSq, v.value = 0, 0, 0
}

// StdDev is the population standard deviation of the last period values,
// scaled by a number of deviations.
type StdDev struct {
	variance   Variance
	deviations float64
	value      float64
}

// NewStdDev creates a rolling standard deviation times deviations.
func NewStdDev(period int, deviations float64) *StdDev {
	return &StdDev{variance: *NewVariance(period), deviations: deviations}
}

func deviation(variance, deviations float64) float64 {
	if variance < 0.00000000000001 {
		return 0
	}
	if deviations != 1.0 {
		return math.Sqrt(variance) * deviations
	}
	return math.Sqrt(variance)
}

func (s *StdDev) Update(x float64) float64 {
	s.value = deviation(s.variance.Update(x), s.deviations)
	return s.value
}

func (s *StdDev) Preview(x float64) float64 {
	return deviation(s.variance.Preview(x), s.deviations)
}

func (s *StdDev) Value() float64 { return s.value }
func (s *StdDev) Ready() bool    { return s.variance.Ready() }
func (s *StdDev) Warmup() int    { return s.variance.Warmup() }

func (s *StdDev) Reset() {
	s.variance.Reset()
	s.value = 0
}

// BB is the Bollinger Bands over the close: a simple moving average and the
// bands deviations standard deviations above and below it. Its outputs are
// the upper, middle and lower bands.
type BB struct {
	middle     Line
	variance   Variance
	deviations float64
	values     [3]float64
}

// NewBB creates Bollinger Bands with a simple moving average.
func NewBB(period int, deviations float64) *BB {
	return &BB{middle: NewSMA(period), variance: *NewVariance(period), deviations: deviations}
}

// NewBBEMA creates Bollinger Bands whose middle band is an exponential
// moving average.
func NewBBEMA(period int, deviations float64) *BB {
	return &BB{middle: NewEMA(period), variance: *NewVariance(period), deviations: deviations}
}

func (b *BB) Outputs() []string { return []string{"upper", "middle", "lower"} }

func bands(middle, variance, deviations float64) [3]float64 {
	width := deviation(variance, 1.0)
	return [3]float64{middle + width*deviations, middle, middle - width*deviations}
}

// UpdateValue adds the value of a new bar.
func (b *BB) UpdateValue(v float64) []float64 {
	b.values = bands(b.middle.Update(v), b.variance.Update(v), b.deviations)
	return b.values[:]
}

func (b *BB) Update(c *model.Candle) []float64 { return b.UpdateValue(c.Close) }

func (b *BB) Preview(c *model.Candle) []float64 {
	values := bands(b.middle.Preview(c.Close), b.variance.Preview(c.Close), b.deviations)
	return values[:]
}

func (b *BB) Values() []float64 {
	values := b.values
	return values[:]
}
func (b *BB) Ready() bool { return b.middle.Ready() }
func (b *BB) Warmup() int { return b.middle.Warmup() }

func (b *BB) Reset() {
	b.middle.Reset()
	b.variance.Reset()
	b.values = [3]float64{}
}

// SuperTrend follows the trend with ATR bands around the median price. It
// reproduces indicator.SuperTrend.
type SuperTrend struct {
	atr                    ATR
	factor                 float64
	count                  int
	previousClose          float64
	finalUpper, finalLower float64
	value                  float64
}

// NewSuperTrend creates a SuperTrend with an ATR of atrPeriod bars.
func NewSuperTrend(atrPeriod int, factor float64) *SuperTrend {
	return &SuperTrend{atr: *NewATR(atrPeriod), factor: factor}
}

func (s *SuperTrend) Update(c *model.Candle) float64 {
	atr := s.atr.Update(c)
	index := s.count
	s.count++
	previousClose := s.previousClose
	s.previousClose = c.Close
	if index == 0 {
		return 0
	}

	basicUpper := (c.High+c.Low)/2.0 + atr*s.factor
	basicLower := (c.High+c.Low)/2.0 - atr*s.factor

	finalUpper := s.finalUpper
	if basicUpper < s.finalUpper || previousClose > s.finalUpper {
		finalUpper = basicUpper
	}
	finalLower := s.finalLower
	if basicLower > s.finalLower || previousClose < s.finalLower {
		finalLower = basicLower
	}

	var value float64
	if s.finalUpper == s.value {
		if c.Close > finalUpper {
			value = finalLower
		} else {
			value = finalUpper
		}
	} else {
		if c.Close < finalLower {
			value = finalUpper
		} else {
			value = finalLower
		}
	}

	s.finalUpper, s.finalLower, s.value = finalUpper, finalLower, value
	return value
}

func (s *SuperTrend) Preview(c *model.Candle) float64 {
	next := *s
	return next.Update(c)
}

func (s *SuperTrend) Value() float64 { return s.value }
func (s *SuperTrend) Ready() bool    { return s.atr.Ready() }
func (s *SuperTrend) Warmup() int    { return s.atr.Warmup() }

func (s *SuperTrend) Reset() {
	*s = SuperTrend{atr: *NewATR(s.atr.period), factor: s.factor}
}
