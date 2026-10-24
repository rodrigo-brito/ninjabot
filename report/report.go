// Package report computes the robustness statistics of a trading record: the
// performance and risk numbers, bootstrap intervals, hypothesis tests, a Monte
// Carlo simulation and the stability over time. It is pure: the same input
// always yields the same report, and it is safe for concurrent use.
package report

import (
	"math"
	"reflect"
	"sort"

	"github.com/rodrigo-brito/ninjabot/tools/metrics"
)

// Point is a value at a Unix time, in seconds.
type Point struct {
	Time  int64   `json:"time"`
	Value float64 `json:"value"`
}

// Candle is an OHLCV bar at a Unix time, in seconds.
type Candle struct {
	Time   int64   `json:"time"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
}

// Trade is a closed round trip: its return as a fraction of the entry value
// and its profit in quote currency, net of fees, at the Unix time it closed.
type Trade struct {
	ID          int64   `json:"id"`
	Time        int64   `json:"time"`
	Profit      float64 `json:"profit"`
	ProfitValue float64 `json:"profit_value"`
}

// Input is everything a report is computed from.
type Input struct {
	Pair  string
	Quote string
	// Trades in the order they closed.
	Trades []Trade
	// Equity is the account value over time, one point per bar. It is empty
	// when the record has no wallet behind it.
	Equity []Point
	// Candles of the traded asset, for buy and hold, beta and alpha. They also
	// tell the bar frequency that annualizes the ratios.
	Candles []Candle
	// Seed of the resampling tests; metrics.DefaultSeed when zero.
	Seed int64
}

const (
	// Iterations is the resampling budget of every bootstrap, Monte
	// Carlo and permutation test of the report. It is high enough for a stable
	// second decimal on a p-value and still runs in a few milliseconds.
	Iterations = 5000

	// Confidence is the confidence level of the intervals and of the
	// tail measures (value at risk, expected shortfall).
	Confidence = 0.95

	// SignificanceLevel is where a p-value flips the verdict of a test.
	SignificanceLevel = 0.05

	// RuinThreshold is the drawdown a simulated run has to reach to be counted
	// as ruin. Most strategies are abandoned well before losing a fifth of the
	// account.
	RuinThreshold = 0.2

	// HistogramBins mirrors the resolution of the terminal summary histogram.
	HistogramBins = 21

	// StabilityWindows is the number of slices the trade history is cut into
	// to expose an edge that lives in a single stretch of the period.
	StabilityWindows = 4

	// MinTradesForWindows keeps the per-window numbers from being noise.
	MinTradesForWindows = StabilityWindows * 5

	yearSeconds = 365 * 24 * 60 * 60
)

// Verdicts of a HypothesisTest.
const (
	VerdictPass         = "pass"
	VerdictWarn         = "warn"
	VerdictInconclusive = "inconclusive"
)

// IDs of the intervals and tests. The dashboard maps them to the text it
// shows, so the wording of the report can change without touching the API.
const (
	IDMeanReturn   = "mean_return"
	IDPayoff       = "payoff"
	IDProfitFactor = "profit_factor"

	IDMeanReturnT         = "mean_return_t"
	IDMeanReturnBootstrap = "mean_return_bootstrap"
	IDIndependenceRuns    = "independence_runs"
	IDStabilityHalves     = "stability_halves"
)

// Build computes the report of the input.
func Build(in Input) Statistics {
	seed := in.Seed
	if seed == 0 {
		seed = metrics.DefaultSeed
	}

	trades := in.Trades

	returns := make([]float64, len(trades))
	pnl := make([]float64, len(trades))
	for i, trade := range trades {
		returns[i] = trade.Profit
		pnl[i] = trade.ProfitValue
	}

	wins, losses := countOutcomes(returns)
	barsPerYear := barsPerYear(in.Candles)
	if barsPerYear == 0 {
		// without candles (a portfolio of several assets), the equity curve
		// tells the bar frequency
		barsPerYear = pointsPerYear(in.Equity)
	}

	statistics := Statistics{
		Pair:  in.Pair,
		Quote: in.Quote,
		Sample: Sample{
			Trades:      len(trades),
			Wins:        wins,
			Losses:      losses,
			Bars:        len(in.Equity),
			BarsPerYear: barsPerYear,
			Iterations:  Iterations,
			Seed:        seed,
		},
		Distribution: histogram(returns),
		Windows:      windows(trades, returns, pnl),
	}

	statistics.Performance = performance(returns, pnl, in.Equity, in.Candles, barsPerYear)
	statistics.Risk = risk(returns, in.Equity)
	statistics.Intervals = intervals(returns, seed)
	statistics.Tests = tests(returns, seed)
	statistics.MonteCarlo = monteCarlo(pnl, in.Equity, seed)

	finite(reflect.ValueOf(&statistics).Elem())
	return statistics
}

// finite replaces NaN and infinite numbers by zero, recursively, so a report
// is always serializable as JSON. Metrics undefined for a sample (the skew of
// two trades, the ratio of a flat curve) read as zero.
func finite(v reflect.Value) {
	switch v.Kind() {
	case reflect.Float64:
		if f := v.Float(); math.IsNaN(f) || math.IsInf(f, 0) {
			v.SetFloat(0)
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			finite(v.Field(i))
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			finite(v.Index(i))
		}
	case reflect.Pointer:
		if !v.IsNil() {
			finite(v.Elem())
		}
	}
}

func countOutcomes(returns []float64) (wins, losses int) {
	for _, value := range returns {
		if value > 0 {
			wins++
		} else {
			losses++
		}
	}
	return wins, losses
}

// barsPerYear infers the sampling frequency of the equity curve from the
// candles, which is what makes an annualized ratio meaningful. The median
// spacing is used so that a gap in the data does not distort it.
func barsPerYear(candles []Candle) float64 {
	if len(candles) < 3 {
		return 0
	}

	gaps := make([]float64, 0, len(candles)-1)
	for i := 1; i < len(candles); i++ {
		if gap := candles[i].Time - candles[i-1].Time; gap > 0 {
			gaps = append(gaps, float64(gap))
		}
	}

	if len(gaps) == 0 {
		return 0
	}

	sort.Float64s(gaps)
	median := gaps[len(gaps)/2]
	if median <= 0 {
		return 0
	}

	return yearSeconds / median
}

// pointsPerYear is barsPerYear measured on the spacing of a series of points.
func pointsPerYear(points []Point) float64 {
	candles := make([]Candle, len(points))
	for i, point := range points {
		candles[i] = Candle{Time: point.Time}
	}
	return barsPerYear(candles)
}

func performance(returns, pnl []float64, equity []Point, candles []Candle, barsPerYear float64) PerformanceStats {
	net := 0.0
	for _, value := range pnl {
		net += value
	}

	wins, _ := countOutcomes(returns)
	performance := PerformanceStats{
		NetProfit:      net,
		BuyAndHold:     buyAndHold(candles),
		MeanReturn:     metrics.Mean(returns),
		MedianReturn:   median(returns),
		Payoff:         metrics.Payoff(returns),
		ProfitFactor:   metrics.ProfitFactor(returns),
		SQN:            sqn(pnl),
		SharpePerTrade: metrics.Sharpe(returns),
	}

	if len(returns) > 0 {
		performance.WinRate = float64(wins) / float64(len(returns))
		performance.Expectancy = net / float64(len(returns))
	}

	if len(equity) > 0 && equity[0].Value > 0 {
		performance.ReturnPct = (equity[len(equity)-1].Value - equity[0].Value) / equity[0].Value
	}

	// The annualized ratios need evenly spaced samples, which only the equity
	// curve provides -- the trades are irregular by nature.
	barReturns := pointReturns(equity)
	if barsPerYear > 0 && len(barReturns) > 1 {
		performance.SharpeAnnual = metrics.Annualize(metrics.Sharpe(barReturns), barsPerYear)
		performance.SortinoAnnual = metrics.Annualize(metrics.Sortino(barReturns), barsPerYear)

		strategy, benchmark := alignReturns(equity, candles)
		if len(strategy) > 1 {
			performance.Beta = metrics.Beta(strategy, benchmark)
			performance.Alpha = metrics.Alpha(strategy, benchmark) * barsPerYear
		}
	}

	if drawdown := equityDrawdown(equity); drawdown > 0 && barsPerYear > 0 && len(barReturns) > 0 {
		years := float64(len(barReturns)) / barsPerYear
		if years > 0 {
			// Calmar compares the yearly return with the worst decline the
			// strategy had to sit through to get it.
			performance.Calmar = (performance.ReturnPct / years) / drawdown
		}
	}

	return performance
}

func risk(returns []float64, equity []Point) RiskStats {
	risk := RiskStats{
		MaxDrawdown:         equityDrawdown(equity),
		ValueAtRisk:         metrics.ValueAtRisk(returns, Confidence),
		ExpectedShortfall:   metrics.ExpectedShortfall(returns, Confidence),
		LongestLosingStreak: longestLosingStreak(returns),
		Skew:                metrics.Skew(returns),
		ExcessKurtosis:      metrics.ExcessKurtosis(returns),
		Autocorrelation:     metrics.Autocorrelation(returns),
		BreakEvenFee:        metrics.BreakEvenFee(returns),
		ProbabilisticSharpe: metrics.ProbabilisticSharpe(returns, 0),
		MinimumTrackRecord:  metrics.MinimumTrackRecord(returns, 0, Confidence),
	}

	if len(returns) > 0 {
		risk.WorstTrade, risk.BestTrade = returns[0], returns[0]
		for _, value := range returns {
			risk.WorstTrade = math.Min(risk.WorstTrade, value)
			risk.BestTrade = math.Max(risk.BestTrade, value)
		}
	}

	// Without a paper wallet there is no equity curve, so the drawdown falls
	// back to the one of the compounded trade sequence.
	if risk.MaxDrawdown == 0 && len(equity) == 0 {
		risk.MaxDrawdown = metrics.MaxDrawdown(compound(returns))
	}

	return risk
}

func intervals(returns []float64, seed int64) []Interval {
	if len(returns) < 2 {
		return []Interval{}
	}

	measures := []struct {
		id        string
		measure   func([]float64) float64
		threshold float64
		format    string
	}{
		{IDMeanReturn, metrics.Mean, 0, "percent"},
		{IDPayoff, metrics.Payoff, 1, "ratio"},
		{IDProfitFactor, metrics.ProfitFactor, 1, "ratio"},
	}

	intervals := make([]Interval, 0, len(measures))
	for _, m := range measures {
		interval := metrics.BootstrapWithSeed(returns, m.measure, Iterations, Confidence, seed)
		intervals = append(intervals, Interval{
			ID:        m.id,
			Mean:      interval.Mean,
			Lower:     interval.Lower,
			Upper:     interval.Upper,
			StdDev:    interval.StdDev,
			Threshold: m.threshold,
			Format:    m.format,
		})
	}

	return intervals
}

func tests(returns []float64, seed int64) []HypothesisTest {
	tTest := metrics.MeanTTest(returns)
	runs := metrics.RunsTest(returns)
	stability := metrics.StabilityTest(returns, Iterations, seed)
	bootstrap := metrics.BootstrapPValue(returns, Iterations, seed)

	return []HypothesisTest{
		newTest(IDMeanReturnT, "t", tTest.Statistic, tTest.PValue, true, len(returns) > 1),
		newTest(IDMeanReturnBootstrap, "p", metrics.Mean(returns), bootstrap, true, len(returns) > 1),
		newTest(IDIndependenceRuns, "z", runs.Statistic, runs.PValue, false, runs.Statistic != 0),
		newTest(IDStabilityHalves, "Δ", stability.Statistic, stability.PValue, false, len(returns) >= 4),
	}
}

func newTest(id, label string, statistic, pValue float64, rejectIsGood, ran bool) HypothesisTest {
	verdict := verdict(pValue, rejectIsGood, ran)
	// keep the report serializable: a test that could not run carries no
	// evidence, which is a p-value of one
	if math.IsNaN(pValue) || math.IsInf(pValue, 0) {
		pValue = 1
	}
	if math.IsNaN(statistic) || math.IsInf(statistic, 0) {
		statistic = 0
	}

	return HypothesisTest{
		ID:             id,
		Statistic:      statistic,
		StatisticLabel: label,
		PValue:         pValue,
		RejectIsGood:   rejectIsGood,
		Verdict:        verdict,
	}
}

// verdict reads a p-value against what rejecting the null means for the
// strategy: for an edge test rejecting is the good news, for an assumption
// test (independence, stability) rejecting is the warning.
func verdict(pValue float64, rejectIsGood, ran bool) string {
	if !ran || math.IsNaN(pValue) {
		return VerdictInconclusive
	}

	rejected := pValue <= SignificanceLevel
	if rejected == rejectIsGood {
		return VerdictPass
	}

	return VerdictWarn
}

func monteCarlo(pnl []float64, equity []Point, seed int64) *MonteCarlo {
	if len(pnl) == 0 || len(equity) == 0 || equity[0].Value <= 0 {
		return nil
	}

	initial := equity[0].Value
	result := metrics.MonteCarlo(pnl, initial, Iterations, RuinThreshold, seed)

	observed := 0.0
	for _, value := range pnl {
		observed += value
	}

	return &MonteCarlo{
		Iterations:        result.Iterations,
		FinalReturn:       percentiles(result.FinalReturn),
		MaxDrawdown:       percentiles(result.MaxDrawdown),
		ProfitProbability: result.ProfitProbability,
		RuinProbability:   result.RuinProbability,
		RuinThreshold:     result.RuinThreshold,
		Observed:          observed / initial,
	}
}

func percentiles(distribution metrics.Distribution) Percentiles {
	return Percentiles{
		Mean:   distribution.Mean,
		Min:    distribution.Min,
		P5:     distribution.P5,
		P25:    distribution.P25,
		Median: distribution.Median,
		P75:    distribution.P75,
		P95:    distribution.P95,
		Max:    distribution.Max,
	}
}

func histogram(returns []float64) []HistogramBin {
	bins := metrics.Histogram(returns, HistogramBins)
	distribution := make([]HistogramBin, 0, len(bins))
	for _, bin := range bins {
		distribution = append(distribution, HistogramBin{Start: bin.Start, End: bin.End, Count: bin.Count})
	}
	return distribution
}

func windows(trades []Trade, returns, pnl []float64) []TradeWindow {
	if len(trades) < MinTradesForWindows {
		return []TradeWindow{}
	}

	slices := metrics.Windows(returns, pnl, StabilityWindows)
	windows := make([]TradeWindow, 0, len(slices))

	offset := 0
	for _, slice := range slices {
		start := trades[offset].Time
		end := trades[offset+slice.Trades-1].Time
		offset += slice.Trades

		windows = append(windows, TradeWindow{
			Index:      slice.Index,
			Trades:     slice.Trades,
			MeanReturn: slice.MeanReturn,
			NetProfit:  slice.NetProfit,
			WinRate:    slice.WinRate,
			Start:      start,
			End:        end,
		})
	}

	return windows
}

// sqn is the System Quality Number of Van Tharp, the t-statistic of the profit
// and loss: how many standard errors of profit the strategy produced.
func sqn(pnl []float64) float64 {
	if len(pnl) < 2 {
		return 0
	}

	deviation := metrics.StdDev(pnl)
	if deviation == 0 {
		return 0
	}

	return math.Sqrt(float64(len(pnl))) * metrics.Mean(pnl) / deviation
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	middle := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[middle-1] + sorted[middle]) / 2
	}

	return sorted[middle]
}

func longestLosingStreak(returns []float64) int {
	longest, current := 0, 0
	for _, value := range returns {
		if value < 0 {
			current++
			if current > longest {
				longest = current
			}
			continue
		}
		current = 0
	}
	return longest
}

func buyAndHold(candles []Candle) float64 {
	if len(candles) < 2 || candles[0].Close <= 0 {
		return 0
	}
	return (candles[len(candles)-1].Close - candles[0].Close) / candles[0].Close
}

func equityDrawdown(equity []Point) float64 {
	values := make([]float64, len(equity))
	for i, point := range equity {
		values[i] = point.Value
	}
	return metrics.MaxDrawdown(values)
}

// pointReturns turns a series of levels into the returns between them.
func pointReturns(points []Point) []float64 {
	returns := make([]float64, 0, len(points))
	for i := 1; i < len(points); i++ {
		if points[i-1].Value > 0 {
			returns = append(returns, (points[i].Value-points[i-1].Value)/points[i-1].Value)
		}
	}
	return returns
}

// compound replays the trade returns on a unit account, the equity curve of a
// strategy that reinvests everything. It is the fallback drawdown of a backtest
// without a paper wallet.
func compound(returns []float64) []float64 {
	equity := make([]float64, 0, len(returns)+1)
	value := 1.0
	equity = append(equity, value)
	for _, r := range returns {
		value *= 1 + r
		equity = append(equity, value)
	}
	return equity
}

// alignReturns pairs each equity return with the return of the asset over the
// same interval, which is what beta and alpha are measured on. Intervals
// without a matching candle are dropped from both samples, so they stay
// aligned.
func alignReturns(equity []Point, candles []Candle) (strategy, benchmark []float64) {
	closes := make(map[int64]float64, len(candles))
	for _, candle := range candles {
		closes[candle.Time] = candle.Close
	}

	strategy = make([]float64, 0, len(equity))
	benchmark = make([]float64, 0, len(equity))

	for i := 1; i < len(equity); i++ {
		previous, current := equity[i-1], equity[i]
		if previous.Value <= 0 {
			continue
		}

		previousClose, ok := closes[previous.Time]
		if !ok || previousClose <= 0 {
			continue
		}
		currentClose, ok := closes[current.Time]
		if !ok || currentClose <= 0 {
			continue
		}

		strategy = append(strategy, (current.Value-previous.Value)/previous.Value)
		benchmark = append(benchmark, (currentClose-previousClose)/previousClose)
	}

	return strategy, benchmark
}

// Statistics is the robustness report of one pair: the numbers the terminal
// summary prints plus the resampling tests that tell how much of the result is
// skill and how much is luck. The dashboard serves it at GET /api/{pair}/statistics and backtest results
// embed it per pair and for the whole run.
//
// Two samples back the report. Trade returns (one value per closed trade, as a
// fraction of the entry price) drive everything that counts trades, and bar
// returns (one value per candle, taken from the equity curve) drive everything
// annualized, since only those are evenly spaced in time.
type Statistics struct {
	Pair        string           `json:"pair"`
	Quote       string           `json:"quote"`
	Sample      Sample           `json:"sample"`
	Performance PerformanceStats `json:"performance"`
	Risk        RiskStats        `json:"risk"`
	Intervals   []Interval       `json:"intervals"`
	Tests       []HypothesisTest `json:"tests"`
	MonteCarlo  *MonteCarlo      `json:"monte_carlo"`
	// Distribution is the histogram of the trade returns.
	Distribution []HistogramBin `json:"distribution"`
	// Windows splits the trade history in equal slices, to expose an edge that
	// only exists in one stretch of the period.
	Windows []TradeWindow `json:"windows"`
}

// Sample describes what the report was computed from, so the dashboard can
// warn about a sample that is too small to conclude anything.
type Sample struct {
	Trades int `json:"trades"`
	Wins   int `json:"wins"`
	Losses int `json:"losses"`
	// Bars is the number of equity points (zero without a paper wallet).
	Bars int `json:"bars"`
	// BarsPerYear is inferred from the candle spacing and is what annualizes
	// the ratios. It is zero when there are not enough candles to tell.
	BarsPerYear float64 `json:"bars_per_year"`
	// Iterations and Seed make every resampling test on this page
	// reproducible: the same backtest always reports the same numbers.
	Iterations int   `json:"iterations"`
	Seed       int64 `json:"seed"`
}

// PerformanceStats holds the return side of the report.
type PerformanceStats struct {
	// NetProfit is in quote currency, ReturnPct is over the initial equity.
	NetProfit  float64 `json:"net_profit"`
	ReturnPct  float64 `json:"return_pct"`
	BuyAndHold float64 `json:"buy_and_hold"`
	WinRate    float64 `json:"win_rate"`
	// MeanReturn and MedianReturn are per trade, as a fraction.
	MeanReturn   float64 `json:"mean_return"`
	MedianReturn float64 `json:"median_return"`
	// Expectancy is the average profit of a trade in quote currency.
	Expectancy   float64 `json:"expectancy"`
	Payoff       float64 `json:"payoff"`
	ProfitFactor float64 `json:"profit_factor"`
	SQN          float64 `json:"sqn"`
	// SharpePerTrade is measured on the trade returns; the annual ratios are
	// measured on the bar returns and are zero without a paper wallet.
	SharpePerTrade float64 `json:"sharpe_per_trade"`
	SharpeAnnual   float64 `json:"sharpe_annual"`
	SortinoAnnual  float64 `json:"sortino_annual"`
	Calmar         float64 `json:"calmar"`
	// Alpha is the annualized excess return over holding the asset, and Beta
	// the exposure to it.
	Alpha float64 `json:"alpha"`
	Beta  float64 `json:"beta"`
}

// RiskStats holds the loss side of the report.
type RiskStats struct {
	MaxDrawdown float64 `json:"max_drawdown"`
	// ValueAtRisk is the trade return that the worst 5% of the trades exceed,
	// and ExpectedShortfall their average.
	ValueAtRisk       float64 `json:"value_at_risk"`
	ExpectedShortfall float64 `json:"expected_shortfall"`
	WorstTrade        float64 `json:"worst_trade"`
	BestTrade         float64 `json:"best_trade"`
	// LongestLosingStreak is the longest run of consecutive losing trades.
	LongestLosingStreak int     `json:"longest_losing_streak"`
	Skew                float64 `json:"skew"`
	ExcessKurtosis      float64 `json:"excess_kurtosis"`
	Autocorrelation     float64 `json:"autocorrelation"`
	// BreakEvenFee is the round-trip fee that would cancel the average trade.
	BreakEvenFee float64 `json:"break_even_fee"`
	// ProbabilisticSharpe is the probability that the true Sharpe is positive,
	// and MinimumTrackRecord the number of trades needed to claim it.
	ProbabilisticSharpe float64 `json:"probabilistic_sharpe"`
	MinimumTrackRecord  float64 `json:"minimum_track_record"`
}

// Interval is a bootstrap confidence interval of one measure. Threshold is the
// value that separates a working strategy from a broken one (zero for a
// return, one for a profit factor), so the dashboard can show whether the
// interval still contains it.
type Interval struct {
	ID        string  `json:"id"`
	Mean      float64 `json:"mean"`
	Lower     float64 `json:"lower"`
	Upper     float64 `json:"upper"`
	StdDev    float64 `json:"std_dev"`
	Threshold float64 `json:"threshold"`
	// Format is how the dashboard renders the numbers: percent or ratio.
	Format string `json:"format"`
}

// HypothesisTest is one statistical test of the report. Verdict is the reading
// of the p-value already resolved against RejectIsGood: "pass" when the test
// supports the strategy, "warn" when it questions it, and "inconclusive" when
// the sample was too small to run it.
type HypothesisTest struct {
	ID             string  `json:"id"`
	Statistic      float64 `json:"statistic"`
	StatisticLabel string  `json:"statistic_label"`
	PValue         float64 `json:"p_value"`
	RejectIsGood   bool    `json:"reject_is_good"`
	Verdict        string  `json:"verdict"`
}

// MonteCarlo is the result of replaying the backtest with resampled trade
// sequences. It answers what the single equity curve cannot: how much of the
// outcome hangs on the order the trades happened to arrive in.
type MonteCarlo struct {
	Iterations int `json:"iterations"`
	// FinalReturn and MaxDrawdown are fractions of the initial equity.
	FinalReturn Percentiles `json:"final_return"`
	MaxDrawdown Percentiles `json:"max_drawdown"`
	// ProfitProbability is the share of simulations that ended in profit and
	// RuinProbability the share whose drawdown passed RuinThreshold.
	ProfitProbability float64 `json:"profit_probability"`
	RuinProbability   float64 `json:"ruin_probability"`
	RuinThreshold     float64 `json:"ruin_threshold"`
	// Observed is the value the backtest actually produced, to be placed
	// against the simulated distribution.
	Observed float64 `json:"observed"`
}

// Percentiles summarizes a simulated distribution.
type Percentiles struct {
	Mean   float64 `json:"mean"`
	Min    float64 `json:"min"`
	P5     float64 `json:"p5"`
	P25    float64 `json:"p25"`
	Median float64 `json:"median"`
	P75    float64 `json:"p75"`
	P95    float64 `json:"p95"`
	Max    float64 `json:"max"`
}

// HistogramBin is one bar of the trade return distribution.
type HistogramBin struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Count int     `json:"count"`
}

// TradeWindow is the performance of one slice of the trade history.
type TradeWindow struct {
	Index      int     `json:"index"`
	Trades     int     `json:"trades"`
	MeanReturn float64 `json:"mean_return"`
	NetProfit  float64 `json:"net_profit"`
	WinRate    float64 `json:"win_rate"`
	// Start and End are the Unix seconds of the first and last trade.
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}
