package report

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tradeAt builds a closed trade.
func tradeAt(id int64, at time.Time, profit, profitValue float64) Trade {
	return Trade{ID: id, Time: at.Unix(), Profit: profit, ProfitValue: profitValue}
}

// profitableInput is a backtest with a small but consistent edge: two winners
// for every loser, over an hourly series.
func profitableInput(t *testing.T, trades int) Input {
	t.Helper()

	base := time.Date(2021, 9, 26, 20, 0, 0, 0, time.UTC)
	input := Input{Pair: "BTCUSDT", Quote: "USDT"}

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

		input.Trades = append(input.Trades, tradeAt(int64(i+1), at, profit, value))
		input.Equity = append(input.Equity, Point{Time: at.Unix(), Value: equity})
		input.Candles = append(input.Candles, Candle{Time: at.Unix(), Close: price, Open: price, High: price, Low: price})
	}

	return input
}

func TestBuild_Sample(t *testing.T) {
	statistics := Build(profitableInput(t, 60))

	assert.Equal(t, "BTCUSDT", statistics.Pair)
	assert.Equal(t, "USDT", statistics.Quote)
	assert.Equal(t, 60, statistics.Sample.Trades)
	assert.Equal(t, 40, statistics.Sample.Wins)
	assert.Equal(t, 20, statistics.Sample.Losses)
	assert.Equal(t, 60, statistics.Sample.Bars)
	assert.Equal(t, Iterations, statistics.Sample.Iterations)
	assert.InDelta(t, 24*365.0, statistics.Sample.BarsPerYear, 1, "hourly candles")
}

func TestBuild_Performance(t *testing.T) {
	statistics := Build(profitableInput(t, 60))
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

func TestBuild_Risk(t *testing.T) {
	statistics := Build(profitableInput(t, 60))
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

func TestBuild_Intervals(t *testing.T) {
	statistics := Build(profitableInput(t, 60))
	require.Len(t, statistics.Intervals, 3)

	byID := map[string]Interval{}
	for _, interval := range statistics.Intervals {
		byID[interval.ID] = interval
		assert.LessOrEqual(t, interval.Lower, interval.Mean)
		assert.LessOrEqual(t, interval.Mean, interval.Upper)
	}

	assert.Equal(t, "percent", byID[IDMeanReturn].Format)
	assert.Equal(t, 0.0, byID[IDMeanReturn].Threshold)
	assert.Positive(t, byID[IDMeanReturn].Lower, "the interval clears zero")
	assert.Equal(t, 1.0, byID[IDProfitFactor].Threshold)
	assert.Equal(t, "ratio", byID[IDPayoff].Format)
}

func TestBuild_Tests(t *testing.T) {
	statistics := Build(profitableInput(t, 60))

	byID := map[string]HypothesisTest{}
	for _, test := range statistics.Tests {
		byID[test.ID] = test
		assert.GreaterOrEqual(t, test.PValue, 0.0)
		assert.LessOrEqual(t, test.PValue, 1.0)
	}
	require.Len(t, byID, 4)

	assert.Equal(t, VerdictPass, byID[IDMeanReturnT].Verdict, "a consistent edge is significant")
	assert.True(t, byID[IDMeanReturnT].RejectIsGood)
	assert.Equal(t, VerdictPass, byID[IDMeanReturnBootstrap].Verdict)

	assert.False(t, byID[IDIndependenceRuns].RejectIsGood)
	assert.Equal(t, VerdictWarn, byID[IDIndependenceRuns].Verdict,
		"the fixture repeats a fixed win-win-loss pattern, which is not independent")

	assert.Equal(t, VerdictPass, byID[IDStabilityHalves].Verdict, "both halves are identical")
}

func TestBuild_TestsWithoutAnEdge(t *testing.T) {
	base := time.Date(2021, 9, 26, 20, 0, 0, 0, time.UTC)
	input := Input{Pair: "BTCUSDT", Quote: "USDT"}

	// A coin flip: the same win and loss, alternating.
	for i := 0; i < 40; i++ {
		profit, value := 0.01, 100.0
		if i%2 == 1 {
			profit, value = -0.01, -100.0
		}
		input.Trades = append(input.Trades, tradeAt(int64(i+1), base.Add(time.Duration(i)*time.Hour), profit, value))
	}

	statistics := Build(input)

	byID := map[string]HypothesisTest{}
	for _, test := range statistics.Tests {
		byID[test.ID] = test
	}

	assert.Equal(t, VerdictWarn, byID[IDMeanReturnT].Verdict, "a zero average is not an edge")
	assert.Equal(t, VerdictWarn, byID[IDMeanReturnBootstrap].Verdict)
	assert.Nil(t, statistics.MonteCarlo, "no equity curve, no simulation")
	assert.Positive(t, statistics.Risk.MaxDrawdown, "the drawdown falls back to the compounded trades")
}

func TestBuild_MonteCarlo(t *testing.T) {
	statistics := Build(profitableInput(t, 60))
	simulation := statistics.MonteCarlo
	require.NotNil(t, simulation)

	assert.Equal(t, Iterations, simulation.Iterations)
	assert.Equal(t, RuinThreshold, simulation.RuinThreshold)
	// The curve of the fixture starts after the first trade, at 10200.
	assert.InDelta(t, 6000.0/10200.0, simulation.Observed, 1e-9, "the net profit over the initial equity")
	assert.Equal(t, 1.0, simulation.ProfitProbability, "every resample of this fixture profits")
	assert.Zero(t, simulation.RuinProbability)
	assert.LessOrEqual(t, simulation.FinalReturn.P5, simulation.FinalReturn.Median)
	assert.LessOrEqual(t, simulation.FinalReturn.Median, simulation.FinalReturn.P95)
	assert.GreaterOrEqual(t, simulation.MaxDrawdown.P95, simulation.MaxDrawdown.Median)

	t.Run("is reproducible", func(t *testing.T) {
		again := Build(profitableInput(t, 60))
		assert.Equal(t, statistics.MonteCarlo, again.MonteCarlo)
		assert.Equal(t, statistics.Tests, again.Tests)
		assert.Equal(t, statistics.Intervals, again.Intervals)
	})
}

func TestBuild_DistributionAndWindows(t *testing.T) {
	statistics := Build(profitableInput(t, 60))

	require.Len(t, statistics.Distribution, HistogramBins)
	total := 0
	for _, bin := range statistics.Distribution {
		total += bin.Count
	}
	assert.Equal(t, 60, total, "every trade lands in a bin")

	require.Len(t, statistics.Windows, StabilityWindows)
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
		short := Build(profitableInput(t, 6))
		assert.Empty(t, short.Windows, "five trades per window is the minimum")
	})
}

func TestBuild_EmptyBacktest(t *testing.T) {
	statistics := Build(Input{Pair: "BTCUSDT", Quote: "USDT"})

	assert.Zero(t, statistics.Sample.Trades)
	assert.Empty(t, statistics.Intervals)
	assert.Empty(t, statistics.Windows)
	assert.Empty(t, statistics.Distribution)
	assert.Nil(t, statistics.MonteCarlo)
	assert.Zero(t, statistics.Performance.NetProfit)

	for _, test := range statistics.Tests {
		assert.Equal(t, VerdictInconclusive, test.Verdict, "nothing to test without trades")
	}
}

func TestBuild_Serializable(t *testing.T) {
	for _, input := range []Input{
		{Pair: "BTCUSDT", Quote: "USDT"},
		{Pair: "BTCUSDT", Quote: "USDT", Trades: []Trade{{ID: 1, Time: 1, Profit: 0.1, ProfitValue: 10}}},
		profitableInput(t, 60),
	} {
		_, err := json.Marshal(Build(input))
		require.NoError(t, err)
	}
}

func TestBuild_BarsPerYearFromEquity(t *testing.T) {
	input := profitableInput(t, 60)
	input.Candles = nil
	statistics := Build(input)
	assert.InDelta(t, 365*24, statistics.Sample.BarsPerYear, 1e-9, "hourly equity points")
	assert.NotZero(t, statistics.Performance.SharpeAnnual)
}
