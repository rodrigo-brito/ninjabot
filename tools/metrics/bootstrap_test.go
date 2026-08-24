package metrics

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gonum.org/v1/gonum/stat"
)

func TestBootstrap(t *testing.T) {
	values := []float64{7, 9, 10, 10, 12, 14, 15, 16, 16, 17, 19, 20, 21, 21, 23}
	mean := func(samples []float64) float64 { return stat.Mean(samples, nil) }

	t.Run("estimates the confidence interval of the mean", func(t *testing.T) {
		result := Bootstrap(values, mean, 10000, 0.95)

		// The resampling is random, so the tolerance has to accommodate the
		// spread of the bootstrap distribution (stddev ~1.24).
		require.InDelta(t, 15.34, result.Mean, 0.2)
		require.InDelta(t, 1.24, result.StdDev, 0.2)
		require.InDelta(t, 12.9, result.Lower, 0.4)
		require.InDelta(t, 17.7, result.Upper, 0.4)
		require.Less(t, result.Lower, result.Mean)
		require.Greater(t, result.Upper, result.Mean)
	})

	t.Run("widens the interval for higher confidence levels", func(t *testing.T) {
		narrow := Bootstrap(values, mean, 5000, 0.80)
		wide := Bootstrap(values, mean, 5000, 0.99)

		require.Less(t, wide.Lower, narrow.Lower)
		require.Greater(t, wide.Upper, narrow.Upper)
	})

	t.Run("collapses to a single value for constant samples", func(t *testing.T) {
		result := Bootstrap([]float64{5, 5, 5}, mean, 100, 0.95)

		require.Equal(t, 5.0, result.Mean)
		require.Equal(t, 0.0, result.StdDev)
		require.Equal(t, 5.0, result.Lower)
		require.Equal(t, 5.0, result.Upper)
	})
}
