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
	chart, err := plot.NewChart(
		plot.WithStrategyIndicators(strategy), // load indicators from strategy
		plot.WithCustomIndicators( // you can specify additiona indicators
			indicator.RSI(14, "purple"),
			indicator.Stoch(8, 3, 3, "red", "blue"),
		),
		plot.WithPaperWallet(wallet), // necessary to display the equity chart
	)
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
+---------+--------+-----+------+--------+--------+-----+----------+-----------+
|  PAIR   | TRADES | WIN | LOSS | % WIN  | PAYOFF | SQN |  PROFIT  |  VOLUME   |
+---------+--------+-----+------+--------+--------+-----+----------+-----------+
| BTCUSDT |     14 |   6 |    8 | 42.9 % |  5.929 | 1.5 | 13511.66 | 448030.05 |
| ETHUSDT |      9 |   6 |    3 | 66.7 % |  3.407 | 1.3 | 21748.41 | 407769.64 |
+---------+--------+-----+------+--------+--------+-----+----------+-----------+
|   TOTAL |     23 |  12 |   11 | 52.2 % |  4.942 | 1.4 | 35260.07 | 855799.68 |
+---------+--------+-----+------+--------+--------+-----+----------+-----------+

-- FINAL WALLET --
0.0000 BTC = 0.0000 USDT
0.0000 ETH = 0.0000 USDT
45260.0735 USDT

----- RETURNS -----
START PORTFOLIO     = 10000.00 USDT
FINAL PORTFOLIO     = 45260.07 USDT
GROSS PROFIT        =  35260.073493 USDT (352.60%)
MARKET CHANGE (B&H) =  407.09%

------ RISK -------
MAX DRAWDOWN = -11.76 %

------ VOLUME -----
ETHUSDT         = 407769.64 USDT
BTCUSDT         = 448030.05 USDT
TOTAL           = 855799.68 USDT
COSTS (0.001*V) = 855.80 USDT (ESTIMATION)
-------------------
Chart available at http://localhost:8080

```

![SignatureJohnLennon](https://user-images.githubusercontent.com/7620947/139601478-7b1d826c-f0f3-4766-951e-b11b1e1c9aa5.png)
