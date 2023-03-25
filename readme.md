![Ninjabot](https://user-images.githubusercontent.com/7620947/161434011-adc89d1a-dccb-45a7-8a07-2bb55e62d2d9.png)

[![tests](https://github.com/rodrigo-brito/ninjabot/actions/workflows/ci.yaml/badge.svg)](https://github.com/rodrigo-brito/ninjabot/actions/workflows/ci.yaml)
[![codecov](https://codecov.io/gh/rodrigo-brito/ninjabot/branch/main/graph/badge.svg)](https://codecov.io/gh/rodrigo-brito/ninjabot)
[![Go Reference](https://pkg.go.dev/badge/github.com/rodrigo-brito/ninjabot.svg)](https://pkg.go.dev/github.com/rodrigo-brito/ninjabot)
[![Discord](https://img.shields.io/discord/960156400376483840?color=5865F2&label=discord)](https://discord.gg/TGCrUH972E)
[![Discord](https://img.shields.io/badge/donate-patreon-red)](https://www.patreon.com/ninjabot_github)

A fast cryptocurrency trading bot framework implemented in Go. Ninjabot permits users to create and test custom strategies for spot markets. 

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
INFO[2023-03-25 12:59] [SETUP] Using paper wallet                   
INFO[2023-03-25 12:59] [SETUP] Initial Portfolio = 10000.000000 USDT 
 100% |█████████████████████████████████████████████████████████████████████████████████████████████████████████████████████████████████████████████████████████████████████████████████████████████████████████████████████████████████| (8626/8626, 85622 it/s)        
+---------+--------+-----+------+--------+--------+-----+----------+-----------+
|  PAIR   | TRADES | WIN | LOSS | % WIN  | PAYOFF | SQN |  PROFIT  |  VOLUME   |
+---------+--------+-----+------+--------+--------+-----+----------+-----------+
| ETHUSDT |     40 |  33 |    7 | 82.5 % |  1.648 | 2.3 |  9061.96 | 209061.96 |
| BTCUSDT |     42 |  30 |   12 | 71.4 % |  1.461 | 1.5 |  5377.25 | 215377.25 |
+---------+--------+-----+------+--------+--------+-----+----------+-----------+
|   TOTAL |     82 |  63 |   19 | 76.8 % |  1.553 | 1.9 | 14439.21 | 424439.21 |
+---------+--------+-----+------+--------+--------+-----+----------+-----------+

-- FINAL WALLET --
0.0000 BTC = 0.0000 USDT
0.0000 ETH = 0.0000 USDT
24439.2081 USDT

----- RETURNS -----
START PORTFOLIO     = 10000.00 USDT
FINAL PORTFOLIO     = 24439.21 USDT
GROSS PROFIT        =  14439.208081 USDT (144.39%)
MARKET CHANGE (B&H) =  407.09%

------ RISK -------
MAX DRAWDOWN = -12.30 %

------ VOLUME -----
BTCUSDT         = 215377.25 USDT
ETHUSDT         = 209061.96 USDT
TOTAL           = 424439.21 USDT
-------------------
Chart available at http://localhost:8080
```

### Plot result

<img width="100%"  src="https://user-images.githubusercontent.com/7620947/139601478-7b1d826c-f0f3-4766-951e-b11b1e1c9aa5.png" />

### Features

|                    	| Binance Spot 	| Binance Futures 	|
|--------------------	|--------------	|----------------	    |
| Order Market       	|       :ok:      	|        :ok:       |
| Order Market Quote 	|       :ok:      	|                 	|
| Order Limit        	|       :ok:      	|        :ok:       |
| Order Stop         	|       :ok:      	|        :ok:       |
| Order OCO          	|       :ok:     	|                 	|
| Backtesting        	|       :ok:     	|        :construction:         	|

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
  - [ ] Support future market - [Details](https://github.com/rodrigo-brito/ninjabot/issues/106)

### Exchanges

Currently, we only support [Binance](https://www.binance.com/en?ref=35723227) exchange. If you want to include support for other exchanges, you need to implement a new `struct` that implements the interface `Exchange`. You can check some examples in [exchange](./pkg/exchange) directory.

### Support the project

|  | Address  |
| --- | --- |
|**BTC** | `bc1qpk6yqju6rkz33ntzj8kuepmynmztzydmec2zm4`|
|**ETH** | `0x2226FFe4aBD2Afa84bf7222C2b17BBC65F64555A` |
|**LTC** | `ltc1qj2n9r4yfsm5dnsmmtzhgj8qcj8fjpcvgkd9v3j` |

**Patreon**: https://www.patreon.com/ninjabot_github
