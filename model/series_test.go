package model

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSeries_Last(t *testing.T) {
	series := Series([]float64{1, 2, 3, 4, 5})
	require.Equal(t, 5.0, series.Last(0))
	require.Equal(t, 3.0, series.Last(2))
}

func TestSeries_LastValues(t *testing.T) {
	t.Run("with value", func(t *testing.T) {
		series := Series([]float64{1, 2, 3, 4, 5})
		require.Equal(t, []float64{4, 5}, series.LastValues(2))
	})

	t.Run("empty", func(t *testing.T) {
		series := Series([]float64{})
		require.Empty(t, series.LastValues(2))
	})
}

func TestNumDecPlaces(t *testing.T) {
	tt := []struct {
		Value  float64
		Expect int64
	}{
		{0.1, 1},
		{0.10001, 5},
		{1000, 0},
		{-1000, 0},
		{-1.1, 1},
	}

	for _, tc := range tt {
		t.Run(fmt.Sprintf("given %f", tc.Value), func(t *testing.T) {
			require.Equal(t, tc.Expect, NumDecPlaces(tc.Value))
		})
	}
}
