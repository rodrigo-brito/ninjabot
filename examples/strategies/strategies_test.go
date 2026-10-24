package strategies

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/backtest"
	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/strategy"
	"github.com/rodrigo-brito/ninjabot/ui"
	uiindicator "github.com/rodrigo-brito/ninjabot/ui/indicator"
)

func run(t *testing.T, ds *backtest.Dataset, name string, params map[string]any) *backtest.Result {
	t.Helper()
	s, err := strategy.Default.New(name, params)
	require.NoError(t, err)
	engine, err := backtest.New(ds, backtest.Config{
		Strategy: s, Quote: "USDT", Balances: map[string]float64{"USDT": 10000},
		MakerFee: 0.001, TakerFee: 0.001, NoStatistics: true,
	})
	require.NoError(t, err)
	result, err := engine.Run(context.Background())
	require.NoError(t, err)
	return result
}

// TestExamples records the results of the example strategies with their
// default parameters over the sample data. The strategies compute their
// indicators incrementally over the whole history since ROADMAP 2.4; the
// numbers changed then and are the reference from there on.
func TestExamples(t *testing.T) {
	log.SetLevel(log.ErrorLevel)
	ds, err := backtest.LoadDataset(
		exchange.PairFeed{Pair: "BTCUSDT", File: "../../testdata/btc-1h.csv", Timeframe: "1h"},
		exchange.PairFeed{Pair: "ETHUSDT", File: "../../testdata/eth-1h.csv", Timeframe: "1h"},
	)
	require.NoError(t, err)

	for _, tc := range []struct {
		name   string
		trades int
		final  float64
	}{
		{"emacross", 23, 43225.2687},
		{"turtle", 24, 20199.1942},
		{"ocosell", 43, 14251.4000},
		{"trailing", 31, 13451.0981},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := run(t, ds, tc.name, nil)
			require.Len(t, result.Trades, tc.trades)
			require.InDelta(t, tc.final, result.Wallet.Final, 0.01)

			again := run(t, ds, tc.name, nil)
			require.Equal(t, result.Trades, again.Trades, "deterministic")
		})
	}
}

func TestRegistered(t *testing.T) {
	names := map[string]bool{}
	for _, def := range strategy.Default.Describe() {
		names[def.Name] = true
		require.NotEmpty(t, def.Description)
		require.NotEmpty(t, def.Params)
	}
	for _, name := range []string{"emacross", "turtle", "ocosell", "trailing"} {
		require.True(t, names[name], name)
	}

	_, err := strategy.Default.New("emacross", map[string]any{"fast": 30, "slow": 20})
	require.ErrorIs(t, err, strategy.ErrInvalidParams)

	s, err := strategy.Default.New("emacross", map[string]any{"timeframe": "1h", "fast": 5})
	require.NoError(t, err)
	require.Equal(t, "1h", s.Timeframe())
	require.Equal(t, 22, s.WarmupPeriod())

	zero := new(CrossEMA)
	require.Equal(t, "4h", zero.Timeframe(), "the zero value uses the defaults")
}

// TestDashboardIntegration runs a streaming strategy with the dashboard
// subscribed, the way examples/backtesting does, and checks the dashboard
// receives the indicators the strategy computed.
func TestDashboardIntegration(t *testing.T) {
	log.SetLevel(log.ErrorLevel)
	ds, err := backtest.LoadDataset(
		exchange.PairFeed{Pair: "BTCUSDT", File: "../../testdata/btc-1h.csv", Timeframe: "1h"},
	)
	require.NoError(t, err)

	s := new(CrossEMA)
	chart, err := ui.New(ui.WithStrategyIndicators(s), ui.WithCustomIndicators(uiindicator.RSI(14, "purple")))
	require.NoError(t, err)

	engine, err := backtest.New(ds, backtest.Config{
		Strategy: s, Quote: "USDT", Balances: map[string]float64{"USDT": 10000}, NoStatistics: true,
		CandleSubscribers:  []func(model.Candle){chart.OnCandle},
		OrderSubscribers:   []func(model.Order){chart.OnOrder},
		IndicatorObservers: []strategy.IndicatorObserver{chart},
	})
	require.NoError(t, err)
	result, err := engine.Run(context.Background())
	require.NoError(t, err)

	server := httptest.NewServer(chart.Handler(context.Background()))
	defer server.Close()
	response, err := http.Get(server.URL + "/api/BTCUSDT/snapshot")
	require.NoError(t, err)
	defer response.Body.Close()
	var snapshot ui.Snapshot
	require.NoError(t, json.NewDecoder(response.Body).Decode(&snapshot))

	require.Len(t, snapshot.Indicators, 2)
	require.Equal(t, "RSI(14)", snapshot.Indicators[0].Name)
	require.Equal(t, "MA's", snapshot.Indicators[1].Name)
	ma := snapshot.Indicators[1].Metrics
	require.Len(t, ma, 2)
	require.Equal(t, "EMA 8", ma[0].Name)
	// one point per closed 4h candle from the warmup index on, like the
	// dashboard always plotted
	require.Equal(t, len(snapshot.Candles)-s.WarmupPeriod(), len(ma[0].Points))
	require.Len(t, snapshot.Orders, len(result.Orders))
}
