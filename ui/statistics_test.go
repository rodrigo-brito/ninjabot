package ui

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/report"
)

// tradeAt builds a closed trade: a filled order carrying the result of a round
// trip, which is what the report samples.
func tradeAt(id int64, at time.Time, profit, profitValue float64) Order {
	return Order{
		ID:          id,
		Pair:        "BTCUSDT",
		Side:        string(model.SideTypeSell),
		Type:        string(model.OrderTypeMarket),
		Status:      string(model.OrderStatusTypeFilled),
		Price:       100,
		Quantity:    1,
		CreatedAt:   at.Unix(),
		UpdatedAt:   at.Unix(),
		Profit:      profit,
		ProfitValue: profitValue,
	}
}

// profitableInput is a backtest with a small but consistent edge: two winners
// for every loser, over an hourly series.
func profitableInput(t *testing.T, trades int) statisticsInput {
	t.Helper()

	base := time.Date(2021, 9, 26, 20, 0, 0, 0, time.UTC)
	input := statisticsInput{Pair: "BTCUSDT", Quote: "USDT"}

	equity := 10000.0
	price := 100.0
	for i := 0; i < trades; i++ {
		at := base.Add(time.Duration(i) * time.Hour)

		profit, value := 0.02, 200.0
		if i%3 == 2 {
			profit, value = -0.01, -100.0
		}

		equity += value
		price += value / 20

		input.Orders = append(input.Orders, tradeAt(int64(i+1), at, profit, value))
		input.Equity = append(input.Equity, Point{Time: at.Unix(), Value: equity})
		input.Candles = append(input.Candles, Candle{Time: at.Unix(), Close: price, Open: price, High: price, Low: price})
	}

	return input
}

func TestBuildStatistics_Sample(t *testing.T) {
	statistics := buildStatistics(profitableInput(t, 60))

	assert.Equal(t, "BTCUSDT", statistics.Pair)
	assert.Equal(t, "USDT", statistics.Quote)
	assert.Equal(t, 60, statistics.Sample.Trades)
	assert.Equal(t, 40, statistics.Sample.Wins)
	assert.Equal(t, 20, statistics.Sample.Losses)
	assert.Equal(t, 60, statistics.Sample.Bars)
	assert.Equal(t, report.Iterations, statistics.Sample.Iterations)
	assert.InDelta(t, 24*365.0, statistics.Sample.BarsPerYear, 1, "hourly candles")
}

func TestBuildStatistics_IgnoresOrdersThatAreNotTrades(t *testing.T) {
	input := profitableInput(t, 30)
	base := time.Date(2021, 9, 26, 20, 0, 0, 0, time.UTC)

	pending := tradeAt(900, base, 0.5, 500)
	pending.Status = string(model.OrderStatusTypeNew)
	entry := tradeAt(901, base, 0, 0)

	input.Orders = append(input.Orders, pending, entry)

	statistics := buildStatistics(input)
	assert.Equal(t, 30, statistics.Sample.Trades, "only filled orders with a result are trades")
}

func TestBuildStatistics_Performance(t *testing.T) {
	statistics := buildStatistics(profitableInput(t, 60))
	performance := statistics.Performance

	assert.InDelta(t, 6000.0, performance.NetProfit, 1e-6, "40 winners of 200 and 20 losers of 100")
	assert.InDelta(t, 100.0, performance.Expectancy, 1e-6)
	assert.InDelta(t, 2.0/3.0, performance.WinRate, 1e-9)
	assert.InDelta(t, 0.01, performance.MeanReturn, 1e-9)
	assert.InDelta(t, 0.02, performance.MedianReturn, 1e-9)
	assert.InDelta(t, 2.0, performance.Payoff, 1e-9, "winners are twice the losers")
	assert.InDelta(t, 4.0, performance.ProfitFactor, 1e-9)
	assert.Positive(t, performance.SQN)
	assert.Positive(t, performance.SharpePerTrade)
	assert.Positive(t, performance.SharpeAnnual)
	assert.Positive(t, performance.SortinoAnnual)
	assert.Positive(t, performance.Calmar)
	assert.Positive(t, performance.ReturnPct)
	assert.NotZero(t, performance.BuyAndHold)
	assert.NotZero(t, performance.Beta, "the equity moves with the asset in this fixture")
}

func TestBuildStatistics_Risk(t *testing.T) {
	statistics := buildStatistics(profitableInput(t, 60))
	risk := statistics.Risk

	assert.Positive(t, risk.MaxDrawdown)
	assert.InDelta(t, -0.01, risk.WorstTrade, 1e-9)
	assert.InDelta(t, 0.02, risk.BestTrade, 1e-9)
	assert.Equal(t, 1, risk.LongestLosingStreak, "the fixture never loses twice in a row")
	assert.Negative(t, risk.ValueAtRisk)
	assert.LessOrEqual(t, risk.ExpectedShortfall, risk.ValueAtRisk)
	assert.InDelta(t, 0.005, risk.BreakEvenFee, 1e-9, "half of the average trade")
	assert.Greater(t, risk.ProbabilisticSharpe, 0.9, "a consistent edge over 60 trades")
	assert.Positive(t, risk.MinimumTrackRecord)
}

func TestBuildStatistics_Intervals(t *testing.T) {
	statistics := buildStatistics(profitableInput(t, 60))
	require.Len(t, statistics.Intervals, 3)

	byID := map[string]Interval{}
	for _, interval := range statistics.Intervals {
		byID[interval.ID] = interval
		assert.LessOrEqual(t, interval.Lower, interval.Mean)
		assert.LessOrEqual(t, interval.Mean, interval.Upper)
	}

	assert.Equal(t, "percent", byID[report.IDMeanReturn].Format)
	assert.Equal(t, 0.0, byID[report.IDMeanReturn].Threshold)
	assert.Positive(t, byID[report.IDMeanReturn].Lower, "the interval clears zero")
	assert.Equal(t, 1.0, byID[report.IDProfitFactor].Threshold)
	assert.Equal(t, "ratio", byID[report.IDPayoff].Format)
}

func TestBuildStatistics_Tests(t *testing.T) {
	statistics := buildStatistics(profitableInput(t, 60))

	byID := map[string]HypothesisTest{}
	for _, test := range statistics.Tests {
		byID[test.ID] = test
		assert.GreaterOrEqual(t, test.PValue, 0.0)
		assert.LessOrEqual(t, test.PValue, 1.0)
	}
	require.Len(t, byID, 4)

	assert.Equal(t, report.VerdictPass, byID[report.IDMeanReturnT].Verdict, "a consistent edge is significant")
	assert.True(t, byID[report.IDMeanReturnT].RejectIsGood)
	assert.Equal(t, report.VerdictPass, byID[report.IDMeanReturnBootstrap].Verdict)

	assert.False(t, byID[report.IDIndependenceRuns].RejectIsGood)
	assert.Equal(t, report.VerdictWarn, byID[report.IDIndependenceRuns].Verdict,
		"the fixture repeats a fixed win-win-loss pattern, which is not independent")

	assert.Equal(t, report.VerdictPass, byID[report.IDStabilityHalves].Verdict, "both halves are identical")
}

func TestBuildStatistics_TestsWithoutAnEdge(t *testing.T) {
	base := time.Date(2021, 9, 26, 20, 0, 0, 0, time.UTC)
	input := statisticsInput{Pair: "BTCUSDT", Quote: "USDT"}

	// A coin flip: the same win and loss, alternating.
	for i := 0; i < 40; i++ {
		profit, value := 0.01, 100.0
		if i%2 == 1 {
			profit, value = -0.01, -100.0
		}
		input.Orders = append(input.Orders, tradeAt(int64(i+1), base.Add(time.Duration(i)*time.Hour), profit, value))
	}

	statistics := buildStatistics(input)

	byID := map[string]HypothesisTest{}
	for _, test := range statistics.Tests {
		byID[test.ID] = test
	}

	assert.Equal(t, report.VerdictWarn, byID[report.IDMeanReturnT].Verdict, "a zero average is not an edge")
	assert.Equal(t, report.VerdictWarn, byID[report.IDMeanReturnBootstrap].Verdict)
	assert.Nil(t, statistics.MonteCarlo, "no equity curve, no simulation")
	assert.Positive(t, statistics.Risk.MaxDrawdown, "the drawdown falls back to the compounded trades")
}

func TestBuildStatistics_MonteCarlo(t *testing.T) {
	statistics := buildStatistics(profitableInput(t, 60))
	simulation := statistics.MonteCarlo
	require.NotNil(t, simulation)

	assert.Equal(t, report.Iterations, simulation.Iterations)
	assert.Equal(t, report.RuinThreshold, simulation.RuinThreshold)
	// The curve of the fixture starts after the first trade, at 10200.
	assert.InDelta(t, 6000.0/10200.0, simulation.Observed, 1e-9, "the net profit over the initial equity")
	assert.Equal(t, 1.0, simulation.ProfitProbability, "every resample of this fixture profits")
	assert.Zero(t, simulation.RuinProbability)
	assert.LessOrEqual(t, simulation.FinalReturn.P5, simulation.FinalReturn.Median)
	assert.LessOrEqual(t, simulation.FinalReturn.Median, simulation.FinalReturn.P95)
	assert.GreaterOrEqual(t, simulation.MaxDrawdown.P95, simulation.MaxDrawdown.Median)

	t.Run("is reproducible", func(t *testing.T) {
		again := buildStatistics(profitableInput(t, 60))
		assert.Equal(t, statistics.MonteCarlo, again.MonteCarlo)
		assert.Equal(t, statistics.Tests, again.Tests)
		assert.Equal(t, statistics.Intervals, again.Intervals)
	})
}

func TestBuildStatistics_DistributionAndWindows(t *testing.T) {
	statistics := buildStatistics(profitableInput(t, 60))

	require.Len(t, statistics.Distribution, report.HistogramBins)
	total := 0
	for _, bin := range statistics.Distribution {
		total += bin.Count
	}
	assert.Equal(t, 60, total, "every trade lands in a bin")

	require.Len(t, statistics.Windows, report.StabilityWindows)
	trades := 0
	for i, window := range statistics.Windows {
		assert.Equal(t, i, window.Index)
		assert.Equal(t, 15, window.Trades)
		assert.Positive(t, window.NetProfit)
		assert.LessOrEqual(t, window.Start, window.End)
		trades += window.Trades
	}
	assert.Equal(t, 60, trades)

	t.Run("skips the windows of a short sample", func(t *testing.T) {
		short := buildStatistics(profitableInput(t, 6))
		assert.Empty(t, short.Windows, "five trades per window is the minimum")
	})
}

func TestBuildStatistics_EmptyBacktest(t *testing.T) {
	statistics := buildStatistics(statisticsInput{Pair: "BTCUSDT", Quote: "USDT"})

	assert.Zero(t, statistics.Sample.Trades)
	assert.Empty(t, statistics.Intervals)
	assert.Empty(t, statistics.Windows)
	assert.Empty(t, statistics.Distribution)
	assert.Nil(t, statistics.MonteCarlo)
	assert.Zero(t, statistics.Performance.NetProfit)

	for _, test := range statistics.Tests {
		assert.Equal(t, report.VerdictInconclusive, test.Verdict, "nothing to test without trades")
	}
}

func TestServer_Statistics(t *testing.T) {
	c := newTestChart(t)
	base := time.Date(2021, 9, 26, 20, 0, 0, 0, time.UTC)

	for i := 0; i < 10; i++ {
		c.OnCandle(candleAt("ETHUSDT", base.Add(time.Duration(i)*time.Hour), 3000+float64(i)*10))
	}
	for i := 0; i < 6; i++ {
		profit := 0.02
		if i%3 == 2 {
			profit = -0.01
		}
		c.OnOrder(model.Order{
			ID:          int64(i + 1),
			Pair:        "ETHUSDT",
			Side:        model.SideTypeSell,
			Type:        model.OrderTypeMarket,
			Status:      model.OrderStatusTypeFilled,
			Price:       3000,
			Quantity:    1,
			CreatedAt:   base.Add(time.Duration(i) * time.Hour),
			UpdatedAt:   base.Add(time.Duration(i) * time.Hour),
			Profit:      profit,
			ProfitValue: profit * 1000,
		})
	}

	srv := newTestServer(t, c, nil)

	resp, body := get(t, srv.URL+"/api/ethusdt/statistics")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var statistics Statistics
	require.NoError(t, json.Unmarshal([]byte(body), &statistics))
	assert.Equal(t, "ETHUSDT", statistics.Pair)
	assert.Equal(t, "USDT", statistics.Quote)
	assert.Equal(t, 6, statistics.Sample.Trades)
	assert.Len(t, statistics.Intervals, 3)
	assert.Len(t, statistics.Tests, 4)

	resp, _ = get(t, srv.URL+"/api/unknown/statistics")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
