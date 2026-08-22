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

## Installation

`go get -u github.com/rodrigo-brito/ninjabot/...`

## Examples of Usage

Check [examples](examples) directory:

- Paper Wallet (Live Simulation)
- Backtesting (Simulation with historical data)
- Real Account (Binance)

### CLI

To download historical data you can download ninjabot CLI from:

- Pre-build binaries in [release page](https://github.com/rodrigo-brito/ninjabot/releases)
- Or with `go install github.com/rodrigo-brito/ninjabot/cmd/ninjabot@latest`

**Example of usage**
```bash
# Download candles of BTCUSDT to btc.csv file (Last 30 days, timeframe 1D)
ninjabot download --pair BTCUSDT --timeframe 1d --days 30 --output ./btc.csv
```

### Backtesting Example

- Backtesting a custom strategy from [examples](examples) directory:
```
go run examples/backtesting/main.go
```

Output:

```
INFO[2023-03-25 13:54] [SETUP] Using paper wallet                   
INFO[2023-03-25 13:54] [SETUP] Initial Portfolio = 10000.000000 USDT 
+---------+--------+-----+------+--------+--------+----------+-----+----------+-----------+
|  PAIR   | TRADES | WIN | LOSS | % WIN  | PAYOFF | PR FACT. | SQN |  PROFIT  |  VOLUME   |
+---------+--------+-----+------+--------+--------+----------+-----+----------+-----------+
| BTCUSDT |     14 |   6 |    8 | 42.9 % |  5.813 |    4.360 | 1.4 | 12756.38 | 437924.95 |
| ETHUSDT |     12 |   6 |    6 | 50.0 % |  6.662 |    6.662 | 1.3 | 20468.89 | 394148.32 |
+---------+--------+-----+------+--------+--------+----------+-----+----------+-----------+
|   TOTAL |     26 |  12 |   14 | 46.2 % |  6.205 |    5.423 | 1.3 | 33225.27 | 832073.28 |
+---------+--------+-----+------+--------+--------+----------+-----+----------+-----------+

----- FINAL WALLET -----
0.0000 BTC = 0.0000 USDT
0.0000 ETH = 0.0000 USDT
43225.2687 USDT

----- RETURNS -----
START PORTFOLIO     = 10000.00 USDT
FINAL PORTFOLIO     = 43225.27 USDT
GROSS PROFIT        =  34057.342011 USDT (340.57%)
TRADING FEES        =  832.073277 USDT
NET PROFIT          =  33225.268735 USDT (332.25%)
MARKET CHANGE (B&H) =  407.09%

------ RISK -------
MAX DRAWDOWN = -11.76 %

------ VOLUME -----
BTCUSDT         = 437924.95 USDT
ETHUSDT         = 394148.32 USDT
TOTAL           = 832073.28 USDT
-------------------
Dashboard available at http://localhost:8080

```

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

If the bundle cannot be fetched, the bot keeps running, the JSON API (`/api/pairs`, `/api/{pair}/snapshot`, `/api/events`) stays up and `http://localhost:8080` explains how to fix it.

<img width="100%"  src="https://user-images.githubusercontent.com/7620947/139601478-7b1d826c-f0f3-4766-951e-b11b1e1c9aa5.png" />

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

- [x] Bot Utilities
  - [x] CLI to download historical data
  - [x] Plot (Candles + Sell / Buy orders, Indicators)
  - [x] Telegram Controller (Status, Buy, Sell, and Notification)
  - [x] Heikin Ashi candle type support
  - [x] Trailing stop tool
  - [x] In app order scheduler

# Roadmap
  - [ ] Include Web UI Controller
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
