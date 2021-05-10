package main

import (
	"context"

	"github.com/rodrigo-brito/ninjabot"
	"github.com/rodrigo-brito/ninjabot/example"
	"github.com/rodrigo-brito/ninjabot/pkg/exchange"
	"github.com/rodrigo-brito/ninjabot/pkg/model"
	"github.com/rodrigo-brito/ninjabot/pkg/storage"

	log "github.com/sirupsen/logrus"
)

func main() {
	ctx := context.Background()

	settings := model.Settings{
		Pairs: []string{
			"BTCUSDT",
		},
	}

	dataSource, err := exchange.NewCSVFeed(exchange.PairFeed{
		Pair:      "BTCUSDT",
		File:      "testdata/btc-1m.csv",
		Timeframe: "1m",
	})
	if err != nil {
		log.Fatal(err)
	}

	storage, err := storage.NewMemory()
	if err != nil {
		log.Fatal(err)
	}

	paperWallet := exchange.NewPaperWallet(
		ctx,
		"USDT",
		exchange.WithPaperAsset("USDT", 10000),
		exchange.WithDataSource(dataSource),
	)

	strategy := new(example.MyStrategy)
	bot, err := ninjabot.NewBot(
		ctx,
		settings,
		paperWallet,
		strategy,
		ninjabot.WithStorage(storage),
		ninjabot.WithCandleSubscription(paperWallet),
		ninjabot.WithLogLevel(log.ErrorLevel),
	)
	if err != nil {
		log.Fatalln(err)
	}

	err = bot.Run(ctx)
	if err != nil {
		log.Fatalln(err)
	}

	// Print bot results
	bot.Summary()
	paperWallet.Summary()
}
