package portfolio

import (
	"bytes"
	"context"
	"encoding/csv"
	"os"
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

func TestAlignForwardFill(t *testing.T) {
	times, aligned := AlignForwardFill(map[string][]backtest.Point{
		"hourly": {{Time: 3600, Value: 10}, {Time: 7200, Value: 11}, {Time: 10800, Value: 12}, {Time: 14400, Value: 13}},
		"slow":   {{Time: 7200, Value: 100}, {Time: 14400, Value: 90}},
		"empty":  {},
	}, map[string]float64{"hourly": 5, "slow": 50, "empty": 7})

	require.Equal(t, []int64{3600, 7200, 10800, 14400}, times)
	require.Equal(t, []float64{10, 11, 12, 13}, aligned["hourly"])
	require.Equal(t, []float64{50, 100, 100, 90}, aligned["slow"], "initial value, then forward fill")
	require.Equal(t, []float64{7, 7, 7, 7}, aligned["empty"])
}

func TestSampleAndReturns(t *testing.T) {
	times := []int64{3600, 7200, 10800, 14400}
	require.Equal(t, []float64{2, 4}, sample(times, []float64{1, 2, 3, 4}, 7200))
	require.Equal(t, []float64{1, 2, 3, 4}, sample(times, []float64{1, 2, 3, 4}, 0))
	require.Equal(t, []float64{1, 0.5, -1}, returns([]float64{1, 2, 3, 0}))
	require.Equal(t, []float64{0}, returns([]float64{0, 5}), "a zero level has no return")
}

func loadSpec(t *testing.T) (Spec, *backtest.Dataset) {
	t.Helper()
	file, err := os.Open("../examples/specs/portfolio.json")
	require.NoError(t, err)
	defer file.Close()
	spec, err := ReadSpec(file)
	require.NoError(t, err)
	ds, err := spec.Runs()[0].LoadDataset("../examples/specs")
	require.NoError(t, err)
	return spec, ds
}

func TestRun(t *testing.T) {
	spec, ds := loadSpec(t)
	result, err := Run(context.Background(), ds, spec, strategy.Default, Options{})
	require.NoError(t, err)

	require.Len(t, result.Strategies, 2)
	require.Equal(t, "trend", result.Strategies[0].ID)
	require.Equal(t, 5000.0, result.Strategies[0].Capital)
	require.Equal(t, 3000.0, result.Strategies[1].Capital)
	require.Equal(t, []string{"BTCUSDT"}, result.Strategies[1].Result.Config.Pairs)

	combined := result.Combined
	require.InDelta(t, 2000.0, combined.Unallocated, 1e-9)
	require.Equal(t, 10000.0, combined.Initial)
	for i := range combined.Times {
		sum := combined.Unallocated
		for _, id := range spec.IDs() {
			sum += combined.ByStrategy[id][i]
		}
		require.InDelta(t, sum, combined.Equity[i], 1e-6, "time %d", combined.Times[i])
	}
	finals := combined.Unallocated + result.Strategies[0].Result.Wallet.Final + result.Strategies[1].Result.Wallet.Final
	require.InDelta(t, finals, combined.Final, 1e-6)
	require.InDelta(t, combined.Final-combined.Initial, combined.NetProfit, 1e-9)

	// each strategy matches the same run on its own
	for i, run := range spec.Runs() {
		engine, err := backtest.FromSpec(ds, run, strategy.Default)
		require.NoError(t, err)
		alone, err := engine.Run(context.Background())
		require.NoError(t, err)
		require.Equal(t, alone.Trades, result.Strategies[i].Result.Trades)
	}

	correlation := combined.Correlation
	require.Equal(t, []string{"trend", "breakout"}, correlation.IDs)
	require.Equal(t, 1.0, correlation.Values[0][0])
	require.Equal(t, correlation.Values[0][1], correlation.Values[1][0])
	require.True(t, correlation.Values[0][1] > -1 && correlation.Values[0][1] < 1)

	require.Len(t, combined.Contribution, 2)
	share := combined.Contribution[0].PnLShare + combined.Contribution[1].PnLShare
	require.InDelta(t, 1.0, share, 1e-9)
	require.NotZero(t, combined.SharpeAnnual)
	require.Positive(t, combined.MaxDrawdown)
	require.NotNil(t, combined.Statistics)
	require.Equal(t, len(result.Strategies[0].Result.Trades)+len(result.Strategies[1].Result.Trades),
		combined.Statistics.Sample.Trades)
}

func TestRun_Deterministic(t *testing.T) {
	spec, ds := loadSpec(t)
	encode := func(parallel int) string {
		result, err := Run(context.Background(), ds, spec, strategy.Default, Options{Parallel: parallel, NoStatistics: true})
		require.NoError(t, err)
		var buffer bytes.Buffer
		require.NoError(t, result.WriteJSON(&buffer))
		return buffer.String()
	}
	serial := encode(1)
	require.Equal(t, serial, encode(4))
	require.Equal(t, serial, encode(0))

	decoded, err := ReadResult(bytes.NewBufferString(serial))
	require.NoError(t, err)
	require.Len(t, decoded.Strategies, 2)
	_, err = ReadResult(bytes.NewBufferString(`{"schema_version": 7}`))
	require.Error(t, err)
}

func TestWriteEquityCSV(t *testing.T) {
	spec, ds := loadSpec(t)
	result, err := Run(context.Background(), ds, spec, strategy.Default, Options{NoStatistics: true})
	require.NoError(t, err)
	var buffer bytes.Buffer
	require.NoError(t, result.WriteEquityCSV(&buffer))
	rows, err := csv.NewReader(&buffer).ReadAll()
	require.NoError(t, err)
	require.Equal(t, []string{"time", "portfolio", "trend", "breakout"}, rows[0])
	require.Len(t, rows, len(result.Combined.Times)+1)
}

func TestSpec_Validate(t *testing.T) {
	spec, _ := loadSpec(t)
	require.NoError(t, spec.Validate())
	require.Len(t, spec.Hash(), 64)
	require.Equal(t, spec.Hash(), spec.Normalize().Hash())

	broken := spec
	broken.Mode = "hedged"
	broken.Strategies = []StrategySpec{
		{ID: "a", Strategy: "emacross", Weight: 0.7},
		{ID: "a", Strategy: "turtle", Weight: 0.6, Pairs: []string{"XRPUSDT"}},
		{Strategy: "turtle", Weight: 0},
	}
	err := broken.Validate()
	require.ErrorIs(t, err, backtest.ErrInvalidSpec)
	for _, problem := range []string{
		`mode "hedged" is not supported`, `id "a" is used twice`, "strategies[2].id is required",
		"strategies[2].weight must be positive", "the weights add up to 1.3",
		"strategies[1] (a): pair XRPUSDT has no data source",
	} {
		require.Contains(t, err.Error(), problem)
	}

	_, err = Run(context.Background(), nil, broken, strategy.Default, Options{})
	require.ErrorIs(t, err, backtest.ErrInvalidSpec)

	unknown := spec
	unknown.Strategies = []StrategySpec{{ID: "x", Strategy: "nope", Weight: 1}}
	_, ds := loadSpec(t)
	_, err = Run(context.Background(), ds, unknown, strategy.Default, Options{})
	require.ErrorIs(t, err, strategy.ErrUnknownStrategy)
}

func TestRun_Shared(t *testing.T) {
	spec, ds := loadSpec(t)
	spec.Mode = ModeShared
	result, err := Run(context.Background(), ds, spec, strategy.Default, Options{})
	require.NoError(t, err)
	require.NotNil(t, result.Account)
	require.Len(t, result.Strategies, 2)
	require.Equal(t, "emacross", result.Strategies[0].Result.Config.Strategy)
	require.Equal(t, 5000.0, result.Strategies[0].Capital)

	// the attribution adds up to the shared wallet
	combined := result.Combined
	require.InDelta(t, result.Account.Wallet.Final, combined.Final, 1e-6)
	trades := 0
	for _, s := range result.Strategies {
		trades += len(s.Result.Trades)
		for _, trade := range s.Result.Trades {
			require.Equal(t, s.ID, trade.Tag)
		}
	}
	require.Equal(t, len(result.Account.Trades), trades)

	isolated, err := Run(context.Background(), ds, loadSpecOnly(t), strategy.Default, Options{NoStatistics: true})
	require.NoError(t, err)
	// with budgets that never compete for cash, sharing the wallet takes the
	// same trades as isolated wallets
	require.Equal(t, len(isolated.Strategies[1].Result.Trades), len(result.Strategies[1].Result.Trades))
}

func loadSpecOnly(t *testing.T) Spec {
	spec, _ := loadSpec(t)
	return spec
}
