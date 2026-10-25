package stream

import (
	"math"

	"github.com/rodrigo-brito/ninjabot/model"
)

// RSI is the relative strength index with Wilder smoothing.
type RSI struct {
	period     int
	count      int
	previous   float64
	gain, loss float64
	value      float64
}

// NewRSI creates a relative strength index. Like talib, a period below 2
// always yields zero.
func NewRSI(period int) *RSI { return &RSI{period: period} }

func (r *RSI) Update(v float64) float64 {
	r.count++
	if r.period < 2 {
		return 0
	}
	if r.count == 1 {
		r.previous = v
		return 0
	}

	diff := v - r.previous
	r.previous = v
	n := float64(r.period)

	if r.count <= r.period+1 {
		if diff < 0 {
			r.loss -= diff
		} else {
			r.gain += diff
		}
		if r.count < r.period+1 {
			return 0
		}
		r.loss /= n
		r.gain /= n
	} else {
		r.loss *= n - 1
		r.gain *= n - 1
		if diff < 0 {
			r.loss -= diff
		} else {
			r.gain += diff
		}
		r.loss /= n
		r.gain /= n
	}

	total := r.gain + r.loss
	if -0.00000000000001 < total && total < 0.00000000000001 {
		r.value = 0
	} else {
		r.value = 100.0 * (r.gain / total)
	}
	return r.value
}

func (r *RSI) Preview(v float64) float64 {
	next := *r
	return next.Update(v)
}

func (r *RSI) Value() float64 { return r.value }
func (r *RSI) Ready() bool    { return r.period >= 2 && r.count > r.period }
func (r *RSI) Warmup() int    { return r.period }

func (r *RSI) Reset() {
	*r = RSI{period: r.period}
}

// MACD is the moving average convergence divergence. Its outputs are the
// MACD line, the signal line and the histogram.
type MACD struct {
	fast, slow, signal EMA
	lookback           int
	count              int
	values             [3]float64
}

// NewMACD creates a MACD. Like talib, the fast and slow periods are swapped
// when given in the wrong order.
func NewMACD(fastPeriod, slowPeriod, signalPeriod int) *MACD {
	if slowPeriod < fastPeriod {
		fastPeriod, slowPeriod = slowPeriod, fastPeriod
	}
	return &MACD{
		fast:     *newEMA(fastPeriod, 2.0/float64(fastPeriod+1)),
		slow:     *newEMA(slowPeriod, 2.0/float64(slowPeriod+1)),
		signal:   *newEMA(signalPeriod, 2.0/float64(signalPeriod+1)),
		lookback: (signalPeriod - 1) + (slowPeriod - 1),
	}
}

func (m *MACD) Outputs() []string { return []string{"macd", "signal", "hist"} }

// UpdateValue adds the close of a new bar.
func (m *MACD) UpdateValue(v float64) []float64 {
	index := m.count
	m.count++

	line := m.fast.Update(v) - m.slow.Update(v)
	macd := 0.0
	if index >= m.lookback-1 {
		macd = line
	}
	// talib seeds the signal with the zeros that precede the MACD line
	signal := m.signal.Update(macd)
	hist := 0.0
	if index >= m.lookback {
		hist = macd - signal
	}

	m.values = [3]float64{macd, signal, hist}
	return m.values[:]
}

func (m *MACD) Update(c *model.Candle) []float64 { return m.UpdateValue(c.Close) }

func (m *MACD) Preview(c *model.Candle) []float64 {
	next := *m
	return next.UpdateValue(c.Close)
}

func (m *MACD) Values() []float64 {
	values := m.values
	return values[:]
}
func (m *MACD) Ready() bool { return m.count > m.lookback }
func (m *MACD) Warmup() int { return m.lookback }

func (m *MACD) Reset() {
	m.fast.Reset()
	m.slow.Reset()
	m.signal.Reset()
	m.count = 0
	m.values = [3]float64{}
}

// Stoch is the slow stochastic oscillator with simple moving averages. Its
// outputs are %K and %D.
type Stoch struct {
	fastK        int
	highs, lows  ring
	slowK, slowD SMA
	lookback     int
	count        int
	values       [2]float64
}

// NewStoch creates a slow stochastic: the fast %K over fastK bars, smoothed
// by a simple average of slowK bars, and %D, its simple average of slowD bars.
func NewStoch(fastK, slowK, slowD int) *Stoch {
	return &Stoch{
		fastK:    fastK,
		highs:    newRing(fastK),
		lows:     newRing(fastK),
		slowK:    *NewSMA(slowK),
		slowD:    *NewSMA(slowD),
		lookback: (fastK - 1) + (slowK - 1) + (slowD - 1),
	}
}

func (s *Stoch) Outputs() []string { return []string{"k", "d"} }

// fastValue is the fast %K of c as the next bar, from the previous fastK-1
// bars and c itself.
func (s *Stoch) fastValue(c *model.Candle) (float64, bool) {
	index := s.count
	if index < s.fastK-1 {
		return 0, false
	}

	highest, lowest := c.High, c.Low
	from := index - s.fastK + 1
	for i := from; i < index; i++ {
		if high := s.highs.at(i); high > highest {
			highest = high
		}
		if low := s.lows.at(i); low < lowest {
			lowest = low
		}
	}

	diff := (highest - lowest) / 100.0
	if diff == 0 {
		return 0, true
	}
	return (c.Close - lowest) / diff, true
}

func (s *Stoch) Update(c *model.Candle) []float64 {
	fast, ok := s.fastValue(c)
	s.highs.push(c.High)
	s.lows.push(c.Low)
	index := s.count
	s.count++

	if !ok {
		return s.values[:]
	}
	k := s.slowK.Update(fast)
	d := s.slowD.Update(k)
	if index >= s.lookback {
		s.values = [2]float64{k, d}
	}
	return s.values[:]
}

func (s *Stoch) Preview(c *model.Candle) []float64 {
	index := s.count
	fast, ok := s.fastValue(c)
	if !ok || index < s.lookback {
		values := s.values
		return values[:]
	}
	k := s.slowK.Preview(fast)
	return []float64{k, s.slowD.Preview(k)}
}

func (s *Stoch) Values() []float64 {
	values := s.values
	return values[:]
}
func (s *Stoch) Ready() bool { return s.count > s.lookback }
func (s *Stoch) Warmup() int { return s.lookback }

func (s *Stoch) Reset() {
	s.highs.reset()
	s.lows.reset()
	s.slowK.Reset()
	s.slowD.Reset()
	s.count = 0
	s.values = [2]float64{}
}

// CCI is the commodity channel index over the typical price.
type CCI struct {
	period int
	window ring
	value  float64
}

// NewCCI creates a commodity channel index. Period must be positive.
func NewCCI(period int) *CCI {
	if period < 1 {
		panic("stream: CCI period must be positive")
	}
	return &CCI{period: period, window: newRing(period)}
}

func (c *CCI) compute(values []float64, last float64) float64 {
	// like talib, sum the circular buffer in storage order
	average := 0.0
	for _, v := range values {
		average += v
	}
	average /= float64(c.period)
	deviation := 0.0
	for _, v := range values {
		deviation += math.Abs(v - average)
	}
	diff := last - average
	if diff != 0.0 && deviation != 0.0 {
		return diff / (0.015 * (deviation / float64(c.period)))
	}
	return 0
}

func (c *CCI) Update(candle *model.Candle) float64 {
	typical := (candle.High + candle.Low + candle.Close) / 3
	c.window.push(typical)
	if c.window.count < c.period {
		return 0
	}
	c.value = c.compute(c.window.values, typical)
	return c.value
}

func (c *CCI) Preview(candle *model.Candle) float64 {
	if c.window.count+1 < c.period {
		return 0
	}
	typical := (candle.High + candle.Low + candle.Close) / 3
	values := make([]float64, len(c.window.values))
	copy(values, c.window.values)
	values[c.window.count%c.period] = typical
	return c.compute(values, typical)
}

func (c *CCI) Value() float64 { return c.value }
func (c *CCI) Ready() bool    { return c.window.count >= c.period }
func (c *CCI) Warmup() int    { return c.period - 1 }

func (c *CCI) Reset() {
	c.window.reset()
	c.value = 0
}

// WillR is the Williams %R: where the close sits in the range of the last
// period bars, from 0 (highest) to -100 (lowest).
type WillR struct {
	period      int
	highs, lows ring
	value       float64
}

// NewWillR creates a Williams %R. Period must be positive.
func NewWillR(period int) *WillR {
	if period < 1 {
		panic("stream: WillR period must be positive")
	}
	return &WillR{period: period, highs: newRing(period), lows: newRing(period)}
}

func (w *WillR) compute(candle *model.Candle, index int) float64 {
	highest, lowest := candle.High, candle.Low
	for i := index - w.period + 1; i < index; i++ {
		highest = math.Max(highest, w.highs.at(i))
		lowest = math.Min(lowest, w.lows.at(i))
	}
	diff := (highest - lowest) / (-100.0)
	if diff == 0 {
		return 0
	}
	return (highest - candle.Close) / diff
}

func (w *WillR) Update(candle *model.Candle) float64 {
	index := w.highs.count
	value := 0.0
	if index >= w.period-1 {
		value = w.compute(candle, index)
		w.value = value
	}
	w.highs.push(candle.High)
	w.lows.push(candle.Low)
	return value
}

func (w *WillR) Preview(candle *model.Candle) float64 {
	index := w.highs.count
	if index < w.period-1 {
		return 0
	}
	return w.compute(candle, index)
}

func (w *WillR) Value() float64 { return w.value }
func (w *WillR) Ready() bool    { return w.highs.count >= w.period }
func (w *WillR) Warmup() int    { return w.period - 1 }

func (w *WillR) Reset() {
	w.highs.reset()
	w.lows.reset()
	w.value = 0
}
