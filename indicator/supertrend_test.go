package indicator

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSuperTrend(t *testing.T) {
	t.Run("returns one value per bar", func(t *testing.T) {
		result := SuperTrend(sampleHigh, sampleLow, sampleClose, 10, 3)

		require.Len(t, result, len(sampleClose))
		require.Zero(t, result[0], "the first bar has no previous band to compare with")
	})

	t.Run("tracks below the price on an uptrend", func(t *testing.T) {
		size := 60
		high := make([]float64, size)
		low := make([]float64, size)
		close := make([]float64, size)
		for i := 0; i < size; i++ {
			close[i] = 100 + float64(i)
			high[i] = close[i] + 1
			low[i] = close[i] - 1
		}

		result := SuperTrend(high, low, close, 10, 3)

		last := result[size-1]
		require.Less(t, last, close[size-1], "on a steady uptrend the band sits below the close")
		require.Greater(t, last, 0.0)
	})

	t.Run("tracks above the price on a downtrend", func(t *testing.T) {
		size := 60
		high := make([]float64, size)
		low := make([]float64, size)
		close := make([]float64, size)
		for i := 0; i < size; i++ {
			close[i] = 200 - float64(i)
			high[i] = close[i] + 1
			low[i] = close[i] - 1
		}

		result := SuperTrend(high, low, close, 10, 3)

		require.Greater(t, result[size-1], close[size-1], "on a steady downtrend the band sits above the close")
	})

	t.Run("flips sides when the trend reverses", func(t *testing.T) {
		size := 80
		high := make([]float64, size)
		low := make([]float64, size)
		close := make([]float64, size)
		for i := 0; i < size; i++ {
			if i < size/2 {
				close[i] = 100 + float64(i)*2
			} else {
				close[i] = 100 + float64(size-i)*2
			}
			high[i] = close[i] + 2
			low[i] = close[i] - 2
		}

		result := SuperTrend(high, low, close, 5, 2)

		below := result[size/2-1] < close[size/2-1]
		above := result[size-1] > close[size-1]
		require.True(t, below, "band below the close before the reversal")
		require.True(t, above, "band above the close after the reversal")
	})

	t.Run("widens the band with a larger factor", func(t *testing.T) {
		narrow := SuperTrend(sampleHigh, sampleLow, sampleClose, 10, 1)
		wide := SuperTrend(sampleHigh, sampleLow, sampleClose, 10, 5)

		last := len(sampleClose) - 1
		narrowDistance := math.Abs(narrow[last] - sampleClose[last])
		wideDistance := math.Abs(wide[last] - sampleClose[last])
		require.Greater(t, wideDistance, narrowDistance)
	})
}
