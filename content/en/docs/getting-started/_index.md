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
go get -u github.com/rodrigo-brito/ninjabot/...
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
	chart, err := plot.NewChart(plot.WithIndicators(
		indicator.EMA(8, "red"),
		indicator.EMA(21, "#000"),
		indicator.RSI(14, "purple"),
		indicator.Stoch(8, 3, 3, "red", "blue"),
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

	// Execute backtest
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
```

To execute your strategy, just run:

```bash
go run main.go
```


Output:

```
INFO[2021-10-31 18:13] [SETUP] Using paper wallet                   
INFO[2021-10-31 18:13] [SETUP] Initial Portfolio = 10000.000000 USDT 
+---------+--------+-----+------+--------+--------+----------+-----------+
|  PAIR   | TRADES | WIN | LOSS | % WIN  | PAYOFF |  PROFIT  |  VOLUME   |
+---------+--------+-----+------+--------+--------+----------+-----------+
| BTCUSDT |     22 |  10 |   12 | 45.5 % |  4.726 |  7086.25 | 279230.67 |
| ETHUSDT |     22 |  14 |    8 | 63.6 % |  4.356 | 12723.04 | 272443.48 |
+---------+--------+-----+------+--------+--------+----------+-----------+
|   TOTAL |     44 |  24 |   20 | 54.5 % |  4.541 | 19809.29 | 551674.15 |
+---------+--------+-----+------+--------+--------+----------+-----------+

--------------
WALLET SUMMARY
--------------
0.000000 BTC = 0.000000 USDT
0.000000 ETH = 0.000000 USDT

TRADING VOLUME
BTCUSDT        = 279230.67 USDT
ETHUSDT        = 272443.48 USDT

29809.287688 USDT
--------------
START PORTFOLIO = 10000.00 USDT
FINAL PORTFOLIO = 29809.29 USDT
GROSS PROFIT    =  19809.287688 USDT (198.09%)
MARKET (B&H)    =  407.84%
MAX DRAWDOWN    =  -7.55 %
VOLUME          =  551674.15 USDT
COSTS (0.001*V) =  551.67 USDT (ESTIMATION) 
--------------

Chart available at http://localhost:8080
```

![SignatureJohnLennon](https://user-images.githubusercontent.com/7620947/139601478-7b1d826c-f0f3-4766-951e-b11b1e1c9aa5.png)
