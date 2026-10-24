package metrics

import (
	"math/rand"
	"sort"

	"gonum.org/v1/gonum/stat"
)

type BootstrapInterval struct {
	Lower  float64
	Upper  float64
	StdDev float64
	Mean   float64
}

// Bootstrap calculates the confidence interval of a sample using the bootstrap
// method. It resamples from DefaultSeed, so the same sample always reports the
// same interval -- a report that moves on every reload cannot be acted on.
func Bootstrap(values []float64, measure func([]float64) float64, sampleSize int,
	confidence float64) BootstrapInterval {

	return BootstrapWithSeed(values, measure, sampleSize, confidence, DefaultSeed)
}

// BootstrapWithSeed is Bootstrap with an explicit seed, for callers that need
// several independent resamplings of the same data.
func BootstrapWithSeed(values []float64, measure func([]float64) float64, sampleSize int,
	confidence float64, seed int64) BootstrapInterval {

	if len(values) == 0 || sampleSize <= 0 {
		return BootstrapInterval{}
	}

	random := rand.New(rand.NewSource(seed)) //nolint:gosec // reproducibility, not cryptography

	data := make([]float64, 0, sampleSize)
	samples := make([]float64, len(values))

	for i := 0; i < sampleSize; i++ {
		for j := range samples {
			samples[j] = values[random.Intn(len(values))]
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
