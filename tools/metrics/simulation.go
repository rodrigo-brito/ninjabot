package metrics

import (
	"math"
	"math/rand"
	"sort"

	"gonum.org/v1/gonum/stat"
)

// The resampling tests of this file answer the question a single equity curve
// cannot: how much of the result depends on the particular sequence of trades
// that happened to be drawn? Every function takes an explicit seed so a report
// is reproducible -- reloading the dashboard must not move the numbers.

// DefaultSeed keeps the simulations deterministic across runs and processes.
const DefaultSeed int64 = 1

// Distribution summarizes a simulated sample by its percentiles.
type Distribution struct {
	Mean   float64
	Min    float64
	P5     float64
	P25    float64
	Median float64
	P75    float64
	P95    float64
	Max    float64
}

func newDistribution(values []float64) Distribution {
	if len(values) == 0 {
		return Distribution{}
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	quantile := func(p float64) float64 {
		return stat.Quantile(p, stat.LinInterp, sorted, nil)
	}

	return Distribution{
		Mean:   Mean(sorted),
		Min:    sorted[0],
		P5:     quantile(0.05),
		P25:    quantile(0.25),
		Median: quantile(0.5),
		P75:    quantile(0.75),
		P95:    quantile(0.95),
		Max:    sorted[len(sorted)-1],
	}
}

// MonteCarloResult is the outcome of resampling the trade sequence.
type MonteCarloResult struct {
	Iterations int
	// FinalReturn is the distribution of the final return, as a fraction of
	// the initial equity.
	FinalReturn Distribution
	// MaxDrawdown is the distribution of the largest peak-to-trough decline,
	// as a positive fraction.
	MaxDrawdown Distribution
	// ProfitProbability is the share of simulations that ended in profit.
	ProfitProbability float64
	// RuinProbability is the share of simulations whose drawdown went past
	// RuinThreshold.
	RuinProbability float64
	RuinThreshold   float64
}

// MonteCarlo draws new trade sequences from the observed profit and loss and
// replays them over the initial equity. Resampling with replacement keeps the
// distribution of the trades and destroys their order, which separates the
// edge of the strategy from the luck of the sequence: a strategy whose 5th
// percentile is still profitable does not depend on a lucky ordering, and the
// drawdown distribution shows how deep a run of bad trades can go.
//
// The profit and loss is applied additively, so the simulation keeps the
// position sizing of the backtest instead of assuming a fixed fraction of the
// equity is risked on every trade.
func MonteCarlo(pnl []float64, initialEquity float64, iterations int,
	ruinThreshold float64, seed int64) MonteCarloResult {

	result := MonteCarloResult{Iterations: iterations, RuinThreshold: ruinThreshold}
	if len(pnl) == 0 || iterations <= 0 || initialEquity <= 0 {
		return result
	}

	random := rand.New(rand.NewSource(seed)) //nolint:gosec // reproducibility, not cryptography

	returns := make([]float64, iterations)
	drawdowns := make([]float64, iterations)
	equity := make([]float64, len(pnl)+1)
	profitable, ruined := 0, 0

	for i := 0; i < iterations; i++ {
		equity[0] = initialEquity
		for j := range pnl {
			equity[j+1] = equity[j] + pnl[random.Intn(len(pnl))]
		}

		final := equity[len(equity)-1]
		returns[i] = (final - initialEquity) / initialEquity
		drawdowns[i] = MaxDrawdown(equity)

		if final > initialEquity {
			profitable++
		}
		if drawdowns[i] >= ruinThreshold {
			ruined++
		}
	}

	result.FinalReturn = newDistribution(returns)
	result.MaxDrawdown = newDistribution(drawdowns)
	result.ProfitProbability = float64(profitable) / float64(iterations)
	result.RuinProbability = float64(ruined) / float64(iterations)

	return result
}

// BootstrapPValue tests the null hypothesis that the average value of the
// sample is zero, without assuming any shape for its distribution: the sample
// is centered on zero and resampled, and the p-value is the share of
// resamples whose average reaches the observed one.
//
// It is the distribution-free counterpart of MeanTTest, and the two agreeing
// is a good sign that the result does not hang on the normality assumption.
func BootstrapPValue(values []float64, iterations int, seed int64) float64 {
	if len(values) < 2 || iterations <= 0 {
		return 1
	}

	observed := Mean(values)
	if observed <= 0 {
		return 1
	}

	centered := make([]float64, len(values))
	for i, value := range values {
		centered[i] = value - observed
	}

	random := rand.New(rand.NewSource(seed)) //nolint:gosec // reproducibility, not cryptography

	atLeastAsExtreme := 0
	for i := 0; i < iterations; i++ {
		sum := 0.0
		for range centered {
			sum += centered[random.Intn(len(centered))]
		}
		if sum/float64(len(centered)) >= observed {
			atLeastAsExtreme++
		}
	}

	// The +1 correction keeps the p-value away from an impossible zero.
	return float64(atLeastAsExtreme+1) / float64(iterations+1)
}

// Window is the performance of one slice of the trade history, used to check
// whether the edge holds over time or comes from a single lucky stretch.
type Window struct {
	Index      int
	Trades     int
	MeanReturn float64
	NetProfit  float64
	WinRate    float64
}

// Windows splits the trade history into count consecutive slices of equal
// size. A strategy that only makes money in one window is fitted to it.
func Windows(returns, pnl []float64, count int) []Window {
	if len(returns) == 0 || count <= 0 || len(returns) < count {
		return nil
	}

	windows := make([]Window, 0, count)
	size := len(returns) / count

	for i := 0; i < count; i++ {
		start := i * size
		end := start + size
		if i == count-1 {
			end = len(returns)
		}

		slice := returns[start:end]
		wins := 0
		for _, value := range slice {
			if value > 0 {
				wins++
			}
		}

		profit := 0.0
		if len(pnl) == len(returns) {
			for _, value := range pnl[start:end] {
				profit += value
			}
		}

		windows = append(windows, Window{
			Index:      i,
			Trades:     len(slice),
			MeanReturn: Mean(slice),
			NetProfit:  profit,
			WinRate:    float64(wins) / float64(len(slice)),
		})
	}

	return windows
}

// StabilityTest compares the first half of the trades with the second half
// through a permutation test: the trades are shuffled between the two halves
// many times, and the p-value is the share of shuffles whose gap is as wide as
// the observed one. A significant p-value means the performance decayed (or
// improved) beyond what a random split explains, the signature of a strategy
// tuned to one period.
//
// Statistic carries the observed difference between the average return of the
// second half and of the first one.
func StabilityTest(returns []float64, iterations int, seed int64) Test {
	if len(returns) < 4 || iterations <= 0 {
		return Test{PValue: 1}
	}

	half := len(returns) / 2
	observed := Mean(returns[half:]) - Mean(returns[:half])

	shuffled := append([]float64(nil), returns...)
	random := rand.New(rand.NewSource(seed)) //nolint:gosec // reproducibility, not cryptography

	atLeastAsExtreme := 0
	for i := 0; i < iterations; i++ {
		random.Shuffle(len(shuffled), func(a, b int) {
			shuffled[a], shuffled[b] = shuffled[b], shuffled[a]
		})

		difference := Mean(shuffled[half:]) - Mean(shuffled[:half])
		if math.Abs(difference) >= math.Abs(observed) {
			atLeastAsExtreme++
		}
	}

	return Test{
		Statistic: observed,
		PValue:    float64(atLeastAsExtreme+1) / float64(iterations+1),
	}
}
