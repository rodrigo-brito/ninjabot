---
title: "Getting Started"
linkTitle: "Getting Started"
categories: ["Guides"]
weight: 2
description: >
    This page describes the first steps do install and setup a basic bot with Ninjabot
---

## Install CLI

Ninjabot CLI provides utilities commands to support backtesting and bot development.

You can install CLI with the following command
```bash
go install github.com/rodrigo-brito/ninjabot/cmd/ninjabot@latest
```
Or downloading pre-build binaries in [release page](https://github.com/rodrigo-brito/ninjabot/releases).

## Creating a new project

Create a new Go project and initialize `go module` with

```bash
go mod init example
```

Download the latest version of Ninjabot library
```bash
go get -u github.com/rodrigo-brito/ninjabot@latest
```

Downloading 720 days from BTCUSDT historical data for backtesting.
```bash
ninjabot download --pair BTCUSDT --timeframe 1d --days 720 --output ./btc.csv
```

## Creating a backtesting script

Create a new file `main.go` and include the following code:

```go
package main

import (
	"context"
	"fmt"

	"github.com/rodrigo-brito/ninjabot"
	"github.com/rodrigo-brito/ninjabot/examples/strategies"
	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/plot"
	"github.com/rodrigo-brito/ninjabot/storage"

	log "github.com/sirupsen/logrus"
)

func main() {
	ctx := context.Background()

	// Ninjabot settings
	settings := ninjabot.Settings{
		Pairs: []string{
			"BTCUSDT",
		},
	}

	// Load a custom strategy from examples folder
	// To create a custom strategy, check https://rodrigo-brito.github.io/ninjabot/docs/strategy/.
	strategy := new(strategies.CrossEMA)

	// Load your CSV with historical data
	csvFeed, err := exchange.NewCSVFeed(
		strategy.Timeframe(),
		exchange.PairFeed{
			Pair:      "BTCUSDT",
			File:      "btc.csv",
			Timeframe: "1d", // specify the dataset timeframe
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	// Create a storage in memory
	storage, err := storage.FromMemory()
	if err != nil {
		log.Fatal(err)
	}

	// Create a virtual wallet with 10.000 USDT
	wallet := exchange.NewPaperWallet(
		ctx,
		"USDT",
		exchange.WithPaperAsset("USDT", 10000),
		exchange.WithDataFeed(csvFeed),
	)

	// Initialize a chart to plot trading results
	chart := plot.NewChart()

	bot, err := ninjabot.NewBot(
		ctx,
		settings,
		wallet,
		strategy,
		ninjabot.WithStorage(storage),
		ninjabot.WithBacktest(wallet),
		ninjabot.WithCandleSubscription(chart),
		ninjabot.WithOrderSubscription(chart),
		ninjabot.WithLogLevel(log.WarnLevel),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Execute backtest
	err = bot.Run(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Print trading results
	fmt.Println(bot.Summary())
	wallet.Summary()
	err = chart.Start()
	if err != nil {
		log.Fatal(err)
	}
}
```

To execute your strategy, just run:

```bash
go run main.go
```

