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

	// Handle edge cases to avoid NaN
	if len(wins) == 0 || len(loses) == 0 {
		return 0
	}

	avgWin := stat.Mean(wins, nil)
	avgLose := stat.Mean(loses, nil)

	// Avoid division by zero or NaN
	if math.IsNaN(avgWin) || math.IsNaN(avgLose) || avgLose == 0 {
		return 0
	}

	return math.Abs(avgWin / avgLose)
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
