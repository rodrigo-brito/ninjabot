package series

type Series []float64

func (s Series) Values() []float64 {
	return s
}

func (s Series) Last(position int) float64 {
	return s[len(s)-1-position]
}

func (s Series) LastValues(size int) []float64 {
	if l := len(s); l > size {
		return s[l-size:]
	}
	return s
}

func (s Series) Crossover(ref Series) bool {
	return s.Last(0) > ref.Last(0) && s.Last(1) <= ref.Last(1)
}

func (s Series) Crossunder(ref Series) bool {
	return s.Last(0) <= ref.Last(0) && s.Last(1) > ref.Last(1)
}
