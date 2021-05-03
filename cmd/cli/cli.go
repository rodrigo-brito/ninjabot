package main

import (
	"log"
	"os"

	"github.com/rodrigo-brito/ninjabot/pkg/data"
	"github.com/rodrigo-brito/ninjabot/pkg/exchange"

	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "download",
		Usage: "download historical data",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "symbol",
				Aliases:  []string{"s"},
				Usage:    "eg. BTCUSDT",
				Required: true,
			},
			&cli.IntFlag{
				Name:     "limit",
				Aliases:  []string{"l"},
				Usage:    "eg. 100",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "timeframe",
				Aliases:  []string{"t"},
				Usage:    "eg. 1h",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "output",
				Aliases:  []string{"o"},
				Usage:    "eg. ./data",
				Required: true,
			},
		},
		Action: func(c *cli.Context) error {
			exc, err := exchange.NewBinance(c.Context)
			if err != nil {
				return err
			}
			return data.NewDownloader(exc).Download(c.Context, c.String("symbol"),
				c.String("timeframe"), c.Int("limit"), c.String("output"))
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
