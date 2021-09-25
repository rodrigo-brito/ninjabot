![image](https://user-images.githubusercontent.com/7620947/119247309-b69d1580-bb5e-11eb-9d81-4495dfc45f21.png)

[![tests](https://github.com/rodrigo-brito/ninjabot/actions/workflows/ci.yaml/badge.svg)](https://github.com/rodrigo-brito/ninjabot/actions/workflows/ci.yaml)
[![Go Reference](https://pkg.go.dev/badge/github.com/rodrigo-brito/ninjabot.svg)](https://pkg.go.dev/github.com/rodrigo-brito/ninjabot)

A fast cryptocurrency trading bot framework implemented in Go. Ninjabot permits users to create and test custom strategies for spot markets. 

Documentation: https://rodrigo-brito.github.io/ninjabot/

:warning: **Caution:** Working in progress - It's not production ready :construction:

## Installation

`go get -u github.com/rodrigo-brito/ninjabot/...`

## Examples of Usage

Check [examples](examples) directory:

- Paper Wallet (Live Simulation)
- Backtesting
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
[SETUP] Using paper wallet                   
[SETUP] Initial Portfolio = 10000.000000 USDT 
+---------+--------+-----+------+--------+--------+------------+
|  PAIR   | TRADES | WIN | LOSS | % WIN  | PAYOFF |   PROFIT   |
+---------+--------+-----+------+--------+--------+------------+
| ETHUSDT |     19 |   9 |   10 | 47.4 % |  6.975 |  6334.1268 |
| BTCUSDT |     17 |   6 |   11 | 35.3 % |  7.734 |  4803.0181 |
+---------+--------+-----+------+--------+--------+------------+
|   TOTAL |     36 |  15 |   21 | 41.7 % |  7.333 | 11137.1449 |
+---------+--------+-----+------+--------+--------+------------+
--------------
WALLET SUMMARY
--------------
0.000000 ETH
0.000000 BTC
21137.144920 USDT
--------------
START PORTFOLIO =  10000 USDT
FINAL PORTFOLIO =  21137.14492013396 USDT
GROSS PROFIT    =  11137.144920 USDT (111.37%)
MARKET CHANGE   =  396.71%

--------------
Chart available at http://localhost:8080
```

### Plot result:

<img width="500"  src="https://user-images.githubusercontent.com/7620947/118583297-38f69580-b76b-11eb-8a7f-ad3999541cac.png" />

### Features:

- [x] Live Trading
  - [x] Custom Strategy
  - [x] Order Limit, Market, OCO

- [x] Backtesting
  - [x] Paper Wallet (Live Trading with fake wallet)
  - [x] Load Feed from CSV
  - [x] Order Limit, Market, OCO

- [x] Bot Utilities
  - [x] CLI to download historical data
  - [x] Plot (Candles + Sell / Buy orders)
  - [x] Telegram Controller (Status, Buy, Sell)


# Roadmap
  - [ ] Stop Orders in Backtesting
  - [ ] Plot Indicators

### Exchanges:

Currently, we only support [Binance](https://www.binance.com/en?ref=35723227) exchange. If you want to include support for other exchanges, you need to implement a new `struct` that implements the interface `Exchange`. You can check some examples in [exchange](./pkg/exchange) directory.
