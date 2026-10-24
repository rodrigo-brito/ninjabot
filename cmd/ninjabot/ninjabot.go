// Command ninjabot downloads data and runs backtests from spec files. It
// knows the example strategies; see package cli to build a command line with
// your own.
package main

import (
	"github.com/rodrigo-brito/ninjabot/cli"
	_ "github.com/rodrigo-brito/ninjabot/examples/strategies" // registers the example strategies
	"github.com/rodrigo-brito/ninjabot/strategy"
)

func main() {
	cli.Main(strategy.Default)
}
