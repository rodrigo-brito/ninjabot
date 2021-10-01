![image](https://user-images.githubusercontent.com/7620947/119247309-b69d1580-bb5e-11eb-9d81-4495dfc45f21.png)

[![tests](https://github.com/rodrigo-brito/ninjabot/actions/workflows/ci.yaml/badge.svg)](https://github.com/rodrigo-brito/ninjabot/actions/workflows/ci.yaml)
[![codecov](https://codecov.io/gh/sudiptog81/ninjabot/branch/main/graph/badge.svg?token=VfrjGMzXC5)](https://codecov.io/gh/sudiptog81/ninjabot)
[![Go Reference](https://pkg.go.dev/badge/github.com/rodrigo-brito/ninjabot.svg)](https://pkg.go.dev/github.com/rodrigo-brito/ninjabot)

A fast cryptocurrency trading bot framework implemented in Go. Ninjabot permits users to create and test custom strategies for spot markets. 

Documentation: https://rodrigo-brito.github.io/ninjabot/

:warning: **Caution:** Working in progress - It's not production ready :construction:

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
INFO[2021-09-29 00:00] [SETUP] Using paper wallet                   
INFO[2021-09-29 00:00] [SETUP] Initial Portfolio = 10000.000000 USDT 
finished
+---------+--------+-----+------+--------+--------+----------+-----------+
|  PAIR   | TRADES | WIN | LOSS | % WIN  | PAYOFF |  PROFIT  |  VOLUME   |
+---------+--------+-----+------+--------+--------+----------+-----------+
| BTCUSDT |     17 |   6 |   11 | 35.3 % |  7.038 |  7424.37 | 250246.73 |
| ETHUSDT |     17 |   9 |    8 | 52.9 % |  7.400 |  9270.30 | 168350.93 |
+---------+--------+-----+------+--------+--------+----------+-----------+
|   TOTAL |     34 |  15 |   19 | 44.1 % |  7.219 | 16694.67 | 418597.66 |
+---------+--------+-----+------+--------+--------+----------+-----------+

--------------
WALLET SUMMARY
--------------
0.000000 ETH
0.000000 BTC

TRADING VOLUME
ETHUSDT        = 185030.63 USDT
BTCUSDT        = 255182.59 USDT

26694.674186 USDT
--------------
START PORTFOLIO =  10000 USDT
FINAL PORTFOLIO =  26694.674186473057 USDT
GROSS PROFIT    =  16694.674186 USDT (166.95%)
MARKET CHANGE   =  420.18%
VOLUME          =  440213.22 USDT
COSTS (0.001*V) =  440.21 USDT (ESTIMATION) 
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
  - [ ] Include trailing stop tool
  - [ ] Stop Orders in backtesting
  - [ ] Plot Indicators

### Exchanges:

Currently, we only support [Binance](https://www.binance.com/en?ref=35723227) exchange. If you want to include support for other exchanges, you need to implement a new `struct` that implements the interface `Exchange`. You can check some examples in [exchange](./pkg/exchange) directory.
