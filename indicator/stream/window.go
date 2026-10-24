package stream

// Max is the highest value of the last period values.
type Max struct{ extreme }

// Min is the lowest value of the last period values.
type Min struct{ extreme }

// NewMax creates a rolling maximum. Like talib, a period below 2 always
// yields zero.
func NewMax(period int) *Max { return &Max{newExtreme(period, true)} }

// NewMin creates a rolling minimum. Like talib, a period below 2 always
// yields zero.
func NewMin(period int) *Min { return &Min{newExtreme(period, false)} }

type extreme struct {
	period  int
	highest bool
	window  ring
	index   int // absolute position of the current extreme
	value   float64
}

func newExtreme(period int, highest bool) extreme {
	size := period
	if size < 1 {
		size = 1
	}
	return extreme{period: period, highest: highest, window: newRing(size), index: -1}
}

func (e *extreme) better(a, b float64) bool {
	if e.highest {
		return a >= b
	}
	return a <= b
}

func (e *extreme) Update(v float64) float64 {
	if e.period < 2 {
		e.window.push(v)
		return 0
	}

	e.window.push(v)
	today := e.window.count - 1
	if today < e.period-1 {
		return 0
	}

	trailing := today - e.period + 1
	if e.index < trailing {
		// the extreme left the window: scan it again
		e.index = trailing
		e.value = e.window.at(trailing)
		for i := trailing + 1; i <= today; i++ {
			if value := e.window.at(i); e.better(value, e.value) && value != e.value {
				e.index, e.value = i, value
			}
		}
	} else if e.better(v, e.value) {
		e.index, e.value = today, v
	}
	return e.value
}

func (e *extreme) Preview(v float64) float64 {
	if e.period < 2 || e.window.count+1 < e.period {
		return 0
	}
	value := v
	for i := e.window.count - e.period + 1; i < e.window.count; i++ {
		if candidate := e.window.at(i); e.better(candidate, value) {
			value = candidate
		}
	}
	return value
}

func (e *extreme) Value() float64 { return e.value }
func (e *extreme) Ready() bool    { return e.period >= 2 && e.window.count >= e.period }
func (e *extreme) Warmup() int    { return e.period - 1 }

func (e *extreme) Reset() {
	e.window.reset()
	e.index, e.value = -1, 0
}
