package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMonteCarlo(t *testing.T) {
	t.Run("is reproducible", func(t *testing.T) {
		pnl := []float64{120, -80, 45, -30, 210, -95}
		first := MonteCarlo(pnl, 10000, 500, 0.2, DefaultSeed)
		second := MonteCarlo(pnl, 10000, 500, 0.2, DefaultSeed)
		assert.Equal(t, first, second, "the same seed must give the same report")
	})

	t.Run("always profits when no trade loses", func(t *testing.T) {
		result := MonteCarlo([]float64{10, 20, 30}, 1000, 200, 0.1, DefaultSeed)
		assert.Equal(t, 1.0, result.ProfitProbability)
		assert.Zero(t, result.RuinProbability)
		assert.Zero(t, result.MaxDrawdown.P95, "a curve that only goes up has no drawdown")
		assert.Positive(t, result.FinalReturn.P5)
	})

	t.Run("never profits when every trade loses", func(t *testing.T) {
		result := MonteCarlo([]float64{-10, -20, -30}, 1000, 200, 0.02, DefaultSeed)
		assert.Zero(t, result.ProfitProbability)
		assert.Equal(t, 1.0, result.RuinProbability)
		assert.Negative(t, result.FinalReturn.P95)
	})

	t.Run("orders the percentiles", func(t *testing.T) {
		result := MonteCarlo([]float64{120, -80, 45, -30, 210, -95}, 10000, 1000, 0.2, DefaultSeed)
		d := result.FinalReturn
		assert.LessOrEqual(t, d.Min, d.P5)
		assert.LessOrEqual(t, d.P5, d.P25)
		assert.LessOrEqual(t, d.P25, d.Median)
		assert.LessOrEqual(t, d.Median, d.P75)
		assert.LessOrEqual(t, d.P75, d.P95)
		assert.LessOrEqual(t, d.P95, d.Max)
		assert.Equal(t, 1000, result.Iterations)
		assert.Equal(t, 0.2, result.RuinThreshold)
	})

	t.Run("has nothing to simulate without trades or equity", func(t *testing.T) {
		assert.Zero(t, MonteCarlo(nil, 1000, 100, 0.2, DefaultSeed).FinalReturn.Mean)
		assert.Zero(t, MonteCarlo([]float64{10}, 0, 100, 0.2, DefaultSeed).ProfitProbability)
		assert.Zero(t, MonteCarlo([]float64{10}, 1000, 0, 0.2, DefaultSeed).Iterations)
	})
}

func TestBootstrapPValue(t *testing.T) {
	t.Run("rejects the null for a consistent edge", func(t *testing.T) {
		returns := make([]float64, 0, 60)
		for i := 0; i < 30; i++ {
			returns = append(returns, 0.02, 0.01)
		}
		assert.Less(t, BootstrapPValue(returns, 2000, DefaultSeed), 0.05)
	})

	t.Run("keeps the null for a coin flip", func(t *testing.T) {
		returns := []float64{0.01, -0.009, 0.02, -0.021, 0.005, -0.004, 0.011, -0.012}
		assert.Greater(t, BootstrapPValue(returns, 2000, DefaultSeed), 0.05)
	})

	t.Run("is inconclusive for a losing sample", func(t *testing.T) {
		assert.Equal(t, 1.0, BootstrapPValue([]float64{-0.01, -0.02, 0.005}, 500, DefaultSeed))
		assert.Equal(t, 1.0, BootstrapPValue([]float64{0.01}, 500, DefaultSeed))
	})

	t.Run("never returns an impossible zero", func(t *testing.T) {
		returns := make([]float64, 200)
		for i := range returns {
			returns[i] = 0.05
		}
		assert.Positive(t, BootstrapPValue(returns, 100, DefaultSeed))
	})
}

func TestWindows(t *testing.T) {
	returns := []float64{0.01, 0.02, -0.01, 0.03, -0.02, 0.04}
	pnl := []float64{10, 20, -10, 30, -20, 40}

	windows := Windows(returns, pnl, 3)
	require.Len(t, windows, 3)

	assert.Equal(t, 0, windows[0].Index)
	assert.Equal(t, 2, windows[0].Trades)
	assert.InDelta(t, 30.0, windows[0].NetProfit, 1e-9)
	assert.InDelta(t, 1.0, windows[0].WinRate, 1e-9)
	assert.InDelta(t, 0.015, windows[0].MeanReturn, 1e-9)
	assert.InDelta(t, 0.5, windows[1].WinRate, 1e-9)

	t.Run("keeps the remainder in the last window", func(t *testing.T) {
		windows := Windows([]float64{1, 2, 3, 4, 5}, nil, 2)
		require.Len(t, windows, 2)
		assert.Equal(t, 2, windows[0].Trades)
		assert.Equal(t, 3, windows[1].Trades)
		assert.Zero(t, windows[0].NetProfit, "without a matching P&L there is nothing to sum")
	})

	t.Run("refuses to split what it cannot", func(t *testing.T) {
		assert.Nil(t, Windows(nil, nil, 4))
		assert.Nil(t, Windows([]float64{1, 2}, nil, 4))
		assert.Nil(t, Windows([]float64{1, 2}, nil, 0))
	})
}

func TestStabilityTest(t *testing.T) {
	t.Run("flags an edge that decayed", func(t *testing.T) {
		returns := make([]float64, 0, 40)
		for i := 0; i < 20; i++ {
			returns = append(returns, 0.03)
		}
		for i := 0; i < 20; i++ {
			returns = append(returns, -0.03)
		}

		test := StabilityTest(returns, 2000, DefaultSeed)
		assert.Negative(t, test.Statistic, "the second half is worse")
		assert.True(t, test.Significant(0.05))
	})

	t.Run("accepts a stable strategy", func(t *testing.T) {
		returns := make([]float64, 0, 40)
		for i := 0; i < 20; i++ {
			returns = append(returns, 0.02, -0.01)
		}

		test := StabilityTest(returns, 2000, DefaultSeed)
		assert.False(t, test.Significant(0.05))
	})

	t.Run("is reproducible and inconclusive without a sample", func(t *testing.T) {
		returns := []float64{0.01, -0.02, 0.03, -0.01, 0.02, 0.01}
		assert.Equal(t, StabilityTest(returns, 500, DefaultSeed), StabilityTest(returns, 500, DefaultSeed))
		assert.Equal(t, 1.0, StabilityTest([]float64{0.01, 0.02}, 500, DefaultSeed).PValue)
	})
}
