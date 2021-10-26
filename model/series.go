package model

import (
	"strconv"
	"strings"
)

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

func (s Series) Cross(ref Series) bool {
	return s.Crossover(ref) || s.Crossunder(ref)
}

func NumDecPlaces(v float64) int64 {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	i := strings.IndexByte(s, '.')
	if i > -1 {
		return int64(len(s) - i - 1)
	}
	return 0
}
