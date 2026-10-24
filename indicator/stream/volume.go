package stream

import "github.com/rodrigo-brito/ninjabot/model"

// OBV is the on-balance volume: the running total of the volume, added on
// rising closes and subtracted on falling ones, starting at the first volume.
type OBV struct {
	count         int
	previousClose float64
	value         float64
}

// NewOBV creates an on-balance volume.
func NewOBV() *OBV { return &OBV{} }

func (o *OBV) Update(c *model.Candle) float64 {
	o.count++
	if o.count == 1 {
		o.previousClose, o.value = c.Close, c.Volume
		return o.value
	}
	if c.Close > o.previousClose {
		o.value += c.Volume
	} else if c.Close < o.previousClose {
		o.value -= c.Volume
	}
	o.previousClose = c.Close
	return o.value
}

func (o *OBV) Preview(c *model.Candle) float64 {
	next := *o
	return next.Update(c)
}

func (o *OBV) Value() float64 { return o.value }
func (o *OBV) Ready() bool    { return o.count > 0 }
func (o *OBV) Warmup() int    { return 0 }
func (o *OBV) Reset()         { *o = OBV{} }
