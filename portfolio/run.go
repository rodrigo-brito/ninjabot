package portfolio

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"sort"
	"time"

	str2duration "github.com/xhit/go-str2duration/v2"
	"golang.org/x/sync/errgroup"
	"gonum.org/v1/gonum/stat"

	"github.com/rodrigo-brito/ninjabot/backtest"
	"github.com/rodrigo-brito/ninjabot/report"
	"github.com/rodrigo-brito/ninjabot/strategy"
	"github.com/rodrigo-brito/ninjabot/tools/metrics"
)

// ResultVersion is the version of the portfolio result layout.
const ResultVersion = 1

// Contribution is the share of a strategy in the portfolio result.
type Contribution struct {
	ID string `json:"id"`
	// PnL is the profit of the strategy in quote currency, and PnLShare its
	// fraction of the portfolio profit.
	PnL      float64 `json:"pnl"`
	PnLShare float64 `json:"pnl_share"`
	// MarginalSharpe is how much the annualized Sharpe ratio of the
	// portfolio drops without the strategy: positive when it helps.
	MarginalSharpe float64 `json:"marginal_sharpe"`
}

// Matrix is a square matrix over the strategies, in the order of IDs.
type Matrix struct {
	IDs    []string    `json:"ids"`
	Values [][]float64 `json:"values"`
}

// StrategyResult is the run of one strategy.
type StrategyResult struct {
	ID      string           `json:"id"`
	Weight  float64          `json:"weight"`
	Capital float64          `json:"capital"`
	Result  *backtest.Result `json:"result"`
}

// Combined is the portfolio as a whole.
type Combined struct {
	// Times are Unix seconds; Equity and ByStrategy have one value per time.
	Times       []int64              `json:"times"`
	Equity      []float64            `json:"equity"`
	ByStrategy  map[string][]float64 `json:"by_strategy"`
	Unallocated float64              `json:"unallocated"`
	Initial     float64              `json:"initial"`
	Final       float64              `json:"final"`
	NetProfit   float64              `json:"net_profit"`
	ReturnPct   float64              `json:"return_pct"`
	MaxDrawdown float64              `json:"max_drawdown"`
	// SharpeAnnual is measured on the bar returns of the combined curve,
	// sampled on the grid of the coarsest timeframe.
	SharpeAnnual float64 `json:"sharpe_annual"`
	// Correlation of the bar returns of the strategies, on the same grid.
	Correlation  Matrix             `json:"correlation"`
	Contribution []Contribution     `json:"contribution"`
	Statistics   *report.Statistics `json:"statistics,omitempty"`
}

// Result is the outcome of a portfolio backtest.
type Result struct {
	SchemaVersion int              `json:"schema_version"`
	Spec          Spec             `json:"spec"`
	Strategies    []StrategyResult `json:"strategies"`
	Combined      Combined         `json:"combined"`
	// Account is the shared wallet as a whole, in the shared mode.
	Account *backtest.Result `json:"account,omitempty"`
}

// Options tune a portfolio run.
type Options struct {
	// Parallel bounds the strategies run at once; runtime.NumCPU() when zero.
	Parallel int
	// NoStatistics skips the robustness reports of every run and of the
	// portfolio.
	NoStatistics bool
}

// Run backtests every strategy of the spec over the dataset, in parallel,
// and combines the results. The output does not depend on the order the
// runs finish in.
func Run(ctx context.Context, ds *backtest.Dataset, spec Spec, registry *strategy.Registry, options Options) (*Result, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	spec = spec.Normalize()

	if spec.Mode == ModeShared {
		return runShared(ctx, ds, spec, registry, options)
	}

	parallel := options.Parallel
	if parallel <= 0 {
		parallel = runtime.NumCPU()
	}

	runs := spec.Runs()
	results := make([]*backtest.Result, len(runs))
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(parallel)
	for i, run := range runs {
		group.Go(func() error {
			engine, err := backtest.FromSpec(ds, run, registry, func(cfg *backtest.Config) {
				cfg.NoStatistics = options.NoStatistics
			})
			if err != nil {
				return fmt.Errorf("strategy %s: %w", run.Name, err)
			}
			result, err := engine.Run(groupCtx)
			if err != nil {
				return fmt.Errorf("strategy %s: %w", run.Name, err)
			}
			results[i] = result
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}

	result := &Result{SchemaVersion: ResultVersion, Spec: spec}
	for i, run := range runs {
		result.Strategies = append(result.Strategies, StrategyResult{
			ID: spec.Strategies[i].ID, Weight: spec.Strategies[i].Weight, Capital: run.Capital, Result: results[i],
		})
	}
	result.Combined = combine(spec, result.Strategies, options.NoStatistics)
	return result, nil
}

// runShared runs the strategies on one wallet and attributes it to them.
func runShared(ctx context.Context, ds *backtest.Dataset, spec Spec, registry *strategy.Registry,
	options Options) (*Result, error) {
	if spec.Window != nil {
		ds = ds.Window(spec.Window.Start.Time, spec.Window.End.Time)
	}

	runs := spec.Runs()
	cfg := backtest.SharedConfig{
		Quote: spec.Quote, Capital: spec.Capital, MakerFee: spec.Fees.Maker, TakerFee: spec.Fees.Taker,
		NoStatistics: options.NoStatistics,
	}
	for i, s := range spec.Strategies {
		instance, err := registry.New(s.Strategy, s.Params)
		if err != nil {
			return nil, fmt.Errorf("strategy %s: %w", s.ID, err)
		}
		cfg.Strategies = append(cfg.Strategies, backtest.SharedStrategy{
			Tag: s.ID, Strategy: instance, Pairs: runs[i].Pairs, Timeframe: s.Timeframe, Budget: runs[i].Capital,
		})
	}

	shared, err := backtest.RunShared(ctx, ds, cfg)
	if err != nil {
		return nil, err
	}

	result := &Result{SchemaVersion: ResultVersion, Spec: spec, Account: shared.Account}
	for i, tag := range shared.Tags {
		tag.Result.Config.Strategy = spec.Strategies[i].Strategy
		if def, err := registry.Lookup(spec.Strategies[i].Strategy); err == nil {
			tag.Result.Config.Params, _ = def.Resolve(spec.Strategies[i].Params)
		}
		result.Strategies = append(result.Strategies, StrategyResult{
			ID: tag.Tag, Weight: spec.Strategies[i].Weight, Capital: runs[i].Capital, Result: tag.Result,
		})
	}
	result.Combined = combine(spec, result.Strategies, options.NoStatistics)
	return result, nil
}

func combine(spec Spec, runs []StrategyResult, noStatistics bool) Combined {
	curves := make(map[string][]backtest.Point, len(runs))
	initial := make(map[string]float64, len(runs))
	for _, run := range runs {
		curves[run.ID] = run.Result.Equity
		initial[run.ID] = run.Capital
	}
	times, byStrategy := AlignForwardFill(curves, initial)

	combined := Combined{
		Times:       times,
		ByStrategy:  byStrategy,
		Unallocated: spec.Unallocated(),
		Initial:     spec.Capital,
		Equity:      make([]float64, len(times)),
	}
	for i := range times {
		total := combined.Unallocated
		for _, run := range runs {
			total += byStrategy[run.ID][i]
		}
		combined.Equity[i] = total
	}

	combined.Final = spec.Capital
	if n := len(combined.Equity); n > 0 {
		combined.Final = combined.Equity[n-1]
	}
	combined.NetProfit = combined.Final - combined.Initial
	if combined.Initial > 0 {
		combined.ReturnPct = combined.NetProfit / combined.Initial
	}
	combined.MaxDrawdown = metrics.MaxDrawdown(combined.Equity)

	step := coarsestStep(runs)
	barsPerYear := 0.0
	if step > 0 {
		barsPerYear = float64(365*24*60*60) / float64(step)
	}
	sharpe := func(values []float64) float64 {
		r := returns(sample(times, values, step))
		if len(r) < 2 || barsPerYear == 0 {
			return 0
		}
		return finite(metrics.Annualize(metrics.Sharpe(r), barsPerYear))
	}
	combined.SharpeAnnual = sharpe(combined.Equity)

	ids := make([]string, len(runs))
	for i, run := range runs {
		ids[i] = run.ID
	}
	combined.Correlation = correlation(ids, times, byStrategy, step)

	for _, run := range runs {
		pnl := run.Result.Wallet.Final - run.Capital
		contribution := Contribution{ID: run.ID, PnL: pnl}
		if combined.NetProfit != 0 {
			contribution.PnLShare = pnl / combined.NetProfit
		}
		without := make([]float64, len(times))
		for i := range times {
			without[i] = combined.Equity[i] - byStrategy[run.ID][i] + initial[run.ID]
		}
		contribution.MarginalSharpe = combined.SharpeAnnual - sharpe(without)
		combined.Contribution = append(combined.Contribution, contribution)
	}

	if !noStatistics {
		statistics := portfolioStatistics(spec, runs, times, combined.Equity)
		combined.Statistics = &statistics
	}
	return combined
}

// coarsestStep is the longest timeframe among the runs, in seconds.
func coarsestStep(runs []StrategyResult) int64 {
	var step time.Duration
	for _, run := range runs {
		duration, err := str2duration.ParseDuration(run.Result.Config.Timeframe)
		if err == nil && duration > step {
			step = duration
		}
	}
	return int64(step.Seconds())
}

func correlation(ids []string, times []int64, byStrategy map[string][]float64, step int64) Matrix {
	matrix := Matrix{IDs: ids, Values: make([][]float64, len(ids))}
	series := make([][]float64, len(ids))
	for i, id := range ids {
		series[i] = returns(sample(times, byStrategy[id], step))
	}
	for i := range ids {
		matrix.Values[i] = make([]float64, len(ids))
		for j := range ids {
			switch {
			case i == j:
				matrix.Values[i][j] = 1
			case len(series[i]) > 1:
				matrix.Values[i][j] = finite(stat.Correlation(series[i], series[j], nil))
			}
		}
	}
	return matrix
}

func portfolioStatistics(spec Spec, runs []StrategyResult, times []int64, equity []float64) report.Statistics {
	trades := make([]report.Trade, 0)
	for _, run := range runs {
		for _, trade := range run.Result.Trades {
			trades = append(trades, report.Trade{ID: trade.OrderID, Time: trade.ClosedAt.Unix(),
				Profit: trade.ProfitPercent, ProfitValue: trade.ProfitValue})
		}
	}
	sort.SliceStable(trades, func(i, j int) bool { return trades[i].Time < trades[j].Time })

	points := make([]report.Point, len(times))
	for i, t := range times {
		points[i] = report.Point{Time: t, Value: equity[i]}
	}
	return report.Build(report.Input{Pair: "portfolio", Quote: spec.Quote, Trades: trades, Equity: points})
}

func finite(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
}
