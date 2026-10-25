package sweep

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/backtest"
	"github.com/rodrigo-brito/ninjabot/strategy"
)

func TestRun_Overfitting(t *testing.T) {
	summary, err := Run(context.Background(), loadSpec(t), exampleDir, strategy.Default, Options{Out: t.TempDir()})
	require.NoError(t, err)
	require.NotNil(t, summary.Overfitting)
	require.Equal(t, 12870, summary.Overfitting.Splits)
	require.True(t, summary.Overfitting.PBO >= 0 && summary.Overfitting.PBO <= 1)
	require.Nil(t, summary.Overfitting.Logits, "the summary keeps only the estimate")
	require.Len(t, summary.Ranking[0].Blocks, blockCount)
	require.Nil(t, overfitting(summary.Ranking[:1]))
}

func TestRunWalkForward(t *testing.T) {
	spec := loadSpec(t)
	result, err := RunWalkForward(context.Background(), spec, exampleDir, strategy.Default,
		WalkForwardOptions{Train: "60d", Test: "30d"})
	require.NoError(t, err)
	require.Equal(t, "sharpe_annual", result.Metric)
	require.Len(t, result.Folds, 4)

	for i, fold := range result.Folds {
		require.Empty(t, fold.Error)
		require.Equal(t, fold.TrainEnd, fold.TestStart)
		require.Equal(t, 60*24*time.Hour, fold.TrainEnd.Sub(fold.TrainStart))
		require.Equal(t, 30*24*time.Hour, fold.TestEnd.Sub(fold.TestStart))
		if i > 0 {
			require.Equal(t, result.Folds[i-1].TestStart, fold.TestStart.Add(-30*24*time.Hour), "rolls by the test length")
		}
		require.NotEmpty(t, fold.Params)
	}

	compounded := 1.0
	for _, fold := range result.Folds {
		compounded *= 1 + fold.OutSample.ReturnPct
	}
	require.InDelta(t, compounded-1, result.OutOfSampleReturn, 1e-9)
	require.NotZero(t, result.Efficiency)

	again, err := RunWalkForward(context.Background(), spec, exampleDir, strategy.Default,
		WalkForwardOptions{Train: "60d", Test: "30d", Parallel: 1})
	require.NoError(t, err)
	require.Equal(t, result, again, "deterministic")

	split, err := RunWalkForward(context.Background(), spec, exampleDir, strategy.Default,
		WalkForwardOptions{Train: "120d", Test: "50d", Metric: "return_pct"})
	require.NoError(t, err)
	require.Len(t, split.Folds, 1, "a train/test split")

	for _, options := range []WalkForwardOptions{
		{Train: "", Test: "30d"},
		{Train: "60d", Test: "x"},
		{Train: "60d", Test: "30d", Step: "-1d"},
		{Train: "60d", Test: "30d", Metric: "luck"},
	} {
		_, err := RunWalkForward(context.Background(), spec, exampleDir, strategy.Default, options)
		require.ErrorIs(t, err, backtest.ErrInvalidSpec)
	}
	_, err = RunWalkForward(context.Background(), spec, exampleDir, strategy.Default,
		WalkForwardOptions{Train: "300d", Test: "30d"})
	require.ErrorIs(t, err, backtest.ErrData)
}

func TestSensitivity(t *testing.T) {
	spec := loadSpec(t).Spec
	points, err := Sensitivity(context.Background(), spec, exampleDir, strategy.Default, []float64{0, 0.001, 0.005})
	require.NoError(t, err)
	require.Len(t, points, 3)
	require.Zero(t, points[0].Metrics.Fees)
	require.Greater(t, points[0].Metrics.NetProfit, points[1].Metrics.NetProfit)
	require.Greater(t, points[1].Metrics.NetProfit, points[2].Metrics.NetProfit)

	_, err = Sensitivity(context.Background(), spec, exampleDir, strategy.Default, nil)
	require.ErrorIs(t, err, backtest.ErrInvalidSpec)
	_, err = Sensitivity(context.Background(), spec, exampleDir, strategy.Default, []float64{1.5})
	require.ErrorIs(t, err, backtest.ErrInvalidSpec)
}
