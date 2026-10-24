package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/rodrigo-brito/ninjabot/backtest"
	"github.com/rodrigo-brito/ninjabot/strategy"
)

// Artifacts written by `backtest --out <dir>`.
const (
	ResultFile = "result.json"
	TradesFile = "trades.csv"
	EquityFile = "equity.csv"
	OrdersFile = "orders.csv"
	SpecFile   = "spec.json"
)

// specFlag is checked by readSpec rather than marked required, so a missing
// spec exits with the usage code.
var specFlag = &cli.StringFlag{Name: "spec", Aliases: []string{"s"}, Usage: "run spec file (JSON or YAML), required"}

func backtestCommand(registry *strategy.Registry) *cli.Command {
	return &cli.Command{
		Name:  "backtest",
		Usage: "Run a backtest described by a spec file",
		Description: "Writes result.json, trades.csv, equity.csv, orders.csv and the normalized spec.json\n" +
			"to --out, or result.json to stdout with --out -. Exit codes: 0 ok, 1 runtime error,\n" +
			"2 invalid spec, 3 data error, 4 unknown strategy or invalid parameters.",
		Flags: []cli.Flag{
			specFlag,
			&cli.StringFlag{Name: "out", Aliases: []string{"o"}, Usage: "output directory, or - for result.json on stdout"},
			&cli.StringFlag{Name: "format", Aliases: []string{"f"}, Usage: "stdout format: table or json", Value: "table"},
			&cli.BoolFlag{Name: "quiet", Aliases: []string{"q"}, Usage: "print nothing on success"},
			&cli.BoolFlag{Name: "no-statistics", Usage: "skip the robustness reports (faster)"},
			&cli.BoolFlag{Name: "verbose", Aliases: []string{"v"}, Usage: "log orders and trades to stderr"},
		},
		Action: func(c *cli.Context) error {
			format := c.String("format")
			if format != "table" && format != "json" {
				return usageError("--format must be table or json, got %q", format)
			}
			setLogLevel(c.Bool("verbose"))

			spec, dir, err := readSpec(c.String("spec"))
			if err != nil {
				return err
			}
			result, err := runSpec(c, spec, dir, registry, c.Bool("no-statistics"))
			if err != nil {
				return err
			}

			out := c.String("out")
			switch {
			case out == "-":
				return result.WriteJSON(c.App.Writer)
			case out != "":
				if resolved, err := spec.Resolve(registry); err == nil {
					spec = resolved
				}
				if err := writeArtifacts(out, spec, result); err != nil {
					return err
				}
			}

			if c.Bool("quiet") {
				return nil
			}
			if format == "json" {
				return result.WriteJSON(c.App.Writer)
			}
			if resolved, err := spec.Resolve(registry); err == nil {
				spec = resolved
			}
			return printSummary(c.App.Writer, spec, result)
		},
	}
}

func setLogLevel(verbose bool) {
	if verbose {
		log.SetLevel(log.InfoLevel)
		return
	}
	log.SetLevel(log.WarnLevel)
}

// readSpec reads and validates a spec file. It returns the directory of the
// file, which relative data files are resolved against.
func readSpec(path string) (backtest.Spec, string, error) {
	if path == "" {
		return backtest.Spec{}, "", usageError("--spec is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return backtest.Spec{}, "", usageError("read spec: %v", err)
	}
	defer file.Close()

	spec, err := backtest.ReadSpec(file)
	if err != nil {
		return backtest.Spec{}, "", err
	}
	if err := spec.Validate(); err != nil {
		return backtest.Spec{}, "", err
	}
	return spec, filepath.Dir(path), nil
}

func runSpec(c *cli.Context, spec backtest.Spec, dir string, registry *strategy.Registry,
	noStatistics bool) (*backtest.Result, error) {
	ds, err := spec.LoadDataset(dir)
	if err != nil {
		return nil, err
	}
	engine, err := backtest.FromSpec(ds, spec, registry, func(cfg *backtest.Config) {
		cfg.NoStatistics = noStatistics
	})
	if err != nil {
		return nil, err
	}
	return engine.Run(c.Context)
}

// writeArtifacts writes the files of a run to dir.
func writeArtifacts(dir string, spec backtest.Spec, result *backtest.Result) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	writers := []struct {
		name  string
		write func(io.Writer) error
	}{
		{ResultFile, result.WriteJSON},
		{TradesFile, result.WriteTradesCSV},
		{EquityFile, result.WriteEquityCSV},
		{OrdersFile, result.WriteOrdersCSV},
		{SpecFile, func(w io.Writer) error {
			encoder := json.NewEncoder(w)
			encoder.SetIndent("", "  ")
			return encoder.Encode(spec.Normalize())
		}},
	}
	for _, writer := range writers {
		if err := writeFile(filepath.Join(dir, writer.name), writer.write); err != nil {
			return err
		}
	}
	return nil
}

func writeFile(path string, write func(io.Writer) error) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := write(file); err != nil {
		file.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	return file.Close()
}

func printSummary(w io.Writer, spec backtest.Spec, result *backtest.Result) error {
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	row := func(label, format string, args ...any) {
		fmt.Fprintf(table, "%s\t%s\n", label, fmt.Sprintf(format, args...))
	}

	wallet := result.Wallet
	row("strategy", "%s %v", result.Config.Strategy, result.Config.Params)
	row("spec", "%s", spec.Hash()[:12])
	row("pairs", "%s", strings.Join(result.Config.Pairs, ", "))
	row("period", "%s → %s (%d candles)", result.Period.Start.Format("2006-01-02"),
		result.Period.End.Format("2006-01-02"), result.Period.Candles)
	row("trades", "%d", len(result.Trades))
	row("net profit", "%.2f %s (%.2f%%)", wallet.NetProfit, result.Config.Quote, wallet.ReturnPct*100)
	row("buy and hold", "%.2f%%", wallet.BuyAndHold*100)
	row("max drawdown", "%.2f%%", wallet.MaxDrawdown.Value*100)
	row("fees", "%.2f %s", wallet.Fees, result.Config.Quote)
	if p := result.Portfolio; p != nil {
		row("win rate", "%.1f%%", p.Performance.WinRate*100)
		row("profit factor", "%.3f", p.Performance.ProfitFactor)
		row("sharpe (annual)", "%.3f", p.Performance.SharpeAnnual)
		row("sortino (annual)", "%.3f", p.Performance.SortinoAnnual)
	}
	for _, warning := range result.Warnings {
		row("warning", "%s", warning)
	}
	return table.Flush()
}
