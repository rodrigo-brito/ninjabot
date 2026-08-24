![Ninjabot](https://user-images.githubusercontent.com/7620947/161434011-adc89d1a-dccb-45a7-8a07-2bb55e62d2d9.png)

[![tests](https://github.com/rodrigo-brito/ninjabot/actions/workflows/ci.yaml/badge.svg)](https://github.com/rodrigo-brito/ninjabot/actions/workflows/ci.yaml)
[![codecov](https://codecov.io/gh/rodrigo-brito/ninjabot/branch/main/graph/badge.svg)](https://codecov.io/gh/rodrigo-brito/ninjabot)
[![Go Reference](https://pkg.go.dev/badge/github.com/rodrigo-brito/ninjabot.svg)](https://pkg.go.dev/github.com/rodrigo-brito/ninjabot)
[![Discord](https://img.shields.io/discord/960156400376483840?color=5865F2&label=discord)](https://discord.gg/TGCrUH972E)
[![Discord](https://img.shields.io/badge/donate-patreon-red)](https://www.patreon.com/ninjabot_github)

A fast cryptocurrency trading bot framework implemented in Go. Ninjabot permits users to create and test custom strategies for spot and futures market. 

Docs: https://rodrigo-brito.github.io/ninjabot/

| DISCLAIMER                                                                                                                                                                                                           |
|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| This software is for educational purposes only. Do not risk money which you are afraid to lose.  USE THE SOFTWARE AT YOUR OWN RISK. THE AUTHORS AND ALL AFFILIATES ASSUME NO RESPONSIBILITY FOR YOUR TRADING RESULTS |

## Contents

- [Installation](#installation)
- [Examples of Usage](#examples-of-usage) - [CLI](#cli), [Backtesting](#backtesting-example)
- [Creating a strategy](#creating-a-strategy)
- [Trading Fees](#trading-fees)
- [Dashboard](#dashboard) - [Bot control panel](#bot-control-panel), [HTTP API](#http-api)
- [Indicators](#indicators)
- [Telegram and notifications](#telegram-and-notifications)
- [Short positions](#short-positions)
- [Storage](#storage)
- [Trading tools](#trading-tools)
- [Exchange options](#exchange-options)
- [Features](#features) and [Roadmap](#roadmap)

## Installation

`go get -u github.com/rodrigo-brito/ninjabot/...`

## Examples of Usage

Check [examples](examples) directory:

- [Backtesting](examples/backtesting/backtesting.go) - simulation with historical data from CSV
- [Paper Wallet](examples/paperwallet/paperwallet.go) - live simulation with real time data, dashboard control panel included
- [Spot Market](examples/spotmarket/spot.go) - real account on Binance spot
- [Futures Market](examples/futuremarket/futures.go) - real account on Binance futures, with leverage and margin type
- [Strategies](examples/strategies) - EMA cross, OCO sell, trailing stop and turtle

### CLI

To download historical data you can download ninjabot CLI from:

- Pre-build binaries in [release page](https://github.com/rodrigo-brito/ninjabot/releases)
- Or with `go install github.com/rodrigo-brito/ninjabot/cmd/ninjabot@latest`

**Example of usage**
```bash
# Download candles of BTCUSDT to btc.csv file (Last 30 days, timeframe 1D)
ninjabot download --pair BTCUSDT --timeframe 1d --days 30 --output ./btc.csv

# A fixed period, from the futures market
ninjabot download --pair BTCUSDT --timeframe 1h --start 2024-01-01 --end 2024-06-30 --output ./btc.csv --futures
```

| Flag | Description |
|---|---|
| `--pair`, `-p` | Pair to download, eg. `BTCUSDT`. **Required** |
| `--timeframe`, `-t` | Candle timeframe, eg. `1m`, `1h`, `1d`. **Required** |
| `--output`, `-o` | Destination CSV file. **Required** |
| `--days`, `-d` | Number of days to download, counting back from today. Without `--days`, `--start` and `--end`, the last month is downloaded. |
| `--start`, `-s` / `--end`, `-e` | Fixed period, `YYYY-MM-DD`. Both must be informed together, and they take precedence over `--days`. |
| `--futures`, `-f` | Download from the futures market instead of spot. |

### Backtesting Example

- Backtesting a custom strategy from [examples](examples) directory:
```
go run examples/backtesting/backtesting.go
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

The results table is printed by `bot.Summary()`, followed by the distribution of the trade returns and a bootstrapped 95% confidence interval for the return, payoff and profit factor of each pair:

| Column | Meaning |
|---|---|
| `TRADES`, `WIN`, `LOSS`, `% WIN` | Number of closed trades and the hit rate. |
| `PAYOFF` | Average win divided by the average loss, in percentage of the position. |
| `PR FACT.` | Profit factor: sum of the winning returns divided by the sum of the losing ones. |
| `SQN` | System Quality Number (Van Tharp): `sqrt(trades) * mean / stddev` of the trade results. It needs at least two trades with distinct results. |
| `PROFIT`, `VOLUME` | Net profit (after fees) and traded volume, in quote currency. |

The raw data is available in `bot.Controller().Results`, and `bot.SaveReturns(dir)` writes one file per pair with the return of every trade, one per line - handy to analyse the distribution elsewhere. The directory must already exist:

```go
bot.Summary()
err := bot.SaveReturns("./returns") // ./returns/BTCUSDT.csv, ./returns/ETHUSDT.csv
```

### Creating a strategy

A strategy implements four methods - the timeframe it runs on, how many candles it needs to warm up the indicators, the indicators themselves (also plotted on the dashboard) and the trading logic, called once per closed candle:

```go
type CrossEMA struct{}

func (e CrossEMA) Timeframe() string { return "4h" }

func (e CrossEMA) WarmupPeriod() int { return 22 }

func (e CrossEMA) Indicators(df *ninjabot.Dataframe) []strategy.ChartIndicator {
    df.Metadata["ema8"] = indicator.EMA(df.Close, 8)
    df.Metadata["sma21"] = indicator.SMA(df.Close, 21)

    return []strategy.ChartIndicator{
        {
            Overlay:   true,
            GroupName: "MA's",
            Time:      df.Time,
            Metrics: []strategy.IndicatorMetric{
                {Values: df.Metadata["ema8"], Name: "EMA 8", Color: "red", Style: strategy.StyleLine},
                {Values: df.Metadata["sma21"], Name: "SMA 21", Color: "blue", Style: strategy.StyleLine},
            },
        },
    }
}

func (e *CrossEMA) OnCandle(df *ninjabot.Dataframe, broker service.Broker) {
    assetPosition, quotePosition, err := broker.Position(df.Pair)
    if err != nil {
        log.Error(err)
        return
    }

    if quotePosition >= 10 && df.Metadata["ema8"].Crossover(df.Metadata["sma21"]) {
        _, err := broker.CreateOrderMarket(ninjabot.SideTypeBuy, df.Pair, quotePosition/df.Close.Last(0))
        if err != nil {
            log.Error(err)
        }
        return
    }

    if assetPosition > 0 && df.Metadata["ema8"].Crossunder(df.Metadata["sma21"]) {
        if _, err := broker.CreateOrderMarket(ninjabot.SideTypeSell, df.Pair, assetPosition); err != nil {
            log.Error(err)
        }
    }
}
```

The `broker` places market, limit, stop and OCO orders (`CreateOrderMarket`, `CreateOrderMarketQuote`, `CreateOrderLimit`, `CreateOrderStop`, `CreateOrderOCO`), cancels them (`Cancel`) and reports the current position (`Position`, `Account`).

To also react **before** the candle closes, implement `strategy.HighFrequencyStrategy` - the same interface plus `OnPartialCandle(df, broker)`, called on every update of the open candle, once the warmup period is filled. In backtesting the partial candles come from the resampling of the CSV feed, so a strategy on `4h` fed with a `1h` CSV is called three times before each close.

The complete strategies are in [examples/strategies](examples/strategies).

### Trading Fees

The paper wallet charges the exchange fees on every fill, so backtest and paper trading results are net of costs:

```go
wallet := exchange.NewPaperWallet(
    ctx,
    "USDT",
    exchange.WithPaperAsset("USDT", 10000),
    exchange.WithPaperFee(0.001, 0.001), // maker, taker - 0.1% is the Binance spot default
    exchange.WithDataFeed(csvFeed),
)
```

Orders that rest on the book (limit and take profit) pay the **maker** fee, the ones that cross the spread (market and triggered stop orders) pay the **taker** fee. The fee is charged in quote currency, stored in `Order.Fee` and deducted from the profit of each trade, so the results table, the dashboard and the `NET PROFIT` line all report returns after costs.

An order sized with the entire available balance leaves no room for its fee - the wallet fills a slightly smaller quantity instead of rejecting it.

### Dashboard

The `ui` package serves a web dashboard with candles, indicators, orders, equity curve and trade statistics, with live updates when running in paper/live mode:

```go
chart, err := ui.New(
    ui.WithStrategyIndicators(strategy),
    ui.WithCustomIndicators(indicator.RSI(14, "purple")),
    ui.WithPaperWallet(wallet),
)
// ... ninjabot.WithCandleSubscription(chart), ninjabot.WithOrderSubscription(chart)
chart.Start() // http://localhost:8080
```

The dashboard bundle is **not embedded** in your binary. On first start it is downloaded from the GitHub release that matches the ninjabot version in your `go.mod`, verified against the release checksums and cached in your user cache directory (`~/Library/Caches/ninjabot/ui` on macOS, `~/.cache/ninjabot/ui` on Linux, `%LocalAppData%\ninjabot\ui` on Windows). After that it works offline.

| Override | Effect |
|---|---|
| `NINJABOT_UI_DIR=/path/to/web/dist` or `ui.WithUIDir(dir)` | Serve a local build of the dashboard (no download). |
| `NINJABOT_UI_VERSION=v1.2.3` / `latest` or `ui.WithUIVersion(v)` | Pin the release tag to download. By default the tag is the ninjabot version in your `go.mod`; for a commit hash (pseudo-version) the nearest release before it is used, and inside a local checkout (`go run ./examples/...`, `replace` directive) the nearest `git` tag. |
| `ui.WithCacheDir(dir)` | Change where bundles are cached. |

When no release can be inferred - a development build with no reachable `git` tag - the latest release is used and a warning is logged, since that bundle may not match the API of your build. Building the dashboard locally (`make ui-install && make ui-build`) and pointing `NINJABOT_UI_DIR` at `web/dist` avoids the mismatch - see [CONTRIBUTING.md](CONTRIBUTING.md) for the web development workflow.

If the bundle cannot be fetched, the bot keeps running, the JSON API stays up and `http://localhost:8080` explains how to fix it.

<img width="100%"  src="https://user-images.githubusercontent.com/7620947/139601478-7b1d826c-f0f3-4766-951e-b11b1e1c9aa5.png" />

#### Bot control panel

The dashboard can also drive the bot - start/stop and manual market orders, mirroring the Telegram commands. Pass the bot controller to the chart before `Start`:

```go
bot, err := ninjabot.NewBot(ctx, settings, paperWallet, strategy, /* ... */)
if err != nil {
    log.Fatal(err)
}

// enable the control panel (start/stop and manual buy/sell)
chart.SetOrderController(bot.Controller())
```

Without a controller the endpoints report `{"enabled": false}` and the panel stays hidden. **Anyone with access to the dashboard port can operate the bot once this is set** - keep the port private (localhost, VPN or an authenticating proxy).

#### HTTP API

The JSON API is served even when the dashboard bundle is unavailable:

| Endpoint | Description |
|---|---|
| `GET /api/health` | Liveness probe. |
| `GET /api/pairs` | Pairs being traded. |
| `GET /api/{pair}/snapshot` | Candles, indicators, orders, equity curve and stats of a pair. |
| `GET /api/{pair}/orders.csv` | Orders of a pair as CSV. |
| `GET /api/events` | Server-Sent Events stream (`candle`, `order`, `controls`) for live updates. |
| `GET /api/controls` | `{"enabled": true, "status": "running"}` - control panel state. |
| `POST /api/controls/start` / `POST /api/controls/stop` | Resume or pause order placement. |
| `POST /api/controls/order` | Manual market order. |

`POST /api/controls/order` takes the amount in quote currency, or a percentage of the available balance (quote balance for buys, asset balance for sells):

```bash
curl -X POST localhost:8080/api/controls/order \
  -d '{"pair": "BTCUSDT", "side": "buy", "amount": 100}'

# 50% of the available balance
curl -X POST localhost:8080/api/controls/order \
  -d '{"pair": "BTCUSDT", "side": "sell", "amount": 50, "percent": true}'
```

Orders sent while the bot is stopped are rejected with `409 Conflict`.

### Indicators

`ui.WithStrategyIndicators(strategy)` plots the indicators declared by your strategy, and `ui.WithCustomIndicators` adds any of the built-ins from `ui/indicator`:

```go
chart, err := ui.New(
    ui.WithCustomIndicators(
        indicator.EMA(8, "red"),
        indicator.SMA(21, "blue"),
        indicator.RSI(14, "purple"),
        indicator.MACD(12, 26, 9, "red", "blue", "orange"),
        indicator.PPO(12, 26, 9, "red", "blue", "orange"),
        indicator.Stoch(8, 3, 3, "red", "blue"),
        indicator.BollingerBands(21, 2.0, "gray", "blue"),
        indicator.Spertrend(14, 3.0, "purple"),
        indicator.CCI(14, "red"),
        indicator.WillR(14, "red"),
        indicator.OBV("blue"),
    ),
)
```

To plot something that is not on the list, implement the `ui.CustomIndicator` interface (`Name`, `Overlay`, `Warmup`, `Metrics` and `Load`) - the files in [ui/indicator](ui/indicator) are short examples.

### Telegram and notifications

With `Telegram.Enabled` in the settings the bot answers on Telegram, and `ninjabot.WithNotifier` sends order and error notifications to any `service.Notifier` (Telegram or e-mail):

```go
settings := ninjabot.Settings{
    Pairs: []string{"BTCUSDT"},
    Telegram: ninjabot.TelegramSettings{
        Enabled: true,
        Token:   os.Getenv("TELEGRAM_TOKEN"),
        Users:   []int{telegramUser}, // only these users are allowed
    },
}
```

| Command | Description |
|---|---|
| `/help` | List the available commands. |
| `/status` | Bot status: `running` or `stopped`. |
| `/profit` | Summary of the trade results so far. |
| `/balance` | Wallet balance, with the value of the open positions. |
| `/start` / `/stop` | Resume or pause order placement. |
| `/buy BTCUSDT 100` | Market buy of 100 USDT (amount in quote currency). |
| `/buy BTCUSDT 50%` | Market buy with 50% of the available quote balance. |
| `/sell BTCUSDT 100` | Market sell of 100 USDT worth of the asset. |
| `/sell BTCUSDT 50%` | Market sell of 50% of the asset balance. |

While stopped, **every** new order is rejected - including the protective stops and take profits your strategy tries to place, so an open position is left unprotected until you send `/start`. Rejected orders are reported instead of failing silently.

E-mail notifications use `notification.NewMail`:

```go
mail := notification.NewMail(notification.MailParams{
    SMTPServerAddress: "smtp.gmail.com",
    SMTPServerPort:    587,
    From:              "from@example.com",
    To:                "to@example.com",
    Password:          os.Getenv("SMTP_PASSWORD"),
})

bot, err := ninjabot.NewBot(ctx, settings, exch, strategy, ninjabot.WithNotifier(mail))
```

### Short positions

The paper wallet supports short selling, so futures strategies can be backtested. A sell bigger than the asset balance opens a short: it is modelled as a **negative asset balance** backed by collateral taken from the quote balance, and the buy that covers it liquidates the position, crediting the profit (or debiting the loss) of the move:

```go
// with no BTC in the wallet, this opens a short of 1 BTC
_, err := bot.Controller().CreateOrderMarket(ninjabot.SideTypeSell, "BTCUSDT", 1)
```

The collateral is `price * quantity`, so a wallet with 200 USDT can short 2 BTC at 100 USDT - the quote balance goes to zero while the position is open, and covering it returns the collateral plus the profit of the move (2 BTC covered at 50 USDT return 300 USDT). Limit and stop orders open and close shorts the same way, locking the collateral until they fill.

An order that only partially covers an open short is settled in two parts - the covering share against the short, the remainder as a new long (or short) position - and the balance reported by `/balance`, the summary and the dashboard account for the open short at the current price.

### Storage

Orders are persisted so that pending orders survive a restart. The default is a local `ninjabot.db` file; the alternatives are:

```go
storage, err := storage.FromMemory()               // in memory, recommended for backtesting
storage, err := storage.FromFile("ninjabot.db")    // local file (BuntDB)
storage, err := storage.FromSQL(sqlite.Open("ninjabot.db")) // any GORM dialect (github.com/glebarez/sqlite here)

bot, err := ninjabot.NewBot(ctx, settings, exch, strategy, ninjabot.WithStorage(storage))
```

### Trading tools

**Trailing stop** - follows the price up and reports when the stop is hit:

```go
trailing := tools.NewTrailingStop()

// on a new position
trailing.Start(currentPrice, stopPrice)

// on every candle
if trailing.Active() && trailing.Update(candle.Close) {
    // stop hit, close the position
    trailing.Stop()
}
```

**Order scheduler** - places a market order as soon as a condition is met:

```go
scheduler := tools.NewScheduler("BTCUSDT")

// buy 1 BTC when the price drops below 30k
scheduler.BuyWhen(1, func(df *ninjabot.Dataframe) bool {
    return df.Close.Last(0) < 30000
})

// on every candle
scheduler.Update(df, broker)
```

A complete example is in [examples/strategies/trailingstop.go](examples/strategies/trailingstop.go).

### Exchange options

```go
// Spot
binance, err := exchange.NewBinance(ctx,
    exchange.WithBinanceCredentials(apiKey, secretKey),
    exchange.WithBinanceHeikinAshiCandle(), // trade with Heikin Ashi candles
    exchange.WithTestNet(),                 // Binance testnet, for safe experiments
)

// Futures
binanceFuture, err := exchange.NewBinanceFuture(ctx,
    exchange.WithBinanceFutureCredentials(apiKey, secretKey),
    exchange.WithBinanceFutureLeverage("BTCUSDT", 1, exchange.MarginTypeIsolated),
    exchange.WithBinanceFuturesHeikinAshiCandle(),
)
```

`exchange.WithCustomMainAPIEndpoint` and `exchange.WithCustomTestnetAPIEndpoint` point the REST and websocket clients somewhere else (a proxy or a mock server), and `exchange.WithMetadataFetcher` attaches custom metadata to every candle.

### Features

|                    	| Binance Spot 	| Binance Futures 	 |
|--------------------	|--------------	|-------------------|
| Order Market       	|       :ok:      	| :ok:              |
| Order Market Quote 	|       :ok:      	| 	                 |
| Order Limit        	|       :ok:      	| :ok:              |
| Order Stop         	|       :ok:      	| :ok:              |
| Order OCO          	|       :ok:     	| 	                 |
| Backtesting        	|       :ok:     	| :ok:         	    |

- [x] Backtesting
  - [x] Paper Wallet (Live Trading with fake wallet)
  - [x] Load Feed from CSV
  - [x] Order Limit, Market, Stop Limit, OCO
  - [x] Maker / taker trading fees
  - [x] Short positions

- [x] Bot Utilities
  - [x] CLI to download historical data
  - [x] Web dashboard (Candles + Sell / Buy orders, Indicators, equity curve and stats)
  - [x] Web UI controller (start / stop and manual orders)
  - [x] Telegram Controller (Status, Balance, Buy, Sell, and Notification)
  - [x] E-mail notifications
  - [x] Heikin Ashi candle type support
  - [x] Trailing stop tool
  - [x] In app order scheduler
  - [x] Storage in memory, local file or SQL

### Roadmap
  - [ ] Include more chart indicators - [Details](https://github.com/rodrigo-brito/ninjabot/issues/110)

### Exchanges

Currently, we only support [Binance](https://www.binance.com/en?ref=35723227) exchange. If you want to include support for other exchanges, you need to implement a new `struct` that implements the interface `Exchange`. You can check some examples in [exchange](exchange) directory.

### Support the project

|  | Address  |
| --- | --- |
|**BTC** | `bc1qpk6yqju6rkz33ntzj8kuepmynmztzydmec2zm4`|
|**ETH** | `0x2226FFe4aBD2Afa84bf7222C2b17BBC65F64555A` |
|**LTC** | `ltc1qj2n9r4yfsm5dnsmmtzhgj8qcj8fjpcvgkd9v3j` |

**Patreon**: https://www.patreon.com/ninjabot_github
