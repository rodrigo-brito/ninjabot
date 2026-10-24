// Package stream provides incremental (streaming) technical indicators: each
// new bar updates the indicator in constant time, instead of recomputing the
// whole series like the functions of the indicator package.
//
// Every indicator reproduces the arithmetic of go-talib, so feeding it a
// series bar by bar yields exactly the values talib computes over the whole
// series: Value() after the i-th update equals talib.X(series)[i], including
// the zeros talib emits before the indicator is ready.
//
// Indicators are not safe for concurrent use; a strategy controller owns one
// set per pair.
package stream

import "github.com/rodrigo-brito/ninjabot/model"

// Line is an indicator over one value per bar, such as the close.
type Line interface {
	// Update adds the value of a new bar and returns the indicator value.
	Update(v float64) float64
	// Preview returns the value the indicator would have if v were the next
	// bar, without changing its state. It serves partial candles.
	Preview(v float64) float64
	// Value returns the last value; zero before the indicator is ready.
	Value() float64
	// Ready reports whether Value is meaningful: more than Warmup bars seen.
	Ready() bool
	// Warmup is the number of bars consumed before the first meaningful
	// value, the lookback of talib.
	Warmup() int
	// Reset clears the state.
	Reset()
}

// Bar is an indicator over whole candles, such as the average true range.
type Bar interface {
	Update(c *model.Candle) float64
	Preview(c *model.Candle) float64
	Value() float64
	Ready() bool
	Warmup() int
	Reset()
}

// Multi is an indicator with several outputs, such as MACD.
type Multi interface {
	// Outputs names the values, in the order Update returns them.
	Outputs() []string
	Update(c *model.Candle) []float64
	Preview(c *model.Candle) []float64
	Values() []float64
	Ready() bool
	Warmup() int
	Reset()
}

// Source extracts the value a Line indicator reads from a candle.
type Source func(*model.Candle) float64

// Common sources.
var (
	Open   Source = func(c *model.Candle) float64 { return c.Open }
	High   Source = func(c *model.Candle) float64 { return c.High }
	Low    Source = func(c *model.Candle) float64 { return c.Low }
	Close  Source = func(c *model.Candle) float64 { return c.Close }
	Volume Source = func(c *model.Candle) float64 { return c.Volume }
	HL2    Source = func(c *model.Candle) float64 { return (c.High + c.Low) / 2 }
	HLC3   Source = func(c *model.Candle) float64 { return (c.High + c.Low + c.Close) / 3 }
	OHLC4  Source = func(c *model.Candle) float64 { return (c.Open + c.High + c.Low + c.Close) / 4 }
)

// FromLine turns a Line into a Bar that reads src from every candle.
func FromLine(l Line, src Source) Bar {
	return &lineBar{line: l, src: src}
}

type lineBar struct {
	line Line
	src  Source
}

func (b *lineBar) Update(c *model.Candle) float64  { return b.line.Update(b.src(c)) }
func (b *lineBar) Preview(c *model.Candle) float64 { return b.line.Preview(b.src(c)) }
func (b *lineBar) Value() float64                 { return b.line.Value() }
func (b *lineBar) Ready() bool                    { return b.line.Ready() }
func (b *lineBar) Warmup() int                    { return b.line.Warmup() }
func (b *lineBar) Reset()                         { b.line.Reset() }

// ring is a fixed-size window of the last values.
type ring struct {
	values []float64
	count  int // total values pushed
}

func newRing(size int) ring {
	return ring{values: make([]float64, size)}
}

// push stores v as the newest value.
func (r *ring) push(v float64) {
	r.values[r.count%len(r.values)] = v
	r.count++
}

// at returns the value pushed at absolute position i; it must still be in the
// window.
func (r *ring) at(i int) float64 {
	return r.values[i%len(r.values)]
}

func (r *ring) reset() {
	clear(r.values)
	r.count = 0
}
