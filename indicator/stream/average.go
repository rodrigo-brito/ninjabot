package stream

// SMA is the simple moving average of the last period values.
type SMA struct {
	period int
	window ring
	sum    float64 // running total of the last period-1 values
	value  float64
}

// NewSMA creates a simple moving average. Period must be positive.
func NewSMA(period int) *SMA {
	if period < 1 {
		panic("stream: SMA period must be positive")
	}
	return &SMA{period: period, window: newRing(period)}
}

func (s *SMA) Update(v float64) float64 {
	s.window.push(v)
	s.sum += v
	if s.window.count < s.period {
		return 0
	}
	// like talib: add the new value, average, then drop the trailing one
	s.value = s.sum / float64(s.period)
	s.sum -= s.window.at(s.window.count - s.period)
	return s.value
}

func (s *SMA) Preview(v float64) float64 {
	if s.window.count+1 < s.period {
		return 0
	}
	return (s.sum + v) / float64(s.period)
}

func (s *SMA) Value() float64 { return s.value }
func (s *SMA) Ready() bool    { return s.window.count >= s.period }
func (s *SMA) Warmup() int    { return s.period - 1 }

func (s *SMA) Reset() {
	s.window.reset()
	s.sum, s.value = 0, 0
}

// EMA is the exponential moving average, seeded with the simple average of
// the first period values.
type EMA struct {
	period int
	k      float64
	count  int
	sum    float64
	value  float64
}

// NewEMA creates an exponential moving average with the usual smoothing
// factor 2/(period+1). Period must be positive.
func NewEMA(period int) *EMA {
	return newEMA(period, 2.0/float64(period+1))
}

func newEMA(period int, k float64) *EMA {
	if period < 1 {
		panic("stream: EMA period must be positive")
	}
	return &EMA{period: period, k: k}
}

func (e *EMA) Update(v float64) float64 {
	e.count++
	switch {
	case e.count < e.period:
		e.sum += v
		return 0
	case e.count == e.period:
		e.sum += v
		e.value = e.sum / float64(e.period)
	default:
		e.value = ((v - e.value) * e.k) + e.value
	}
	return e.value
}

func (e *EMA) Preview(v float64) float64 {
	next := *e
	return next.Update(v)
}

func (e *EMA) Value() float64 { return e.value }
func (e *EMA) Ready() bool    { return e.count >= e.period }
func (e *EMA) Warmup() int    { return e.period - 1 }

func (e *EMA) Reset() {
	e.count, e.sum, e.value = 0, 0, 0
}
