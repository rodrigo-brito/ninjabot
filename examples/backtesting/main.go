package main

import (
	"context"

	"github.com/rodrigo-brito/ninjabot"
	"github.com/rodrigo-brito/ninjabot/examples/strategies"
	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/plot"
	"github.com/rodrigo-brito/ninjabot/plot/indicator"
	"github.com/rodrigo-brito/ninjabot/storage"

	log "github.com/sirupsen/logrus"
)

func main() {
	ctx := context.Background()

	settings := ninjabot.Settings{
		Pairs: []string{
			"BTCUSDT",
			"ETHUSDT",
		},
	}

	strategy := new(strategies.CrossEMA)

	csvFeed, err := exchange.NewCSVFeed(
		strategy.Timeframe(),
		exchange.PairFeed{
			Pair:      "BTCUSDT",
			File:      "testdata/btc-1h.csv",
			Timeframe: "1h",
		},
		exchange.PairFeed{
			Pair:      "ETHUSDT",
			File:      "testdata/eth-1h.csv",
			Timeframe: "1h",
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	storage, err := storage.FromMemory()
	if err != nil {
		log.Fatal(err)
	}

	wallet := exchange.NewPaperWallet(
		ctx,
		"USDT",
		exchange.WithPaperAsset("USDT", 10000),
		exchange.WithDataFeed(csvFeed),
	)

	chart, err := plot.NewChart(plot.WithIndicators(
		indicator.EMA(8, "red"),
		indicator.SMA(21, "#000"),
		indicator.RSI(14, "purple"),
	), plot.WithPaperWallet(wallet))
	if err != nil {
		log.Fatal(err)
	}

	bot, err := ninjabot.NewBot(
		ctx,
		settings,
		wallet,
		strategy,
		ninjabot.WithBacktest(wallet),
		ninjabot.WithStorage(storage),
		ninjabot.WithCandleSubscription(chart),
		ninjabot.WithOrderSubscription(chart),
		ninjabot.WithLogLevel(log.WarnLevel),
	)
	if err != nil {
		log.Fatal(err)
	}

	err = bot.Run(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Print bot results
	bot.Summary()

	// Display candlesticks chart in browser
	err = chart.Start()
	if err != nil {
		log.Fatal(err)
	}
}
