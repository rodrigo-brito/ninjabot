package sweep

import (
	"context"
	"fmt"
	"runtime"
	"time"

	str2duration "github.com/xhit/go-str2duration/v2"
	"golang.org/x/sync/errgroup"

	"github.com/rodrigo-brito/ninjabot/backtest"
	"github.com/rodrigo-brito/ninjabot/strategy"
)

// Evaluate runs the specs over one dataset, in parallel, and returns one
// record per spec, in order. Failures are recorded, not returned.
func Evaluate(ctx context.Context, ds *backtest.Dataset, specs []backtest.Spec, registry *strategy.Registry,
	parallel int) ([]Record, error) {
	if parallel <= 0 {
		parallel = runtime.NumCPU()
	}
	records := make([]Record, len(specs))
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(parallel)
	for i, run := range specs {
		group.Go(func() error {
			records[i] = execute(groupCtx, ds, run, hashOf(run, registry), registry, Options{})
			return groupCtx.Err()
		})
	}
	return records, group.Wait()
}

// best returns the index of the best successful record by metric, or -1.
func best(records []Record, metric string) int {
	higher := backtest.MetricNames[metric]
	index := -1
	var bestValue float64
	for i, record := range records {
		if record.Error != "" {
			continue
		}
		value, _ := record.Metrics.Value(metric)
		if index < 0 || (higher && value > bestValue) || (!higher && value < bestValue) {
			index, bestValue = i, value
		}
	}
	return index
}

// Fold is one step of a walk-forward analysis: the parameters chosen on the
// training window and how they did on the test window that follows.
type Fold struct {
	TrainStart time.Time        `json:"train_start"`
	TrainEnd   time.Time        `json:"train_end"`
	TestStart  time.Time        `json:"test_start"`
	TestEnd    time.Time        `json:"test_end"`
	Params     map[string]any   `json:"params"`
	Timeframe  string           `json:"timeframe,omitempty"`
	InSample   backtest.Metrics `json:"in_sample"`
	OutSample  backtest.Metrics `json:"out_of_sample"`
	Error      string           `json:"error,omitempty"`
}

// WalkForward is the outcome of a walk-forward analysis.
type WalkForward struct {
	Metric string `json:"metric"`
	Folds  []Fold `json:"folds"`
	// OutOfSampleReturn compounds the test windows; InSampleReturn
	// compounds the training windows of the chosen parameters.
	OutOfSampleReturn float64 `json:"out_of_sample_return"`
	InSampleReturn    float64 `json:"in_sample_return"`
	// Efficiency is the mean annualized out-of-sample return over the mean
	// annualized in-sample one: near 1 the optimization generalizes, near 0
	// or below it fits noise.
	Efficiency float64 `json:"efficiency"`
}

// WalkForwardOptions schedule the analysis.
type WalkForwardOptions struct {
	// Train and Test are window lengths such as "90d" and "30d"; Step
	// defaults to Test.
	Train, Test, Step string
	Metric            string // sharpe_annual by default
	Parallel          int
}

// RunWalkForward optimizes the sweep on each training window, keeps the best
// parameters and runs them on the following test window, rolling forward.
// A single fold with Train covering most of the data is a train/test split.
func RunWalkForward(ctx context.Context, spec Spec, dir string, registry *strategy.Registry,
	options WalkForwardOptions) (*WalkForward, error) {
	if options.Metric == "" {
		options.Metric = "sharpe_annual"
	}
	if _, ok := backtest.MetricNames[options.Metric]; !ok {
		return nil, fmt.Errorf("%w: unknown metric %q", backtest.ErrInvalidSpec, options.Metric)
	}
	train, err := parseLength("train", options.Train)
	if err != nil {
		return nil, err
	}
	test, err := parseLength("test", options.Test)
	if err != nil {
		return nil, err
	}
	step := test
	if options.Step != "" {
		if step, err = parseLength("step", options.Step); err != nil {
			return nil, err
		}
	}

	specs, err := spec.Expand()
	if err != nil {
		return nil, err
	}
	ds, err := spec.LoadDataset(dir)
	if err != nil {
		return nil, err
	}
	start, end, err := span(ds, spec.Normalize().Pairs)
	if err != nil {
		return nil, err
	}

	result := &WalkForward{Metric: options.Metric, OutOfSampleReturn: 1, InSampleReturn: 1}
	var inYearly, outYearly float64
	for from := start; !from.Add(train + test).After(end.Add(time.Second)); from = from.Add(step) {
		fold := Fold{TrainStart: from, TrainEnd: from.Add(train), TestStart: from.Add(train), TestEnd: from.Add(train + test)}

		trainSpecs := make([]backtest.Spec, len(specs))
		for i, s := range specs {
			s.Window = &backtest.Window{Start: backtest.Date{Time: fold.TrainStart},
				End: backtest.Date{Time: fold.TrainEnd.Add(-time.Second)}}
			trainSpecs[i] = s
		}
		records, err := Evaluate(ctx, ds, trainSpecs, registry, options.Parallel)
		if err != nil {
			return nil, err
		}
		chosen := best(records, options.Metric)
		if chosen < 0 {
			fold.Error = "no parameter set ran on the training window"
			result.Folds = append(result.Folds, fold)
			continue
		}
		fold.Params, fold.Timeframe, fold.InSample = records[chosen].Params, specs[chosen].Timeframe, records[chosen].Metrics

		testSpec := specs[chosen]
		testSpec.Window = &backtest.Window{Start: backtest.Date{Time: fold.TestStart},
			End: backtest.Date{Time: fold.TestEnd.Add(-time.Second)}}
		out, err := Evaluate(ctx, ds, []backtest.Spec{testSpec}, registry, 1)
		if err != nil {
			return nil, err
		}
		if out[0].Error != "" {
			fold.Error = out[0].Error
		} else {
			fold.OutSample = out[0].Metrics
			result.OutOfSampleReturn *= 1 + fold.OutSample.ReturnPct
			result.InSampleReturn *= 1 + fold.InSample.ReturnPct
			inYearly += fold.InSample.ReturnPct / train.Hours() * 24 * 365
			outYearly += fold.OutSample.ReturnPct / test.Hours() * 24 * 365
		}
		result.Folds = append(result.Folds, fold)
	}
	if len(result.Folds) == 0 {
		return nil, fmt.Errorf("%w: the data spans %s, shorter than train + test", backtest.ErrData, end.Sub(start))
	}

	result.OutOfSampleReturn--
	result.InSampleReturn--
	if inYearly != 0 {
		result.Efficiency = outYearly / inYearly
	}
	return result, nil
}

func parseLength(name, value string) (time.Duration, error) {
	duration, err := str2duration.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("%w: --%s must be a duration such as 30d, got %q", backtest.ErrInvalidSpec, name, value)
	}
	return duration, nil
}

// span is the time covered by the candles of the pairs.
func span(ds *backtest.Dataset, pairs []string) (time.Time, time.Time, error) {
	var start, end time.Time
	for _, pair := range pairs {
		timeframe, err := ds.Timeframe(pair)
		if err != nil {
			return start, end, err
		}
		candles, err := ds.Candles(pair, timeframe)
		if err != nil || len(candles) == 0 {
			return start, end, fmt.Errorf("%w: no candles for %s", backtest.ErrData, pair)
		}
		if start.IsZero() || candles[0].Time.Before(start) {
			start = candles[0].Time
		}
		if last := candles[len(candles)-1].Time; last.After(end) {
			end = last
		}
	}
	return start, end, nil
}

// SensitivityPoint is a run with one fee level.
type SensitivityPoint struct {
	Fee     float64          `json:"fee"`
	Metrics backtest.Metrics `json:"metrics"`
	Error   string           `json:"error,omitempty"`
}

// Sensitivity runs a spec with every fee level, applied to maker and taker
// fills alike. Slippage behaves like an extra fee on every fill, so the
// curve also tells how much slippage the strategy survives.
func Sensitivity(ctx context.Context, spec backtest.Spec, dir string, registry *strategy.Registry,
	fees []float64) ([]SensitivityPoint, error) {
	if len(fees) == 0 {
		return nil, fmt.Errorf("%w: no fee level", backtest.ErrInvalidSpec)
	}
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	ds, err := spec.LoadDataset(dir)
	if err != nil {
		return nil, err
	}
	specs := make([]backtest.Spec, len(fees))
	for i, fee := range fees {
		if fee < 0 || fee >= 1 {
			return nil, fmt.Errorf("%w: fee %v is not a fraction in [0, 1)", backtest.ErrInvalidSpec, fee)
		}
		specs[i] = spec
		specs[i].Fees = backtest.Fees{Maker: fee, Taker: fee}
	}
	records, err := Evaluate(ctx, ds, specs, registry, 0)
	if err != nil {
		return nil, err
	}
	points := make([]SensitivityPoint, len(fees))
	for i, record := range records {
		points[i] = SensitivityPoint{Fee: fees[i], Metrics: record.Metrics, Error: record.Error}
	}
	return points, nil
}
