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

	dataSource, err := exchange.NewCSVFeed(
		"1d",
		exchange.PairFeed{
			Pair: "BTCUSDT",
			File: "data/btc-1d.csv",
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	storage, err := storage.New("backtest.db")
	if err != nil {
		log.Fatal(err)
	}

	wallet := exchange.NewPaperWallet(
		ctx,
		"USDT",
		exchange.WithPaperAsset("USDT", 10000),
		exchange.WithDataSource(dataSource),
	)

	strategy := new(example.MyStrategy)
	bot, err := ninjabot.NewBot(
		ctx,
		settings,
		wallet,
		strategy,
		ninjabot.WithStorage(storage),
		ninjabot.WithCandleSubscription(wallet),
		ninjabot.WithLogLevel(log.ErrorLevel),
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
	wallet.Summary()
}
