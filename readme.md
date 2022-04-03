![image](https://user-images.githubusercontent.com/7620947/119247309-b69d1580-bb5e-11eb-9d81-4495dfc45f21.png)

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

### Plot result:

<img width="100%"  src="https://user-images.githubusercontent.com/7620947/139601478-7b1d826c-f0f3-4766-951e-b11b1e1c9aa5.png" />

### Features:

- [x] Live Trading
  - [x] Custom Strategy
  - [x] Order Limit, Market, Stop Limit, OCO

- [x] Backtesting
  - [x] Paper Wallet (Live Trading with fake wallet)
  - [x] Load Feed from CSV
  - [x] Order Limit, Market, Stop Limit, OCO

- [x] Bot Utilities
  - [x] CLI to download historical data
  - [x] Plot (Candles + Sell / Buy orders, Indicators)
  - [x] Telegram Controller (Status, Buy, Sell, and Notification)


# Roadmap
  - [ ] Include Web UI Controller
  - [ ] Include trailing stop tool
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
