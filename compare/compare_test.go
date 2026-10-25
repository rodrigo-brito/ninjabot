package compare

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/backtest"
	_ "github.com/rodrigo-brito/ninjabot/examples/strategies"
	"github.com/rodrigo-brito/ninjabot/portfolio"
	"github.com/rodrigo-brito/ninjabot/strategy"
)

func writeBacktest(t *testing.T, dir string, params map[string]any) {
	t.Helper()
	file, err := os.Open("../examples/specs/emacross.json")
	require.NoError(t, err)
	defer file.Close()
	spec, err := backtest.ReadSpec(file)
	require.NoError(t, err)
	spec.Params = params
	ds, err := spec.LoadDataset("../examples/specs")
	require.NoError(t, err)
	engine, err := backtest.FromSpec(ds, spec, strategy.Default)
	require.NoError(t, err)
	result, err := engine.Run(context.Background())
	require.NoError(t, err)

	require.NoError(t, os.MkdirAll(dir, 0o755))
	out, err := os.Create(filepath.Join(dir, "result.json"))
	require.NoError(t, err)
	defer out.Close()
	require.NoError(t, result.WriteJSON(out))
}

func writePortfolio(t *testing.T, dir string) {
	t.Helper()
	file, err := os.Open("../examples/specs/portfolio.json")
	require.NoError(t, err)
	defer file.Close()
	spec, err := portfolio.ReadSpec(file)
	require.NoError(t, err)
	ds, err := spec.Runs()[0].LoadDataset("../examples/specs")
	require.NoError(t, err)
	result, err := portfolio.Run(context.Background(), ds, spec, strategy.Default, portfolio.Options{})
	require.NoError(t, err)

	require.NoError(t, os.MkdirAll(dir, 0o755))
	out, err := os.Create(filepath.Join(dir, "portfolio.json"))
	require.NoError(t, err)
	defer out.Close()
	require.NoError(t, result.WriteJSON(out))
}

func TestCompare(t *testing.T) {
	log.SetLevel(log.ErrorLevel)
	root := t.TempDir()
	writeBacktest(t, filepath.Join(root, "fast5"), map[string]any{"fast": 5})
	writeBacktest(t, filepath.Join(root, "fast8"), map[string]any{"fast": 8})
	writePortfolio(t, filepath.Join(root, "basket"))

	comparison, err := Compare([]string{
		filepath.Join(root, "fast5"),
		filepath.Join(root, "fast8", "result.json"),
		filepath.Join(root, "basket"),
	}, nil)
	require.NoError(t, err)
	require.Equal(t, DefaultMetrics, comparison.Metrics)
	require.Len(t, comparison.Runs, 3)
	require.Equal(t, "fast5", comparison.Runs[0].Name)
	require.Equal(t, KindBacktest, comparison.Runs[1].Kind)
	require.Equal(t, KindPortfolio, comparison.Runs[2].Kind)

	for metric, delta := range comparison.Deltas[0] {
		require.Zero(t, delta, metric)
	}
	require.InDelta(t, comparison.Runs[1].Metrics.NetProfit-comparison.Runs[0].Metrics.NetProfit,
		comparison.Deltas[1]["net_profit"], 1e-9)

	basket := comparison.Runs[2].Metrics
	require.Equal(t, 29, basket.Trades)
	require.Positive(t, basket.SharpeAnnual)
	require.Positive(t, basket.ProfitFactor)

	_, err = Compare(nil, nil)
	require.Error(t, err)
	_, err = Compare([]string{filepath.Join(root, "fast5")}, []string{"luck"})
	require.ErrorContains(t, err, "unknown metric")
	_, err = Compare([]string{filepath.Join(root, "missing")}, nil)
	require.Error(t, err)

	broken := filepath.Join(root, "broken.json")
	require.NoError(t, os.WriteFile(broken, []byte("{"), 0o600))
	_, err = Load(broken)
	require.Error(t, err)
	require.NoError(t, os.WriteFile(broken, []byte(`{"schema_version": 9}`), 0o600))
	_, err = Load(broken)
	require.Error(t, err)
	require.NoError(t, os.WriteFile(broken, []byte(`{"schema_version": 9, "combined": {}}`), 0o600))
	_, err = Load(broken)
	require.Error(t, err)
}
