package metrics

import (
	"math"

	"gonum.org/v1/gonum/stat"
)

func Mean(values []float64) float64 {
	return stat.Mean(values, nil)
}

func Payoff(values []float64) float64 {
	wins := []float64{}
	loses := []float64{}
	for _, value := range values {
		if value >= 0 {
			wins = append(wins, value)
		} else {
			loses = append(loses, value)
		}
	}

	return math.Abs(stat.Mean(wins, nil) / stat.Mean(loses, nil))
}

func ProfitFactor(values []float64) float64 {
	var (
		wins  float64
		loses float64
	)

	for _, value := range values {
		if value >= 0 {
			wins += value
		} else {
			loses += value
		}
	}

	if loses == 0 {
		return 10
	}

	return math.Abs(wins / loses)
}
