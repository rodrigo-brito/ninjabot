package metrics

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gonum.org/v1/gonum/stat"
)

func TestBootstrap(t *testing.T) {
	values := []float64{7, 9, 10, 10, 12, 14, 15, 16, 16, 17, 19, 20, 21, 21, 23}
	result := Bootstrap(values, func(samples []float64) float64 {
		return stat.Mean(samples, nil)
	}, 10000, 0.95)

	require.InDelta(t, 15.34, result.Mean, 0.1)
	require.InDelta(t, 1.24, result.StdDev, 0.1)
	require.InDelta(t, 12.9, result.Lower, 0.1)
	require.InDelta(t, 17.7, result.Upper, 0.1)
}
