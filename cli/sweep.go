package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/urfave/cli/v2"

	"github.com/rodrigo-brito/ninjabot/backtest"
	"github.com/rodrigo-brito/ninjabot/compare"
	"github.com/rodrigo-brito/ninjabot/portfolio"
	"github.com/rodrigo-brito/ninjabot/strategy"
	"github.com/rodrigo-brito/ninjabot/sweep"
	"github.com/rodrigo-brito/ninjabot/ui"
)

func sweepCommand(registry *strategy.Registry) *cli.Command {
	return &cli.Command{
		Name:  "sweep",
		Usage: "Run a parameter grid or random sample and rank the runs",
		Description: "The spec is a backtest spec plus \"grid\" ({\"params.fast\": [5, 8]}) or \"random\"\n" +
			"({\"samples\": 100, \"seed\": 1, \"ranges\": {...}}). Writes runs.jsonl and summary.json to --out;\n" +
			"runs already in runs.jsonl are reused.",
		Flags: []cli.Flag{
			specFlag,
			&cli.StringFlag{Name: "out", Aliases: []string{"o"}, Usage: "output directory, required"},
			&cli.IntFlag{Name: "parallel", Aliases: []string{"p"}, Usage: "runs at once (default: number of CPUs)"},
			&cli.StringFlag{Name: "metric", Aliases: []string{"m"}, Usage: "ranking metric", Value: "sharpe_annual"},
			&cli.IntFlag{Name: "top", Usage: "runs listed in the ranking (0: all)", Value: 20},
			&cli.BoolFlag{Name: "full", Usage: "write the artifacts of every run to runs/<hash>/"},
			&cli.BoolFlag{Name: "quiet", Aliases: []string{"q"}, Usage: "print nothing on success"},
		},
		Action: func(c *cli.Context) error {
			setLogLevel(false)
			path := c.String("spec")
			if path != "" && c.String("out") == "" {
				return usageError("--out is required")
			}
			spec, err := readSweepSpec(path)
			if err != nil {
				return err
			}

			summary, err := sweep.Run(c.Context, spec, filepath.Dir(path), registry, sweep.Options{
				Out: c.String("out"), Parallel: c.Int("parallel"), Metric: c.String("metric"),
				Top: c.Int("top"), Full: c.Bool("full"),
			})
			if err != nil {
				return err
			}
			if c.Bool("quiet") {
				return nil
			}

			table := tabwriter.NewWriter(c.App.Writer, 0, 0, 2, ' ', 0)
			fmt.Fprintf(table, "runs %d (cached %d, failed %d), ranked by %s\n",
				summary.Runs, summary.Cached, summary.Failed, summary.Metric)
			if o := summary.Overfitting; o != nil {
				fmt.Fprintf(table, "probability of backtest overfitting %.2f (%d splits)\n", o.PBO, o.Splits)
			}
			fmt.Fprintln(table, "rank\thash\t"+summary.Metric+"\treturn\tdrawdown\ttrades\tparams")
			for i, record := range summary.Ranking {
				value, _ := record.Metrics.Value(summary.Metric)
				params, _ := json.Marshal(record.Params)
				fmt.Fprintf(table, "%d\t%s\t%.4f\t%.2f%%\t%.2f%%\t%d\t%s\n", i+1, record.Hash[:12], value,
					record.Metrics.ReturnPct*100, record.Metrics.MaxDrawdown*100, record.Metrics.Trades, params)
			}
			return table.Flush()
		},
	}
}

func readSweepSpec(path string) (sweep.Spec, error) {
	if path == "" {
		return sweep.Spec{}, usageError("--spec is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return sweep.Spec{}, usageError("read spec: %v", err)
	}
	defer file.Close()
	return sweep.ReadSpec(file)
}

func walkForwardCommand(registry *strategy.Registry) *cli.Command {
	return &cli.Command{
		Name:  "walkforward",
		Usage: "Optimize a sweep on rolling training windows and score it on the windows that follow",
		Flags: []cli.Flag{
			specFlag,
			&cli.StringFlag{Name: "train", Usage: "training window, e.g. 90d", Value: "90d"},
			&cli.StringFlag{Name: "test", Usage: "test window, e.g. 30d", Value: "30d"},
			&cli.StringFlag{Name: "step", Usage: "roll step (default: the test window)"},
			&cli.StringFlag{Name: "metric", Aliases: []string{"m"}, Usage: "optimization metric", Value: "sharpe_annual"},
			&cli.IntFlag{Name: "parallel", Aliases: []string{"p"}, Usage: "runs at once (default: number of CPUs)"},
			&cli.StringFlag{Name: "format", Aliases: []string{"f"}, Usage: "table or json", Value: "table"},
		},
		Action: func(c *cli.Context) error {
			setLogLevel(false)
			spec, err := readSweepSpec(c.String("spec"))
			if err != nil {
				return err
			}
			result, err := sweep.RunWalkForward(c.Context, spec, filepath.Dir(c.String("spec")), registry,
				sweep.WalkForwardOptions{Train: c.String("train"), Test: c.String("test"), Step: c.String("step"),
					Metric: c.String("metric"), Parallel: c.Int("parallel")})
			if err != nil {
				return err
			}
			if c.String("format") == "json" {
				return writeJSON(c, result)
			}
			table := tabwriter.NewWriter(c.App.Writer, 0, 0, 2, ' ', 0)
			fmt.Fprintln(table, "test window	in-sample "+result.Metric+"	in-sample return	out-of-sample return	params")
			for _, fold := range result.Folds {
				in, _ := fold.InSample.Value(result.Metric)
				params, _ := json.Marshal(fold.Params)
				fmt.Fprintf(table, "%s → %s\t%.3f\t%.2f%%\t%.2f%%\t%s\n", fold.TestStart.Format("2006-01-02"),
					fold.TestEnd.Format("2006-01-02"), in, fold.InSample.ReturnPct*100, fold.OutSample.ReturnPct*100, params)
			}
			fmt.Fprintf(table, "out-of-sample return %.2f%%, walk-forward efficiency %.2f\n",
				result.OutOfSampleReturn*100, result.Efficiency)
			return table.Flush()
		},
	}
}

func sensitivityCommand(registry *strategy.Registry) *cli.Command {
	return &cli.Command{
		Name:  "sensitivity",
		Usage: "Run a backtest spec with several fee levels (slippage acts like an extra fee)",
		Flags: []cli.Flag{
			specFlag,
			&cli.Float64SliceFlag{Name: "fee", Usage: "fee levels as fractions (repeatable)",
				Value: cli.NewFloat64Slice(0, 0.0005, 0.001, 0.002, 0.004)},
			&cli.StringFlag{Name: "format", Aliases: []string{"f"}, Usage: "table or json", Value: "table"},
		},
		Action: func(c *cli.Context) error {
			setLogLevel(false)
			spec, dir, err := readSpec(c.String("spec"))
			if err != nil {
				return err
			}
			points, err := sweep.Sensitivity(c.Context, spec, dir, registry, c.Float64Slice("fee"))
			if err != nil {
				return err
			}
			if c.String("format") == "json" {
				return writeJSON(c, points)
			}
			table := tabwriter.NewWriter(c.App.Writer, 0, 0, 2, ' ', 0)
			fmt.Fprintln(table, "fee	trades	return	drawdown	sharpe	fees paid")
			for _, point := range points {
				m := point.Metrics
				fmt.Fprintf(table, "%.4f%%\t%d\t%.2f%%\t%.2f%%\t%.3f\t%.2f\n", point.Fee*100, m.Trades,
					m.ReturnPct*100, m.MaxDrawdown*100, m.SharpeAnnual, m.Fees)
			}
			return table.Flush()
		},
	}
}

func portfolioCommand(registry *strategy.Registry) *cli.Command {
	return &cli.Command{
		Name:  "portfolio",
		Usage: "Backtest several strategies together and report each one and the combination",
		Flags: []cli.Flag{
			specFlag,
			&cli.StringFlag{Name: "out", Aliases: []string{"o"}, Usage: "output directory, or - for portfolio.json on stdout"},
			&cli.IntFlag{Name: "parallel", Aliases: []string{"p"}, Usage: "strategies run at once (default: number of CPUs)"},
			&cli.BoolFlag{Name: "no-statistics", Usage: "skip the robustness reports (faster)"},
			&cli.BoolFlag{Name: "quiet", Aliases: []string{"q"}, Usage: "print nothing on success"},
			&cli.IntFlag{Name: "serve", Usage: "after the run, serve the dashboard on this port"},
		},
		Action: func(c *cli.Context) error {
			setLogLevel(false)
			path := c.String("spec")
			if path == "" {
				return usageError("--spec is required")
			}
			file, err := os.Open(path)
			if err != nil {
				return usageError("read spec: %v", err)
			}
			defer file.Close()
			spec, err := portfolio.ReadSpec(file)
			if err != nil {
				return err
			}
			if err := spec.Validate(); err != nil {
				return err
			}
			ds, err := spec.Runs()[0].LoadDataset(filepath.Dir(path))
			if err != nil {
				return err
			}
			result, err := portfolio.Run(c.Context, ds, spec, registry, portfolio.Options{
				Parallel: c.Int("parallel"), NoStatistics: c.Bool("no-statistics"),
			})
			if err != nil {
				return err
			}

			switch out := c.String("out"); out {
			case "-":
				return result.WriteJSON(c.App.Writer)
			case "":
			default:
				if err := writePortfolio(out, result); err != nil {
					return err
				}
			}
			if !c.Bool("quiet") {
				if err := printPortfolio(c, result); err != nil {
					return err
				}
			}
			if port := c.Int("serve"); port > 0 {
				chart, err := ui.New(ui.WithPortfolio(result), ui.WithPort(port))
				if err != nil {
					return err
				}
				fmt.Fprintf(c.App.ErrWriter, "dashboard on http://localhost:%d\n", port)
				return chart.Start()
			}
			return nil
		},
	}
}

// writePortfolio writes portfolio.json, equity.csv and the artifacts of
// every strategy under runs/<id>/.
func writePortfolio(dir string, result *portfolio.Result) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dir, "portfolio.json"), result.WriteJSON); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dir, EquityFile), result.WriteEquityCSV); err != nil {
		return err
	}
	runs := result.Spec.Runs()
	for i, run := range result.Strategies {
		if err := writeArtifacts(filepath.Join(dir, "runs", run.ID), runs[i], run.Result); err != nil {
			return err
		}
	}
	return nil
}

func printPortfolio(c *cli.Context, result *portfolio.Result) error {
	table := tabwriter.NewWriter(c.App.Writer, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "id\tweight\ttrades\treturn\tdrawdown\tsharpe\tpnl share\tmarginal sharpe")
	for i, run := range result.Strategies {
		m := run.Result.Metrics
		contribution := result.Combined.Contribution[i]
		fmt.Fprintf(table, "%s\t%.2f\t%d\t%.2f%%\t%.2f%%\t%.3f\t%.1f%%\t%+.3f\n", run.ID, run.Weight, m.Trades,
			m.ReturnPct*100, m.MaxDrawdown*100, m.SharpeAnnual, contribution.PnLShare*100, contribution.MarginalSharpe)
	}
	combined := result.Combined
	fmt.Fprintf(table, "portfolio\t1.00\t\t%.2f%%\t%.2f%%\t%.3f\t\t\n", combined.ReturnPct*100,
		combined.MaxDrawdown*100, combined.SharpeAnnual)
	fmt.Fprintln(table)
	fmt.Fprintln(table, "correlation\t"+strings.Join(combined.Correlation.IDs, "\t"))
	for i, id := range combined.Correlation.IDs {
		row := id
		for _, value := range combined.Correlation.Values[i] {
			row += fmt.Sprintf("\t%.2f", value)
		}
		fmt.Fprintln(table, row)
	}
	return table.Flush()
}

func compareCommand() *cli.Command {
	return &cli.Command{
		Name:      "compare",
		Usage:     "Compare runs side by side: result.json, portfolio.json or their directories",
		ArgsUsage: "<run> <run> ...",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{Name: "metric", Aliases: []string{"m"}, Usage: "metrics to compare (repeatable)"},
			&cli.StringFlag{Name: "format", Aliases: []string{"f"}, Usage: "table or json", Value: "table"},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() == 0 {
				return usageError("compare needs at least one run")
			}
			comparison, err := compare.Compare(c.Args().Slice(), c.StringSlice("metric"))
			if err != nil {
				if strings.Contains(err.Error(), "unknown metric") {
					return usageError("%v (known: %s)", err, strings.Join(metricNames(), ", "))
				}
				return err
			}
			switch c.String("format") {
			case "json":
				return writeJSON(c, comparison)
			case "table":
			default:
				return usageError("--format must be table or json")
			}

			table := tabwriter.NewWriter(c.App.Writer, 0, 0, 2, ' ', tabwriter.AlignRight)
			header := "metric\t"
			for _, run := range comparison.Runs {
				header += run.Name + "\t"
			}
			fmt.Fprintln(table, header)
			for _, metric := range comparison.Metrics {
				row := metric + "\t"
				for i, run := range comparison.Runs {
					value, _ := run.Metrics.Value(metric)
					if i == 0 {
						row += fmt.Sprintf("%.4f\t", value)
						continue
					}
					row += fmt.Sprintf("%.4f (%+.4f)\t", value, comparison.Deltas[i][metric])
				}
				fmt.Fprintln(table, row)
			}
			return table.Flush()
		},
	}
}

func metricNames() []string {
	names := make([]string, 0, len(backtest.MetricNames))
	for name := range backtest.MetricNames {
		names = append(names, name)
	}
	sortStrings(names)
	return names
}
