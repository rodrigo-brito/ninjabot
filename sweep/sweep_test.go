package sweep

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/backtest"
	_ "github.com/rodrigo-brito/ninjabot/examples/strategies"
	"github.com/rodrigo-brito/ninjabot/strategy"
)

func TestMain(m *testing.M) {
	log.SetLevel(log.ErrorLevel)
	os.Exit(m.Run())
}

const exampleDir = "../examples/specs"

func loadSpec(t *testing.T) Spec {
	t.Helper()
	file, err := os.Open(filepath.Join(exampleDir, "sweep.json"))
	require.NoError(t, err)
	defer file.Close()
	spec, err := ReadSpec(file)
	require.NoError(t, err)
	return spec
}

func countLines(t *testing.T, path string) int {
	t.Helper()
	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()
	lines := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines++
	}
	return lines
}

func TestExpand_Grid(t *testing.T) {
	specs, err := loadSpec(t).Expand()
	require.NoError(t, err)
	require.Len(t, specs, 12)

	again, err := loadSpec(t).Expand()
	require.NoError(t, err)
	hashes := map[string]bool{}
	for i := range specs {
		require.Equal(t, specs[i].Hash(), again[i].Hash(), "deterministic order")
		hashes[specs[i].Hash()] = true
	}
	require.Len(t, hashes, 12)
	require.Equal(t, map[string]any{"fast": 5.0, "slow": 21.0, "timeframe": "4h"}, specs[0].Params)
}

func TestExpand_Random(t *testing.T) {
	spec := loadSpec(t)
	spec.Grid = nil
	spec.Random = &Random{Samples: 20, Seed: 7, Ranges: map[string]Range{
		"params.fast": {Min: 3, Max: 10, Integer: true},
		"params.slow": {Values: []any{21.0, 34.0}},
		"timeframe":   {Values: []any{"4h", "1d"}},
	}}
	specs, err := spec.Expand()
	require.NoError(t, err)
	require.NotEmpty(t, specs)
	require.LessOrEqual(t, len(specs), 20, "duplicated draws run once")
	for _, s := range specs {
		fast := s.Params["fast"].(float64)
		require.True(t, fast >= 3 && fast <= 10 && fast == float64(int(fast)))
		require.Contains(t, []string{"4h", "1d"}, s.Timeframe)
	}
	again, err := spec.Expand()
	require.NoError(t, err)
	require.Equal(t, len(specs), len(again))
	require.Equal(t, specs[0].Hash(), again[0].Hash(), "seeded")

	spec.Random.Ranges = map[string]Range{"params.size": {Min: 0.1, Max: 0.2}}
	specs, err = spec.Expand()
	require.NoError(t, err)
	value := specs[0].Params["size"].(float64)
	require.True(t, value >= 0.1 && value <= 0.2)
}

func TestValidate(t *testing.T) {
	spec := loadSpec(t)
	both := spec
	both.Random = &Random{Samples: 1}
	require.ErrorIs(t, both.Validate(), backtest.ErrInvalidSpec)

	neither := spec
	neither.Grid = nil
	require.ErrorIs(t, neither.Validate(), backtest.ErrInvalidSpec)

	badKey := spec
	badKey.Grid = map[string][]any{"capital": {1}}
	require.ErrorContains(t, badKey.Validate(), `cannot vary "capital"`)

	empty := spec
	empty.Grid = map[string][]any{"params.fast": {}}
	require.ErrorIs(t, empty.Validate(), backtest.ErrInvalidSpec)

	random := spec
	random.Grid = nil
	random.Random = &Random{Samples: 0}
	require.ErrorIs(t, random.Validate(), backtest.ErrInvalidSpec)
	random.Random = &Random{Samples: 1, Ranges: map[string]Range{"params.fast": {Min: 5, Max: 1}}}
	require.ErrorIs(t, random.Validate(), backtest.ErrInvalidSpec)

	timeframe := spec
	timeframe.Grid = map[string][]any{"timeframe": {4}}
	_, err := timeframe.Expand()
	require.ErrorIs(t, err, backtest.ErrInvalidSpec)
}

func TestRun_RankAndCache(t *testing.T) {
	out := t.TempDir()
	spec := loadSpec(t)

	summary, err := Run(context.Background(), spec, exampleDir, strategy.Default, Options{Out: out, Top: 5})
	require.NoError(t, err)
	require.Equal(t, 12, summary.Runs)
	require.Zero(t, summary.Cached)
	require.Len(t, summary.Ranking, 5)
	for i := 1; i < len(summary.Ranking); i++ {
		require.GreaterOrEqual(t, summary.Ranking[i-1].Metrics.SharpeAnnual, summary.Ranking[i].Metrics.SharpeAnnual)
	}
	require.Equal(t, 12, countLines(t, filepath.Join(out, RunsFile)))
	_, err = os.Stat(filepath.Join(out, SummaryFile))
	require.NoError(t, err)

	// a second run reuses everything and ranks identically
	again, err := Run(context.Background(), spec, exampleDir, strategy.Default, Options{Out: out, Top: 5, Parallel: 1})
	require.NoError(t, err)
	require.Equal(t, 12, again.Cached)
	require.Equal(t, summary.Ranking, again.Ranking)

	// extending the grid only runs the new sets, and keeps the old lines
	extended := loadSpec(t)
	extended.Grid["params.fast"] = []any{5, 8, 13, 3}
	more, err := Run(context.Background(), extended, exampleDir, strategy.Default,
		Options{Out: out, Metric: "max_drawdown"})
	require.NoError(t, err)
	require.Equal(t, 16, more.Runs)
	require.Equal(t, 12, more.Cached)
	require.Equal(t, 16, countLines(t, filepath.Join(out, RunsFile)))
	for i := 1; i < len(more.Ranking); i++ {
		require.LessOrEqual(t, more.Ranking[i-1].Metrics.MaxDrawdown, more.Ranking[i].Metrics.MaxDrawdown)
	}
}

func TestRun_FullAndFailures(t *testing.T) {
	out := t.TempDir()
	spec := loadSpec(t)
	spec.Grid = map[string][]any{"params.fast": {8, 40}} // 40 >= slow: invalid params

	summary, err := Run(context.Background(), spec, exampleDir, strategy.Default, Options{Out: out, Full: true})
	require.NoError(t, err)
	require.Equal(t, 2, summary.Runs)
	require.Equal(t, 1, summary.Failed)
	require.Len(t, summary.Ranking, 1)

	entries, err := os.ReadDir(filepath.Join(out, RunsDir))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	for _, name := range []string{"result.json", "trades.csv", "equity.csv", "spec.json"} {
		_, err := os.Stat(filepath.Join(out, RunsDir, entries[0].Name(), name))
		require.NoError(t, err, name)
	}

	_, err = Run(context.Background(), spec, exampleDir, strategy.Default, Options{})
	require.Error(t, err, "out is required")
	_, err = Run(context.Background(), spec, exampleDir, strategy.Default, Options{Out: out, Metric: "luck"})
	require.ErrorIs(t, err, backtest.ErrInvalidSpec)
}
