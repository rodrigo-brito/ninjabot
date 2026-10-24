package ui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/backtest"
	"github.com/rodrigo-brito/ninjabot/order"
	"github.com/rodrigo-brito/ninjabot/portfolio"
)

func TestServer_Portfolio(t *testing.T) {
	c := newTestChart(t, WithUIDir(t.TempDir()))
	server := httptest.NewServer(c.Handler(context.Background()))
	defer server.Close()

	response, err := http.Get(server.URL + "/api/portfolio")
	require.NoError(t, err)
	response.Body.Close()
	require.Equal(t, http.StatusNotFound, response.StatusCode)

	result := &portfolio.Result{
		Spec: portfolio.Spec{Name: "basket", Quote: "USDT"},
		Strategies: []portfolio.StrategyResult{
			{ID: "a", Weight: 0.5, Capital: 500, Result: &backtest.Result{
				Config:  backtest.ConfigEcho{Strategy: "emacross"},
				Metrics: backtest.Metrics{Trades: 3, ReturnPct: 0.1},
				Trades:  []order.Result{{ClosedAt: time.Unix(10, 0)}},
			}},
		},
		Combined: portfolio.Combined{
			Times:        []int64{1, 2},
			Equity:       []float64{1000, 1050},
			ByStrategy:   map[string][]float64{"a": {500, 550}},
			Initial:      1000,
			Final:        1050,
			Correlation:  portfolio.Matrix{IDs: []string{"a"}, Values: [][]float64{{1}}},
			Contribution: []portfolio.Contribution{{ID: "a", PnLShare: 1, MarginalSharpe: 0.5}},
		},
	}
	c = newTestChart(t, WithUIDir(t.TempDir()), WithPortfolio(result))
	server = httptest.NewServer(c.Handler(context.Background()))
	defer server.Close()

	response, err = http.Get(server.URL + "/api/portfolio")
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	var view PortfolioView
	require.NoError(t, json.NewDecoder(response.Body).Decode(&view))
	require.Equal(t, "basket", view.Name)
	require.Equal(t, []Point{{Time: 1, Value: 1000}, {Time: 2, Value: 1050}}, view.Equity)
	require.Len(t, view.Strategies, 1)
	require.Equal(t, "emacross", view.Strategies[0].Strategy)
	require.Equal(t, 3, view.Strategies[0].Trades)
	require.Equal(t, 0.5, view.Strategies[0].MarginalSharpe)
	require.Equal(t, []Point{{Time: 1, Value: 500}, {Time: 2, Value: 550}}, view.Strategies[0].Equity)
	require.Equal(t, [][]float64{{1}}, view.Correlation)
}
