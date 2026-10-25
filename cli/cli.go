// Package cli is the ninjabot command line: data download, backtests from
// spec files and the description of the strategies and file formats. It is
// a library so that programs with their own strategies get every command:
//
//	func main() {
//		strategy.RegisterParams("mine", "my strategy", NewMine)
//		cli.Main(strategy.Default)
//	}
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/rodrigo-brito/ninjabot/backtest"
	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/strategy"
)

// Exit codes of the command line.
const (
	ExitOK       = 0
	ExitRuntime  = 1 // the run failed
	ExitUsage    = 2 // invalid spec or arguments
	ExitData     = 3 // missing or insufficient data
	ExitStrategy = 4 // unknown strategy or invalid parameters
)

// Main runs the command line with os.Args and exits with its code.
func Main(registry *strategy.Registry) {
	os.Exit(Run(os.Args, os.Stdout, os.Stderr, registry))
}

// Run executes the command line and returns its exit code. Results go to
// stdout; logs and errors go to stderr.
func Run(args []string, stdout, stderr io.Writer, registry *strategy.Registry) int {
	app := NewApp(registry)
	app.Writer = stdout
	app.ErrWriter = stderr
	log.SetOutput(stderr)

	if err := app.Run(args); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return ExitCode(err)
	}
	return ExitOK
}

// ExitCode classifies an error into an exit code.
func ExitCode(err error) int {
	var exit cli.ExitCoder
	switch {
	case err == nil:
		return ExitOK
	case errors.As(err, &exit):
		return exit.ExitCode()
	case errors.Is(err, strategy.ErrUnknownStrategy), errors.Is(err, strategy.ErrInvalidParams):
		return ExitStrategy
	case errors.Is(err, backtest.ErrData), errors.Is(err, exchange.ErrInsufficientData),
		errors.Is(err, backtest.ErrUnknownPair), errors.Is(err, os.ErrNotExist):
		return ExitData
	case errors.Is(err, backtest.ErrInvalidSpec), errors.Is(err, errUsage):
		return ExitUsage
	}
	return ExitRuntime
}

var errUsage = errors.New("usage")

func sortStrings(values []string) { sort.Strings(values) }

func usageError(format string, args ...any) error {
	return fmt.Errorf("%w: %s", errUsage, fmt.Sprintf(format, args...))
}

// NewApp builds the command line over the strategies of registry.
func NewApp(registry *strategy.Registry) *cli.App {
	return &cli.App{
		Name:     "ninjabot",
		HelpName: "ninjabot",
		Usage:    "Utilities for bot creation and backtesting",
		// errors are printed once by Run, with their exit code
		ExitErrHandler: func(*cli.Context, error) {},
		OnUsageError: func(_ *cli.Context, err error, _ bool) error {
			return fmt.Errorf("%w: %w", errUsage, err)
		},
		Commands: []*cli.Command{
			downloadCommand(),
			backtestCommand(registry),
			strategiesCommand(registry),
			validateCommand(registry),
			schemaCommand(registry),
			portfolioCommand(registry),
			sweepCommand(registry),
			walkForwardCommand(registry),
			sensitivityCommand(registry),
			compareCommand(),
		},
	}
}
