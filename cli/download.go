package cli

import (
	"github.com/urfave/cli/v2"

	"github.com/rodrigo-brito/ninjabot/download"
	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/service"
)

func downloadCommand() *cli.Command {
	return &cli.Command{
		Name:     "download",
		HelpName: "download",
		Usage:    "Download historical data",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "pair", Aliases: []string{"p"}, Usage: "eg. BTCUSDT", Required: true},
			&cli.IntFlag{Name: "days", Aliases: []string{"d"}, Usage: "eg. 100 (default 30 days)", Required: false},
			&cli.TimestampFlag{Name: "start", Aliases: []string{"s"}, Usage: "eg. 2021-12-01", Layout: "2006-01-02"},
			&cli.TimestampFlag{Name: "end", Aliases: []string{"e"}, Usage: "eg. 2020-12-31", Layout: "2006-01-02"},
			&cli.StringFlag{Name: "timeframe", Aliases: []string{"t"}, Usage: "eg. 1h", Required: true},
			&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Usage: "eg. ./btc.csv", Required: true},
			&cli.BoolFlag{Name: "futures", Aliases: []string{"f"}, Usage: "true or false", Value: false},
		},
		Action: func(c *cli.Context) error {
			var (
				exc service.Feeder
				err error
			)

			if c.Bool("futures") {
				// fetch data from binance futures
				exc, err = exchange.NewBinanceFuture(c.Context)
			} else {
				// fetch data from binance spot
				exc, err = exchange.NewBinance(c.Context)
			}
			if err != nil {
				return err
			}

			var options []download.Option
			if days := c.Int("days"); days > 0 {
				options = append(options, download.WithDays(days))
			}

			start := c.Timestamp("start")
			end := c.Timestamp("end")
			if start != nil && end != nil && !start.IsZero() && !end.IsZero() {
				options = append(options, download.WithInterval(*start, *end))
			} else if start != nil || end != nil {
				return usageError("START and END must be informed together")
			}

			return download.NewDownloader(exc).Download(c.Context, c.String("pair"),
				c.String("timeframe"), c.String("output"), options...)
		},
	}
}
