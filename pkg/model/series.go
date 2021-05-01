package model

import (
	"strconv"
	"strings"
)

func Last(series []float64, index int) float64 {
	return series[len(series)-index-1]
}

func NumDecPlaces(v float64) int64 {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	i := strings.IndexByte(s, '.')
	if i > -1 {
		return int64(len(s) - i - 1)
	}
	return 0
}
