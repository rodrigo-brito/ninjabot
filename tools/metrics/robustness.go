package metrics

import (
	"math"
	"sort"

	"gonum.org/v1/gonum/stat"
	"gonum.org/v1/gonum/stat/distuv"
)

// This file holds the statistics used to judge how much of a backtest result
// is skill and how much is luck. Every function is pure and takes a plain
// sample, so the same code serves the terminal summary and the web dashboard.
//
// Two samples are used throughout:
//
//   - trade returns: one value per closed trade, as a fraction of the entry
//     price (0.021 = +2.1%). The sample size is the number of trades.
//   - bar returns: one value per candle, taken from the equity curve. They are
//     evenly spaced in time, which is what makes annualization meaningful.

// Test is the outcome of a hypothesis test.
type Test struct {
	// Statistic is the test statistic (t, z, ...).
	Statistic float64
	// PValue is the probability of observing a statistic at least as extreme
	// under the null hypothesis.
	PValue float64
}

// Significant reports whether the null hypothesis is rejected at the given
// significance level (e.g. 0.05).
func (t Test) Significant(level float64) bool {
	return t.PValue <= level
}

// StdDev returns the sample standard deviation. It is zero for samples with
// less than two values.
func StdDev(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	sd := stat.StdDev(values, nil)
	if math.IsNaN(sd) {
		return 0
	}
	return sd
}

// Sharpe is the mean of the sample divided by its standard deviation, with a
// zero risk-free rate. It is expressed in the period of the sample: feed it
// bar returns and Annualize the result, or read it as a per-trade ratio.
func Sharpe(returns []float64) float64 {
	sd := StdDev(returns)
	if sd == 0 {
		return 0
	}
	return Mean(returns) / sd
}

// Sortino replaces the standard deviation of Sharpe by the downside deviation,
// so that upside volatility is not penalized.
func Sortino(returns []float64) float64 {
	if len(returns) < 2 {
		return 0
	}

	var squares float64
	for _, value := range returns {
		if value < 0 {
			squares += value * value
		}
	}

	downside := math.Sqrt(squares / float64(len(returns)))
	if downside == 0 {
		return 0
	}

	return Mean(returns) / downside
}

// Annualize scales a Sharpe-like ratio measured over periods of the sample to
// a yearly figure, by the square root of the number of periods in a year.
func Annualize(ratio float64, periodsPerYear float64) float64 {
	if periodsPerYear <= 0 {
		return 0
	}
	return ratio * math.Sqrt(periodsPerYear)
}

// Skew is the sample skewness: negative values mean a long left tail, the
// shape of a strategy that wins often and loses big.
func Skew(values []float64) float64 {
	if len(values) < 3 {
		return 0
	}
	skew := stat.Skew(values, nil)
	if math.IsNaN(skew) || math.IsInf(skew, 0) {
		return 0
	}
	return skew
}

// ExcessKurtosis is the sample kurtosis minus 3, the kurtosis of a normal
// distribution. Positive values mean fatter tails than a normal.
func ExcessKurtosis(values []float64) float64 {
	if len(values) < 4 {
		return 0
	}
	kurtosis := stat.ExKurtosis(values, nil)
	if math.IsNaN(kurtosis) || math.IsInf(kurtosis, 0) {
		return 0
	}
	return kurtosis
}

// ProbabilisticSharpe is the probability that the true Sharpe ratio of the
// strategy is above benchmark, given the length, skewness and kurtosis of the
// sample (Bailey and Lopez de Prado, 2012). A backtest with few trades, a
// negative skew or fat tails needs a much higher observed Sharpe to reach the
// same confidence.
//
// It answers "how sure am I that this edge is not noise?" on a 0..1 scale.
func ProbabilisticSharpe(returns []float64, benchmark float64) float64 {
	n := float64(len(returns))
	if n < 3 {
		return 0
	}

	sharpe := Sharpe(returns)
	variance := sharpeVariance(sharpe, Skew(returns), ExcessKurtosis(returns))
	if variance <= 0 {
		return 0
	}

	z := (sharpe - benchmark) * math.Sqrt(n-1) / math.Sqrt(variance)

	return distuv.UnitNormal.CDF(z)
}

// MinimumTrackRecord is the number of observations needed for the Sharpe ratio
// of the sample to be above benchmark with the given confidence. Comparing it
// with the number of trades of the backtest tells whether the sample is long
// enough to support the claim.
//
// It returns 0 when the observed Sharpe is already below the benchmark, in
// which case no amount of extra trades would help.
func MinimumTrackRecord(returns []float64, benchmark, confidence float64) float64 {
	if len(returns) < 3 || confidence <= 0 || confidence >= 1 {
		return 0
	}

	sharpe := Sharpe(returns)
	if sharpe <= benchmark {
		return 0
	}

	variance := sharpeVariance(sharpe, Skew(returns), ExcessKurtosis(returns))
	if variance <= 0 {
		return 0
	}

	z := distuv.UnitNormal.Quantile(confidence) / (sharpe - benchmark)

	return 1 + variance*z*z
}

// sharpeVariance is the variance of the Sharpe estimator under non-normal
// returns, the shared core of ProbabilisticSharpe and MinimumTrackRecord.
func sharpeVariance(sharpe, skew, excessKurtosis float64) float64 {
	kurtosis := excessKurtosis + 3
	return 1 - skew*sharpe + (kurtosis-1)/4*sharpe*sharpe
}

// MeanTTest tests the null hypothesis that the average return is zero against
// the one-sided alternative that it is positive. A p-value below 0.05 means
// the average trade is hard to explain by chance alone -- assuming the trades
// are independent, which RunsTest and Autocorrelation check.
func MeanTTest(returns []float64) Test {
	n := float64(len(returns))
	if n < 2 {
		return Test{PValue: 1}
	}

	sd := StdDev(returns)
	if sd == 0 {
		return Test{PValue: 1}
	}

	t := Mean(returns) / (sd / math.Sqrt(n))
	dist := distuv.StudentsT{Mu: 0, Sigma: 1, Nu: n - 1}

	return Test{Statistic: t, PValue: 1 - dist.CDF(t)}
}

// RunsTest is the Wald-Wolfowitz test for independence: it counts the runs of
// consecutive wins and losses and compares them with what a coin flip would
// produce. A significant p-value means wins and losses cluster, so the trades
// are not independent and every confidence interval built on them (including
// the bootstrap ones) is too narrow.
func RunsTest(returns []float64) Test {
	var wins, losses float64
	runs := 0.0
	previous := 0

	for _, value := range returns {
		sign := 1
		if value < 0 {
			sign = -1
		}

		if sign > 0 {
			wins++
		} else {
			losses++
		}

		if sign != previous {
			runs++
			previous = sign
		}
	}

	n := wins + losses
	if wins == 0 || losses == 0 || n < 3 {
		return Test{PValue: 1}
	}

	expected := 2*wins*losses/n + 1
	variance := 2 * wins * losses * (2*wins*losses - n) / (n * n * (n - 1))
	if variance <= 0 {
		return Test{PValue: 1}
	}

	z := (runs - expected) / math.Sqrt(variance)
	p := 2 * (1 - distuv.UnitNormal.CDF(math.Abs(z)))

	return Test{Statistic: z, PValue: p}
}

// Autocorrelation is the lag-1 correlation of the sample. Values far from zero
// mean a trade carries information about the next one, which breaks the
// independence assumption of the confidence intervals.
func Autocorrelation(values []float64) float64 {
	if len(values) < 3 {
		return 0
	}

	current := values[:len(values)-1]
	next := values[1:]
	if StdDev(current) == 0 || StdDev(next) == 0 {
		return 0
	}

	correlation := stat.Correlation(current, next, nil)
	if math.IsNaN(correlation) {
		return 0
	}

	return correlation
}

// ValueAtRisk is the loss that the given fraction of trades does not exceed
// (e.g. confidence 0.95 returns the 5th percentile of the sample).
func ValueAtRisk(returns []float64, confidence float64) float64 {
	if len(returns) == 0 {
		return 0
	}

	sorted := append([]float64(nil), returns...)
	sort.Float64s(sorted)

	return stat.Quantile(1-confidence, stat.LinInterp, sorted, nil)
}

// ExpectedShortfall is the average of the losses beyond the value at risk, the
// answer to "when it goes wrong, how bad is it on average?".
func ExpectedShortfall(returns []float64, confidence float64) float64 {
	if len(returns) == 0 {
		return 0
	}

	limit := ValueAtRisk(returns, confidence)
	tail := make([]float64, 0, len(returns))
	for _, value := range returns {
		if value <= limit {
			tail = append(tail, value)
		}
	}

	if len(tail) == 0 {
		return limit
	}

	return Mean(tail)
}

// MaxDrawdown is the largest peak-to-trough decline of an equity curve, as a
// positive fraction of the peak.
func MaxDrawdown(equity []float64) float64 {
	peak := math.Inf(-1)
	worst := 0.0

	for _, value := range equity {
		if value > peak {
			peak = value
		}
		if peak > 0 {
			if drawdown := (peak - value) / peak; drawdown > worst {
				worst = drawdown
			}
		}
	}

	return worst
}

// BreakEvenFee is the round-trip cost, as a fraction of the traded volume,
// that would wipe out the average trade. A strategy whose break-even fee is
// below the fee of the exchange only works on paper.
//
// Each trade pays the fee twice, on the entry and on the exit fill.
func BreakEvenFee(returns []float64) float64 {
	average := Mean(returns)
	if average <= 0 {
		return 0
	}
	return average / 2
}

// Beta is the sensitivity of the strategy to the asset it trades: 1 means it
// moves with the market, 0 that it is uncorrelated. Both samples must be
// aligned in time and have the same length.
func Beta(strategy, benchmark []float64) float64 {
	if len(strategy) != len(benchmark) || len(strategy) < 2 {
		return 0
	}

	variance := stat.Variance(benchmark, nil)
	if variance == 0 || math.IsNaN(variance) {
		return 0
	}

	covariance := stat.Covariance(strategy, benchmark, nil)
	if math.IsNaN(covariance) {
		return 0
	}

	return covariance / variance
}

// Alpha is the average return the strategy adds on top of what its exposure to
// the asset already explains, per period of the sample.
func Alpha(strategy, benchmark []float64) float64 {
	if len(strategy) != len(benchmark) || len(strategy) < 2 {
		return 0
	}
	return Mean(strategy) - Beta(strategy, benchmark)*Mean(benchmark)
}

// Bin is one bar of a histogram.
type Bin struct {
	Start float64
	End   float64
	Count int
}

// Histogram splits values into evenly spaced bins, the graphical counterpart
// of the return distribution printed by the terminal summary.
func Histogram(values []float64, bins int) []Bin {
	if len(values) == 0 || bins <= 0 {
		return nil
	}

	min, max := values[0], values[0]
	for _, value := range values {
		if value < min {
			min = value
		}
		if value > max {
			max = value
		}
	}

	// A single distinct value would give a zero-width histogram: pad it so the
	// value lands in the middle of a bin.
	if max == min {
		padding := math.Max(math.Abs(max), 1) * 0.01
		min, max = min-padding, max+padding
	}

	width := (max - min) / float64(bins)
	histogram := make([]Bin, bins)
	for i := range histogram {
		histogram[i] = Bin{Start: min + float64(i)*width, End: min + float64(i+1)*width}
	}

	for _, value := range values {
		index := int((value - min) / width)
		if index >= bins {
			index = bins - 1
		}
		if index < 0 {
			index = 0
		}
		histogram[index].Count++
	}

	return histogram
}
