package metrics

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStdDev(t *testing.T) {
	assert.InDelta(t, 2.0, StdDev([]float64{2, 4, 4, 4, 5, 5, 7, 9}), 0.2)
	assert.Zero(t, StdDev([]float64{1}))
	assert.Zero(t, StdDev(nil))
}

func TestSharpe(t *testing.T) {
	t.Run("scales the average by the volatility", func(t *testing.T) {
		calm := Sharpe([]float64{0.01, 0.012, 0.008, 0.011})
		wild := Sharpe([]float64{0.01, 0.2, -0.18, 0.01})
		assert.Greater(t, calm, wild, "the same average with more noise is worth less")
	})

	t.Run("is zero without variation", func(t *testing.T) {
		assert.Zero(t, Sharpe([]float64{0.01, 0.01, 0.01}))
		assert.Zero(t, Sharpe(nil))
	})
}

func TestSortino(t *testing.T) {
	t.Run("ignores upside volatility", func(t *testing.T) {
		returns := []float64{0.01, 0.30, 0.02, -0.01}
		assert.Greater(t, Sortino(returns), Sharpe(returns))
	})

	t.Run("is zero without losses", func(t *testing.T) {
		assert.Zero(t, Sortino([]float64{0.01, 0.02, 0.03}))
	})
}

func TestAnnualize(t *testing.T) {
	assert.InDelta(t, 2.0, Annualize(1, 4), 1e-9)
	assert.Zero(t, Annualize(1, 0))
}

func TestSkewAndKurtosis(t *testing.T) {
	assert.Negative(t, Skew([]float64{0.01, 0.01, 0.01, 0.01, -0.20}), "a big loss skews left")
	assert.Positive(t, ExcessKurtosis([]float64{0, 0, 0, 0, 0, 0, 0, 5, -5}), "outliers fatten the tails")
	assert.Zero(t, Skew([]float64{1, 2}))
	assert.Zero(t, ExcessKurtosis([]float64{1, 2}))
}

func TestProbabilisticSharpe(t *testing.T) {
	short := []float64{0.02, -0.01, 0.03, -0.005, 0.01}
	long := make([]float64, 0, 200)
	for i := 0; i < 40; i++ {
		long = append(long, short...)
	}

	t.Run("grows with the sample size", func(t *testing.T) {
		assert.Greater(t, ProbabilisticSharpe(long, 0), ProbabilisticSharpe(short, 0),
			"the same edge observed for longer is more credible")
	})

	t.Run("stays a probability", func(t *testing.T) {
		for _, value := range []float64{ProbabilisticSharpe(short, 0), ProbabilisticSharpe(long, 0)} {
			assert.GreaterOrEqual(t, value, 0.0)
			assert.LessOrEqual(t, value, 1.0)
		}
	})

	t.Run("drops below a demanding benchmark", func(t *testing.T) {
		assert.Less(t, ProbabilisticSharpe(long, 10), 0.5)
	})

	t.Run("is zero for a sample that is too short", func(t *testing.T) {
		assert.Zero(t, ProbabilisticSharpe([]float64{0.01, 0.02}, 0))
	})
}

func TestMinimumTrackRecord(t *testing.T) {
	returns := []float64{0.02, -0.01, 0.03, -0.005, 0.01, 0.002, -0.004, 0.015}

	t.Run("returns the trades needed to trust the Sharpe", func(t *testing.T) {
		assert.Greater(t, MinimumTrackRecord(returns, 0, 0.95), 1.0)
	})

	t.Run("asks for more trades at a higher confidence", func(t *testing.T) {
		assert.Greater(t, MinimumTrackRecord(returns, 0, 0.99), MinimumTrackRecord(returns, 0, 0.95))
	})

	t.Run("is zero when the Sharpe is already below the benchmark", func(t *testing.T) {
		assert.Zero(t, MinimumTrackRecord(returns, 5, 0.95))
		assert.Zero(t, MinimumTrackRecord([]float64{-0.1, -0.2, -0.3}, 0, 0.95))
	})
}

func TestMeanTTest(t *testing.T) {
	t.Run("rejects the null for a consistent edge", func(t *testing.T) {
		returns := make([]float64, 0, 60)
		for i := 0; i < 30; i++ {
			returns = append(returns, 0.02, 0.01)
		}
		test := MeanTTest(returns)
		assert.Positive(t, test.Statistic)
		assert.True(t, test.Significant(0.05))
	})

	t.Run("keeps the null for a coin flip", func(t *testing.T) {
		test := MeanTTest([]float64{0.01, -0.01, 0.02, -0.02, 0.005, -0.005})
		assert.False(t, test.Significant(0.05))
		assert.InDelta(t, 0.5, test.PValue, 0.01)
	})

	t.Run("is inconclusive without a sample", func(t *testing.T) {
		assert.Equal(t, 1.0, MeanTTest(nil).PValue)
		assert.Equal(t, 1.0, MeanTTest([]float64{0.01, 0.01}).PValue, "no variance, no test")
	})
}

func TestRunsTest(t *testing.T) {
	alternating := make([]float64, 0, 40)
	for i := 0; i < 20; i++ {
		alternating = append(alternating, 0.01, -0.01)
	}

	clustered := make([]float64, 0, 40)
	for i := 0; i < 20; i++ {
		clustered = append(clustered, 0.01)
	}
	for i := 0; i < 20; i++ {
		clustered = append(clustered, -0.01)
	}

	t.Run("flags a sequence that alternates too much", func(t *testing.T) {
		test := RunsTest(alternating)
		assert.Positive(t, test.Statistic, "more runs than chance")
		assert.True(t, test.Significant(0.05))
	})

	t.Run("flags wins and losses that cluster", func(t *testing.T) {
		test := RunsTest(clustered)
		assert.Negative(t, test.Statistic, "fewer runs than chance")
		assert.True(t, test.Significant(0.05))
	})

	t.Run("accepts a plausible sequence", func(t *testing.T) {
		test := RunsTest([]float64{0.01, 0.02, -0.01, 0.03, -0.02, -0.01, 0.01, 0.02, -0.03, 0.01})
		assert.False(t, test.Significant(0.05))
	})

	t.Run("is inconclusive with a single outcome", func(t *testing.T) {
		assert.Equal(t, 1.0, RunsTest([]float64{0.01, 0.02, 0.03}).PValue)
	})
}

func TestAutocorrelation(t *testing.T) {
	assert.InDelta(t, -1.0, Autocorrelation([]float64{1, -1, 1, -1, 1, -1}), 1e-9)
	assert.InDelta(t, 1.0, Autocorrelation([]float64{1, 2, 3, 4, 5, 6}), 1e-9)
	assert.Zero(t, Autocorrelation([]float64{1, 1}))
}

func TestValueAtRiskAndShortfall(t *testing.T) {
	returns := []float64{-0.10, -0.05, -0.02, 0, 0.01, 0.02, 0.03, 0.04, 0.05, 0.06}

	risk := ValueAtRisk(returns, 0.95)
	assert.Less(t, risk, -0.05, "the worst 5% of the trades lose more than 5%")

	shortfall := ExpectedShortfall(returns, 0.95)
	assert.LessOrEqual(t, shortfall, risk, "the tail average is at least as bad as its edge")

	assert.Zero(t, ValueAtRisk(nil, 0.95))
	assert.Zero(t, ExpectedShortfall(nil, 0.95))
}

func TestMaxDrawdown(t *testing.T) {
	assert.InDelta(t, 0.5, MaxDrawdown([]float64{100, 120, 60, 80}), 1e-9)
	assert.Zero(t, MaxDrawdown([]float64{100, 110, 120}))
	assert.Zero(t, MaxDrawdown(nil))
}

func TestBreakEvenFee(t *testing.T) {
	assert.InDelta(t, 0.005, BreakEvenFee([]float64{0.02, 0.0, 0.02, 0.0}), 1e-9)
	assert.Zero(t, BreakEvenFee([]float64{-0.01, 0.005}), "a losing strategy has no room for fees")
}

func TestBetaAndAlpha(t *testing.T) {
	benchmark := []float64{0.01, -0.02, 0.03, -0.01, 0.02}
	doubled := make([]float64, len(benchmark))
	for i, value := range benchmark {
		doubled[i] = 2 * value
	}

	assert.InDelta(t, 2.0, Beta(doubled, benchmark), 1e-9)
	assert.InDelta(t, 0.0, Alpha(doubled, benchmark), 1e-9)

	constant := []float64{0.01, 0.01, 0.01, 0.01, 0.01}
	assert.Zero(t, Beta(constant, constant), "a flat benchmark explains nothing")
	assert.Zero(t, Beta(doubled, benchmark[:2]), "mismatched samples are rejected")
}

func TestHistogram(t *testing.T) {
	t.Run("counts every value once", func(t *testing.T) {
		values := []float64{-0.02, -0.01, 0, 0.01, 0.02, 0.02}
		histogram := Histogram(values, 4)
		require.Len(t, histogram, 4)

		total := 0
		for _, bin := range histogram {
			total += bin.Count
			assert.Greater(t, bin.End, bin.Start)
		}
		assert.Equal(t, len(values), total)
	})

	t.Run("pads a sample with a single value", func(t *testing.T) {
		histogram := Histogram([]float64{0.05, 0.05}, 3)
		require.Len(t, histogram, 3)
		assert.Equal(t, 2, histogram[1].Count, "the value lands in the middle bin")
	})

	t.Run("has nothing to draw without data", func(t *testing.T) {
		assert.Nil(t, Histogram(nil, 10))
		assert.Nil(t, Histogram([]float64{1}, 0))
	})
}

func TestTestSignificant(t *testing.T) {
	assert.True(t, Test{PValue: 0.01}.Significant(0.05))
	assert.False(t, Test{PValue: math.NaN()}.Significant(0.05))
}
