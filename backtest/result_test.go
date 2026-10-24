package backtest

import (
	"bytes"
	"encoding/csv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResult_Statistics(t *testing.T) {
	ds := sampleDataset(t)
	result := runEngineConfig(t, ds, Config{Strategy: ocoStrategy{}, MakerFee: 0.001, TakerFee: 0.001})

	require.Equal(t, SchemaVersion, result.SchemaVersion)
	require.Equal(t, "ocoStrategy", result.Config.Strategy)
	require.Equal(t, []string{"BTCUSDT", "ETHUSDT"}, result.Config.Pairs)
	require.Equal(t, "4h", result.Config.Timeframe)

	require.Len(t, result.Pairs, 2)
	btc := result.Pairs["BTCUSDT"]
	eth := result.Pairs["ETHUSDT"]
	require.Equal(t, 87, btc.Sample.Trades)
	require.Equal(t, 85, eth.Sample.Trades)
	require.NotNil(t, result.Portfolio)
	require.Equal(t, 172, result.Portfolio.Sample.Trades)
	require.InDelta(t, btc.Performance.NetProfit+eth.Performance.NetProfit, result.Portfolio.Performance.NetProfit, 1e-6)
	require.InDelta(t, 6*365, result.Portfolio.Sample.BarsPerYear, 1e-6, "4h bars")

	wallet := result.Wallet
	require.Equal(t, 10000.0, wallet.Initial)
	require.InDelta(t, 8441.1310, wallet.Final, 0.001)
	require.InDelta(t, wallet.Final-wallet.Initial, wallet.NetProfit, 1e-9)
	require.InDelta(t, wallet.NetProfit+wallet.Fees, wallet.GrossProfit, 1e-9)
	require.Positive(t, wallet.Fees)
	require.Positive(t, wallet.MaxDrawdown.Value)
	require.True(t, wallet.MaxDrawdown.Start.Before(wallet.MaxDrawdown.End))
	require.Len(t, wallet.Volume, 2)
	require.Empty(t, result.Warnings)
}

func TestResult_Writers(t *testing.T) {
	ds := sampleDataset(t)
	result := runEngineConfig(t, ds, Config{Strategy: ocoStrategy{}, MakerFee: 0.001, TakerFee: 0.001})

	var buffer bytes.Buffer
	require.NoError(t, result.WriteJSON(&buffer))
	first := buffer.String()

	decoded, err := ReadResult(&buffer)
	require.NoError(t, err)
	require.Equal(t, len(result.Trades), len(decoded.Trades))
	require.Equal(t, result.Wallet.Final, decoded.Wallet.Final)

	buffer.Reset()
	require.NoError(t, decoded.WriteJSON(&buffer))
	require.Equal(t, first, buffer.String(), "a decoded result encodes to the same bytes")

	for name, write := range map[string]func(*bytes.Buffer) error{
		"trades": func(b *bytes.Buffer) error { return result.WriteTradesCSV(b) },
		"equity": func(b *bytes.Buffer) error { return result.WriteEquityCSV(b) },
		"orders": func(b *bytes.Buffer) error { return result.WriteOrdersCSV(b) },
	} {
		t.Run(name, func(t *testing.T) {
			var b bytes.Buffer
			require.NoError(t, write(&b))
			rows, err := csv.NewReader(&b).ReadAll()
			require.NoError(t, err)
			require.Greater(t, len(rows), 1)
			for _, row := range rows {
				require.Len(t, row, len(rows[0]))
			}
		})
	}

	_, err = ReadResult(bytes.NewBufferString(`{"schema_version": 99}`))
	require.Error(t, err)
}

func TestResult_Warnings(t *testing.T) {
	ds := sampleDataset(t)
	result := runEngineConfig(t, ds.Window(ds.store.sources["BTCUSDT"].candles[0].Time,
		ds.store.sources["BTCUSDT"].candles[30].Time), Config{Strategy: ocoStrategy{}, NoStatistics: true})
	require.Len(t, result.Warnings, 1)
}

func TestResult_Metrics(t *testing.T) {
	ds := sampleDataset(t)
	result := runEngineConfig(t, ds, Config{Strategy: ocoStrategy{}, MakerFee: 0.001, TakerFee: 0.001})
	m := result.Metrics
	require.Equal(t, len(result.Trades), m.Trades)
	require.Equal(t, result.Wallet.NetProfit, m.NetProfit)
	require.Equal(t, result.Wallet.MaxDrawdown.Value, m.MaxDrawdown)

	// the cheap metrics agree with the full portfolio report
	p := result.Portfolio.Performance
	require.InDelta(t, p.ProfitFactor, m.ProfitFactor, 1e-9)
	require.InDelta(t, p.Payoff, m.Payoff, 1e-9)
	require.InDelta(t, p.SQN, m.SQN, 1e-9)
	require.InDelta(t, p.SharpeAnnual, m.SharpeAnnual, 1e-9)
	require.InDelta(t, p.SortinoAnnual, m.SortinoAnnual, 1e-9)
	require.InDelta(t, p.WinRate, m.WinRate, 1e-9)

	for name := range MetricNames {
		_, err := m.Value(name)
		require.NoError(t, err, name)
	}
	_, err := m.Value("nope")
	require.Error(t, err)
	require.Zero(t, barsPerYear("bad"))
}

func TestResult_Seed(t *testing.T) {
	ds := sampleDataset(t)
	one := runEngineConfig(t, ds, Config{Strategy: ocoStrategy{}})
	same := runEngineConfig(t, ds, Config{Strategy: ocoStrategy{}, Seed: 1})
	other := runEngineConfig(t, ds, Config{Strategy: ocoStrategy{}, Seed: 42})

	require.Equal(t, one.Portfolio.MonteCarlo, same.Portfolio.MonteCarlo, "zero is the default seed")
	require.Equal(t, int64(42), other.Portfolio.Sample.Seed)
	require.NotEqual(t, one.Portfolio.MonteCarlo, other.Portfolio.MonteCarlo)
	require.Equal(t, one.Metrics, other.Metrics, "the seed only moves the resampling")
}
