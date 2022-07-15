package tools

type TrailingStop struct {
	current float64
	stop    float64
	active  bool
}

func NewTrailingStop() *TrailingStop {
	return &TrailingStop{}
}

func (t *TrailingStop) Start(current, stop float64) {
	t.stop = stop
	t.current = current
	t.active = true
}

func (t *TrailingStop) Stop() {
	t.active = false
}

func (t TrailingStop) Active() bool {
	return t.active
}

func (t *TrailingStop) Update(current float64) bool {
	if !t.active {
		return false
	}

	if current > t.current {
		t.stop = t.stop + (current - t.current)
		t.current = current
		return false
	}

	t.current = current
	return current <= t.stop
}
