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
---------+--------+-----+------+--------+--------+-----+----------+-----------+
|  PAIR   | TRADES | WIN | LOSS | % WIN  | PAYOFF | SQN |  PROFIT  |  VOLUME   |
+---------+--------+-----+------+--------+--------+-----+----------+-----------+
| ETHUSDT |      9 |   6 |    3 | 66.7 % |  3.407 | 1.3 | 21748.41 | 407769.64 |
| BTCUSDT |     14 |   6 |    8 | 42.9 % |  5.929 | 1.5 | 13511.66 | 448030.05 |
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
BTCUSDT         = 448030.05 USDT
ETHUSDT         = 407769.64 USDT
TOTAL           = 855799.68 USDT
-------------------
Dashboard available at http://localhost:8080

```

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
