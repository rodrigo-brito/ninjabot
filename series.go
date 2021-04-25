package ninjabot

func Last(series []float64, index int) float64 {
	return series[len(series)-index-1]
}
