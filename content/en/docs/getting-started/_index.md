---
title: "Getting Started"
linkTitle: "Getting Started"
categories: ["Guides"]
weight: 1
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

	"github.com/rodrigo-brito/ninjabot"
	"github.com/rodrigo-brito/ninjabot/examples/strategies"
	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/storage"
	"github.com/rodrigo-brito/ninjabot/ui"
	"github.com/rodrigo-brito/ninjabot/ui/indicator"

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
		// maker and taker fees charged on every fill, 0.1% is the Binance spot default
		exchange.WithPaperFee(0.001, 0.001),
		exchange.WithDataFeed(csvFeed),
	)

	// Initialize a chart to plot trading results
	chart, err := ui.New(
		ui.WithStrategyIndicators(strategy), // load indicators from strategy
		ui.WithCustomIndicators( // you can specify additional indicators
			indicator.RSI(14, "purple"),
			indicator.Stoch(8, 3, 3, "red", "blue"),
		),
		ui.WithPaperWallet(wallet), // necessary to display the equity chart
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

{{% pageinfo %}}
Up to `v0.5.0` the chart lived in the `plot` package and was created with `plot.NewChart`. It is now the `ui` package, created with `ui.New`, and the indicators moved from `plot/indicator` to `ui/indicator`.
{{% /pageinfo %}}

To execute your strategy, just run:

```bash
go run main.go
```

Output:

```
INFO[2026-08-24 07:21] [SETUP] Using paper wallet
INFO[2026-08-24 07:21] [SETUP] Initial Portfolio = 10000.000000 USDT
+---------+--------+-----+------+--------+--------+----------+-----+----------+-----------+
|  PAIR   | TRADES | WIN | LOSS | % WIN  | PAYOFF | PR FACT. | SQN |  PROFIT  |  VOLUME   |
+---------+--------+-----+------+--------+--------+----------+-----+----------+-----------+
| BTCUSDT |     21 |   7 |   14 | 33.3 % |  5.743 |    2.871 | 1.4 | 12759.66 | 437832.11 |
| ETHUSDT |      9 |   5 |    4 | 55.6 % |  4.803 |    6.004 | 1.3 | 20454.68 | 393786.29 |
+---------+--------+-----+------+--------+--------+----------+-----+----------+-----------+
|   TOTAL |     30 |  12 |   18 | 40.0 % |  5.461 |    3.811 | 1.3 | 33214.34 | 831618.40 |
+---------+--------+-----+------+--------+--------+----------+-----+----------+-----------+

------ RETURN -------
-9.134--4.507  20%    ████▋        6
-4.507-0.1201  43.3%  ██████████▏  13
0.1201-4.747   3.33%  ▊            1
4.747-9.374    3.33%  ▊            1
9.374-14       10%    ██▍          3
14-18.63       3.33%  ▊            1
18.63-23.25    6.67%  █▋           2
23.25-27.88    0%     ▏
27.88-32.51    3.33%  ▊            1
32.51-37.14    0%     ▏
37.14-41.76    0%     ▏
41.76-46.39    0%     ▏
46.39-51.02    0%     ▏
51.02-55.64    3.33%  ▊            1
55.64-60.27    3.33%  ▊            1

------ CONFIDENCE INTERVAL (95%) -------
| BTCUSDT |
RETURN:      4.41% (-1.37% ~ 11.46%)
PAYOFF:      5.99 (2.09 ~ 11.70)
PROF.FACTOR: 3.24 (0.54 ~ 8.48)
| ETHUSDT |
RETURN:      9.42% (-1.03% ~ 24.07%)
PAYOFF:      17.22 (1.10 ~ 23.17)
PROF.FACTOR: 65.16 (0.66 ~ 47.16)

----- FINAL WALLET -----
0.0000 BTC = 0.0185 USDT
0.0000 ETH = 0.0000 USDT
43214.3144 USDT

----- RETURNS -----
START PORTFOLIO     = 10000.00 USDT
FINAL PORTFOLIO     = 43214.33 USDT
GROSS PROFIT        =  34045.951221 USDT (340.46%)
TRADING FEES        =  831.618401 USDT
NET PROFIT          =  33214.332820 USDT (332.14%)
MARKET CHANGE (B&H) =  407.09%

------ RISK -------
MAX DRAWDOWN = -11.76 %

------ VOLUME -----
BTCUSDT         = 437832.11 USDT
ETHUSDT         = 393786.29 USDT
TOTAL           = 831618.40 USDT
-------------------
Dashboard available at http://localhost:8080

```

The meaning of each column, and how to export the raw returns, is described in [Backtesting]({{< relref "/docs/backtesting" >}}).

![Dashboard](https://user-images.githubusercontent.com/7620947/139601478-7b1d826c-f0f3-4766-951e-b11b1e1c9aa5.png)

## Next steps

- [Create your own strategy]({{< relref "/docs/strategy" >}})
- [Backtesting, fees and results]({{< relref "/docs/backtesting" >}})
- [Dashboard and live controls]({{< relref "/docs/dashboard" >}})
- [Telegram and notifications]({{< relref "/docs/telegram" >}})
