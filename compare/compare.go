// Package compare puts the results of several runs side by side: backtest
// results (result.json) and portfolio results (portfolio.json), read from
// their files or output directories.
package compare

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rodrigo-brito/ninjabot/backtest"
	"github.com/rodrigo-brito/ninjabot/portfolio"
)

// Kinds of runs.
const (
	KindBacktest  = "backtest"
	KindPortfolio = "portfolio"
)

// Run is one compared run.
type Run struct {
	Name    string           `json:"name"`
	Path    string           `json:"path"`
	Kind    string           `json:"kind"`
	Metrics backtest.Metrics `json:"metrics"`
}

// Comparison lists the runs and the difference of each metric to the first
// run.
type Comparison struct {
	Metrics []string             `json:"metrics"`
	Runs    []Run                `json:"runs"`
	Deltas  []map[string]float64 `json:"deltas"` // one per run, relative to Runs[0]
}

// DefaultMetrics are compared when none is chosen.
var DefaultMetrics = []string{
	"trades", "net_profit", "return_pct", "max_drawdown", "win_rate", "profit_factor",
	"sharpe_annual", "sortino_annual", "calmar",
}

// Load reads a run from a result file or from an output directory holding
// result.json or portfolio.json.
func Load(path string) (Run, error) {
	file := path
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		file = filepath.Join(path, "result.json")
		if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
			file = filepath.Join(path, "portfolio.json")
		}
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return Run{}, err
	}

	var probe struct {
		Combined json.RawMessage `json:"combined"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return Run{}, fmt.Errorf("compare: %s: %w", file, err)
	}

	run := Run{Name: filepath.Base(path), Path: file}
	if probe.Combined != nil {
		result, err := portfolio.ReadResult(bytes.NewReader(data))
		if err != nil {
			return Run{}, fmt.Errorf("compare: %s: %w", file, err)
		}
		run.Kind = KindPortfolio
		run.Metrics = portfolioMetrics(result)
		return run, nil
	}

	result, err := backtest.ReadResult(bytes.NewReader(data))
	if err != nil {
		return Run{}, fmt.Errorf("compare: %s: %w", file, err)
	}
	run.Kind = KindBacktest
	run.Metrics = result.Metrics
	return run, nil
}

// portfolioMetrics summarizes a portfolio with the metrics of a backtest.
func portfolioMetrics(result *portfolio.Result) backtest.Metrics {
	combined := result.Combined
	m := backtest.Metrics{
		NetProfit:    combined.NetProfit,
		ReturnPct:    combined.ReturnPct,
		MaxDrawdown:  combined.MaxDrawdown,
		SharpeAnnual: combined.SharpeAnnual,
	}
	for _, run := range result.Strategies {
		m.Trades += run.Result.Metrics.Trades
		m.Fees += run.Result.Metrics.Fees
	}
	if s := combined.Statistics; s != nil {
		m.WinRate = s.Performance.WinRate
		m.ProfitFactor = s.Performance.ProfitFactor
		m.Payoff = s.Performance.Payoff
		m.Expectancy = s.Performance.Expectancy
		m.SQN = s.Performance.SQN
		m.SortinoAnnual = s.Performance.SortinoAnnual
		m.Calmar = s.Performance.Calmar
	}
	return m
}

// Compare loads the runs and computes the deltas of the metrics.
func Compare(paths []string, metrics []string) (*Comparison, error) {
	if len(paths) == 0 {
		return nil, errors.New("compare: no run to compare")
	}
	if len(metrics) == 0 {
		metrics = DefaultMetrics
	}
	for _, metric := range metrics {
		if _, ok := backtest.MetricNames[metric]; !ok {
			return nil, fmt.Errorf("compare: unknown metric %q", metric)
		}
	}

	comparison := &Comparison{Metrics: metrics}
	for _, path := range paths {
		run, err := Load(path)
		if err != nil {
			return nil, err
		}
		comparison.Runs = append(comparison.Runs, run)
	}

	base := comparison.Runs[0].Metrics
	for _, run := range comparison.Runs {
		deltas := make(map[string]float64, len(metrics))
		for _, metric := range metrics {
			value, _ := run.Metrics.Value(metric)
			reference, _ := base.Value(metric)
			deltas[metric] = value - reference
		}
		comparison.Deltas = append(comparison.Deltas, deltas)
	}
	return comparison, nil
}
