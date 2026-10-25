package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/backtest"
	_ "github.com/rodrigo-brito/ninjabot/examples/strategies"
	"github.com/rodrigo-brito/ninjabot/strategy"
)

const exampleSpec = "../examples/specs/emacross.json"

func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = Run(append([]string{"ninjabot"}, args...), &out, &errOut, strategy.Default)
	return code, out.String(), errOut.String()
}

func writeSpec(t *testing.T, mutate func(map[string]any)) string {
	t.Helper()
	data, err := os.ReadFile(exampleSpec)
	require.NoError(t, err)
	var spec map[string]any
	require.NoError(t, json.Unmarshal(data, &spec))
	// data files relative to the temporary directory
	for _, source := range spec["data"].([]any) {
		file := source.(map[string]any)["file"].(string)
		absolute, err := filepath.Abs(filepath.Join(filepath.Dir(exampleSpec), file))
		require.NoError(t, err)
		source.(map[string]any)["file"] = absolute
	}
	mutate(spec)
	data, err = json.Marshal(spec)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "spec.json")
	require.NoError(t, os.WriteFile(path, data, 0o600))
	return path
}

func TestBacktest_Artifacts(t *testing.T) {
	out := t.TempDir()
	code, stdout, stderr := run(t, "backtest", "--spec", exampleSpec, "--out", out)
	require.Equal(t, ExitOK, code, stderr)
	require.Contains(t, stdout, "trades            23")

	file, err := os.Open(filepath.Join(out, ResultFile))
	require.NoError(t, err)
	defer file.Close()
	result, err := backtest.ReadResult(file)
	require.NoError(t, err)
	require.Equal(t, "emacross", result.Config.Strategy)
	require.Len(t, result.Trades, 23)
	require.InDelta(t, 43225.2687, result.Wallet.Final, 0.01)
	require.NotNil(t, result.Portfolio)
	require.Len(t, result.Pairs, 2)

	for _, name := range []string{TradesFile, EquityFile, OrdersFile} {
		data, err := os.ReadFile(filepath.Join(out, name))
		require.NoError(t, err)
		rows, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
		require.NoError(t, err, name)
		require.Greater(t, len(rows), 1, name)
	}
	trades, _ := os.ReadFile(filepath.Join(out, TradesFile))
	require.Equal(t, 24, strings.Count(string(trades), "\n"), "header and one row per trade")

	spec, err := os.ReadFile(filepath.Join(out, SpecFile))
	require.NoError(t, err)
	parsed, err := backtest.ParseSpec(spec)
	require.NoError(t, err)
	require.Equal(t, []string{"BTCUSDT", "ETHUSDT"}, parsed.Pairs, "the normalized spec makes the defaults explicit")
}

func TestBacktest_Stdout(t *testing.T) {
	code, stdout, stderr := run(t, "backtest", "--spec", exampleSpec, "--out", "-", "--no-statistics")
	require.Equal(t, ExitOK, code, stderr)
	result, err := backtest.ReadResult(strings.NewReader(stdout))
	require.NoError(t, err)
	require.Nil(t, result.Portfolio)

	code, stdout, _ = run(t, "backtest", "--spec", exampleSpec, "--format", "json", "--no-statistics")
	require.Equal(t, ExitOK, code)
	_, err = backtest.ReadResult(strings.NewReader(stdout))
	require.NoError(t, err)

	code, stdout, _ = run(t, "backtest", "--spec", exampleSpec, "--quiet", "--no-statistics")
	require.Equal(t, ExitOK, code)
	require.Empty(t, stdout)
}

func TestBacktest_ExitCodes(t *testing.T) {
	for name, tc := range map[string]struct {
		args []string
		code int
		text string
	}{
		"missing spec flag": {[]string{"backtest"}, ExitUsage, "spec"},
		"spec not found":    {[]string{"backtest", "--spec", "nope.json"}, ExitUsage, "read spec"},
		"bad format":        {[]string{"backtest", "--spec", exampleSpec, "--format", "xml"}, ExitUsage, "--format"},
		"invalid spec": {[]string{"backtest", "--spec", writeSpec(t, func(s map[string]any) { s["capital"] = 0 })},
			ExitUsage, "capital must be positive"},
		"unknown field": {[]string{"backtest", "--spec", writeSpec(t, func(s map[string]any) { s["captal"] = 1 })},
			ExitUsage, "captal"},
		"missing data": {[]string{"backtest", "--spec", writeSpec(t, func(s map[string]any) {
			s["data"].([]any)[0].(map[string]any)["file"] = "/nonexistent.csv"
		})}, ExitData, "nonexistent"},
		"unknown strategy": {[]string{"backtest", "--spec", writeSpec(t, func(s map[string]any) { s["strategy"] = "nope" })},
			ExitStrategy, "unknown strategy"},
		"invalid params": {[]string{"backtest", "--spec", writeSpec(t, func(s map[string]any) {
			s["params"] = map[string]any{"fast": 50}
		})}, ExitStrategy, "fast period (50) must be below"},
		"schema without kind": {[]string{"schema"}, ExitUsage, "schema needs"},
	} {
		t.Run(name, func(t *testing.T) {
			code, _, stderr := run(t, tc.args...)
			require.Equal(t, tc.code, code, stderr)
			require.Contains(t, stderr, tc.text)
		})
	}
}

func TestStrategies(t *testing.T) {
	code, stdout, _ := run(t, "strategies")
	require.Equal(t, ExitOK, code)
	require.Contains(t, stdout, "emacross")
	require.Contains(t, stdout, "default 8")

	code, stdout, _ = run(t, "strategies", "--json")
	require.Equal(t, ExitOK, code)
	var descriptions []map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &descriptions))
	require.NotEmpty(t, descriptions)
	require.Contains(t, descriptions[0], "schema")
	require.Contains(t, descriptions[0], "params")
}

func TestValidate(t *testing.T) {
	code, stdout, stderr := run(t, "validate", "--spec", exampleSpec)
	require.Equal(t, ExitOK, code, stderr)
	require.Regexp(t, `^ok [0-9a-f]{64}\n$`, stdout)

	code, _, _ = run(t, "validate", "--spec", writeSpec(t, func(s map[string]any) {
		s["params"] = map[string]any{"fats": 3}
	}))
	require.Equal(t, ExitStrategy, code)

	code, _, _ = run(t, "validate", "--spec", writeSpec(t, func(s map[string]any) {
		s["data"].([]any)[0].(map[string]any)["file"] = "/nonexistent.csv"
	}))
	require.Equal(t, ExitData, code)
}

func TestSchema(t *testing.T) {
	for _, args := range [][]string{{"schema", "spec"}, {"schema", "result"}, {"schema", "strategy", "turtle"}} {
		code, stdout, stderr := run(t, args...)
		require.Equal(t, ExitOK, code, stderr)
		var schema map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &schema))
		require.Equal(t, "https://json-schema.org/draft/2020-12/schema", schema["$schema"])
	}
	code, _, _ := run(t, "schema", "strategy", "nope")
	require.Equal(t, ExitStrategy, code)
}

func TestExitCode(t *testing.T) {
	require.Equal(t, ExitOK, ExitCode(nil))
	require.Equal(t, ExitRuntime, ExitCode(os.ErrClosed))
	require.Equal(t, ExitData, ExitCode(os.ErrNotExist))
}

func TestPortfolioSweepCompare(t *testing.T) {
	root := t.TempDir()

	code, stdout, stderr := run(t, "portfolio", "--spec", "../examples/specs/portfolio.json", "--out", filepath.Join(root, "basket"))
	require.Equal(t, ExitOK, code, stderr)
	require.Contains(t, stdout, "correlation")
	for _, path := range []string{"portfolio.json", EquityFile, "runs/trend/result.json", "runs/breakout/trades.csv"} {
		_, err := os.Stat(filepath.Join(root, "basket", path))
		require.NoError(t, err, path)
	}

	code, stdout, _ = run(t, "portfolio", "--spec", "../examples/specs/portfolio.json", "--out", "-", "--no-statistics")
	require.Equal(t, ExitOK, code)
	require.Contains(t, stdout, `"combined"`)

	code, stdout, stderr = run(t, "sweep", "--spec", "../examples/specs/sweep.json", "--out", filepath.Join(root, "grid"), "--top", "3")
	require.Equal(t, ExitOK, code, stderr)
	require.Contains(t, stdout, "runs 12 (cached 0, failed 0)")
	require.Equal(t, 1+1+1+3, strings.Count(stdout, "\n"), "summary, overfitting, header and the top 3")
	require.Contains(t, stdout, "probability of backtest overfitting")

	code, _, stderr = run(t, "backtest", "--spec", exampleSpec, "--out", filepath.Join(root, "single"), "-q")
	require.Equal(t, ExitOK, code, stderr)

	code, stdout, stderr = run(t, "compare", filepath.Join(root, "single"), filepath.Join(root, "basket"))
	require.Equal(t, ExitOK, code, stderr)
	require.Contains(t, stdout, "sharpe_annual")
	require.Contains(t, stdout, "basket")

	code, stdout, _ = run(t, "compare", "--format", "json", "-m", "net_profit", filepath.Join(root, "single"))
	require.Equal(t, ExitOK, code)
	require.Contains(t, stdout, `"deltas"`)

	for name, tc := range map[string]struct {
		args []string
		code int
	}{
		"sweep without out":     {[]string{"sweep", "--spec", "../examples/specs/sweep.json"}, ExitUsage},
		"sweep without spec":    {[]string{"sweep", "--out", root}, ExitUsage},
		"sweep bad metric":      {[]string{"sweep", "--spec", "../examples/specs/sweep.json", "--out", root, "-m", "luck"}, ExitUsage},
		"portfolio no spec":     {[]string{"portfolio"}, ExitUsage},
		"portfolio bad file":    {[]string{"portfolio", "--spec", "nope.json"}, ExitUsage},
		"compare nothing":       {[]string{"compare"}, ExitUsage},
		"compare unknown":       {[]string{"compare", "-m", "luck", filepath.Join(root, "single")}, ExitUsage},
		"compare bad format":    {[]string{"compare", "-f", "xml", filepath.Join(root, "single")}, ExitUsage},
		"compare missing input": {[]string{"compare", filepath.Join(root, "nope")}, ExitData},
	} {
		t.Run(name, func(t *testing.T) {
			code, _, stderr := run(t, tc.args...)
			require.Equal(t, tc.code, code, stderr)
		})
	}
}

func TestWalkForwardAndSensitivity(t *testing.T) {
	code, stdout, stderr := run(t, "walkforward", "--spec", "../examples/specs/sweep.json", "--train", "60d", "--test", "30d")
	require.Equal(t, ExitOK, code, stderr)
	require.Contains(t, stdout, "walk-forward efficiency")

	code, stdout, _ = run(t, "walkforward", "--spec", "../examples/specs/sweep.json", "--train", "60d", "--test", "30d", "-f", "json")
	require.Equal(t, ExitOK, code)
	require.Contains(t, stdout, `"folds"`)

	code, stdout, stderr = run(t, "sensitivity", "--spec", exampleSpec, "--fee", "0", "--fee", "0.002")
	require.Equal(t, ExitOK, code, stderr)
	require.Contains(t, stdout, "0.2000%")

	code, stdout, _ = run(t, "sensitivity", "--spec", exampleSpec, "--fee", "0.001", "-f", "json")
	require.Equal(t, ExitOK, code)
	require.Contains(t, stdout, `"fee": 0.001`)

	code, _, _ = run(t, "walkforward", "--spec", "../examples/specs/sweep.json", "--train", "bad")
	require.Equal(t, ExitUsage, code)
	code, _, _ = run(t, "walkforward")
	require.Equal(t, ExitUsage, code)
	code, _, _ = run(t, "sensitivity", "--spec", exampleSpec, "--fee", "2")
	require.Equal(t, ExitUsage, code)
}

func TestResolvedSpecs(t *testing.T) {
	top := writeSpec(t, func(s map[string]any) {
		s["params"] = map[string]any{"fast": 8}
		s["timeframe"] = "1h"
	})
	inParams := writeSpec(t, func(s map[string]any) {
		s["params"] = map[string]any{"fast": 8, "slow": 21, "min_quote": 10, "timeframe": "1h"}
	})

	_, first, _ := run(t, "validate", "--spec", top)
	_, second, _ := run(t, "validate", "--spec", inParams)
	require.Equal(t, first, second, "the same run hashes the same, however it is written")

	out := t.TempDir()
	code, _, stderr := run(t, "backtest", "--spec", top, "--out", out, "-q", "--no-statistics")
	require.Equal(t, ExitOK, code, stderr)
	data, err := os.ReadFile(filepath.Join(out, SpecFile))
	require.NoError(t, err)
	spec, err := backtest.ParseSpec(data)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"fast": 8.0, "slow": 21.0, "min_quote": 10.0, "timeframe": "1h"}, spec.Params,
		"defaults explicit, the timeframe where the strategy reads it")
	require.Empty(t, spec.Timeframe)

	file, err := os.Open(filepath.Join(out, ResultFile))
	require.NoError(t, err)
	defer file.Close()
	result, err := backtest.ReadResult(file)
	require.NoError(t, err)
	require.Equal(t, "1h", result.Config.Timeframe)
	require.Equal(t, "1h", result.Config.Params["timeframe"])
}

func TestValidate_SweepAndPortfolio(t *testing.T) {
	code, stdout, stderr := run(t, "validate", "--spec", "../examples/specs/sweep.json")
	require.Equal(t, ExitOK, code, stderr)
	require.Equal(t, "ok sweep of 12 runs\n", stdout)

	code, stdout, stderr = run(t, "validate", "--spec", "../examples/specs/portfolio.json")
	require.Equal(t, ExitOK, code, stderr)
	require.Contains(t, stdout, "ok portfolio of 2 strategies")

	code, _, _ = run(t, "validate", "--spec", "../examples/specs/emacross.yaml")
	require.Equal(t, ExitOK, code)
	code, _, _ = run(t, "validate")
	require.Equal(t, ExitUsage, code)
}
