package metrics

import (
	"sort"

	"github.com/samber/lo"
	"gonum.org/v1/gonum/stat"
)

type BootstrapInterval struct {
	Lower  float64
	Upper  float64
	StdDev float64
	Mean   float64
}

// Bootstrap calculates the confidence interval of a sample using the bootstrap method.
func Bootstrap(values []float64, measure func([]float64) float64, sampleSize int,
	confidence float64) BootstrapInterval {

	var data []float64
	for i := 0; i < sampleSize; i++ {
		samples := make([]float64, len(values))
		for j := 0; j < len(values); j++ {
			samples[j] = lo.Sample(values)
		}
		data = append(data, measure(samples))
	}

	tail := 1 - confidence

	sort.Float64s(data)
	mean, stdDev := stat.MeanStdDev(data, nil)
	upper := stat.Quantile(1-tail/2, stat.LinInterp, data, nil)
	lower := stat.Quantile(tail/2, stat.LinInterp, data, nil)

	return BootstrapInterval{
		Lower:  lower,
		Upper:  upper,
		StdDev: stdDev,
		Mean:   mean,
	}
}
