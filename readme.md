# Ninja Bot

[![tests](https://github.com/rodrigo-brito/ninjabot/actions/workflows/ci.yaml/badge.svg)](https://github.com/rodrigo-brito/ninjabot/actions/workflows/ci.yaml)
[![Go Reference](https://pkg.go.dev/badge/github.com/rodrigo-brito/ninjabot.svg)](https://pkg.go.dev/github.com/rodrigo-brito/ninjabot)

A fast cryptocurrency bot implemented in Go

:warning: **Caution:** Working in progress :construction:

## Instalation

`go get -u github.com/rodrigo-brito/ninjabot`

## Example of Usage

Check [example](example) directory:

- Paper Wallet (Live Simulation)
- Backtesting
- Real Account (Binance)

### CLI

To download historical data you can download ninjabot CLI from [release page](https://github.com/rodrigo-brito/ninjabot/releases)
- Download 30 days: `ninjabot download --pair BTCUSDT --timeframe 1d --days 30 --output ./btc.csv`

### Backtesting Example

- Backtesting from [example](example) directory:
```
go run example/backtesting/main.go
```

Output:

```
[SETUP] Using paper wallet                   
[SETUP] Initial Portfolio = 10000.000000 USDT 
+---------+--------+-----+------+--------+--------+-----------+
|  PAIR   | TRADES | WIN | LOSS | % WIN  | PAYOFF |  PROFIT   |
+---------+--------+-----+------+--------+--------+-----------+
| BTCUSDT |     12 |   5 |    7 | 41.7 % |  5.137 | 4217.4657 |
+---------+--------+-----+------+--------+--------+-----------+
|   TOTAL |     12 |   5 |    7 | 41.7 % |  5.137 | 4217.4657 |
+---------+--------+-----+------+--------+--------+-----------+
--------------
WALLET SUMMARY
--------------
0.000000 BTC
14217.465729 USDT
--------------
START PORTFOLIO =  10000 USDT
FINAL PORTFOLIO =  14217.465729229527 USDT
PROFIT = 4217.465729 USDT (42.17%)
--------------
```

### Plot result:

<img width="500"  src="https://user-images.githubusercontent.com/7620947/118583297-38f69580-b76b-11eb-8a7f-ad3999541cac.png"/>

### Roadmap:

- [x] Live Trading
  - [x] Order Limit, Market, OCO, and Stop
  - [x] Custom Strategy

- [x] Backtesting
  - [x] Paper Wallet (Live Trading with fake wallet)
  - [x] Load Feed from CSV
  - [x] Market Orders
  - [x] Limit Orders
  - [ ] OCO Orders
  
- [x] Bot CLI - Utilities to support studies
  - [x] Download
  - [x] Plot (Candles + Sell / Buy orders)

### Exchanges:
- [x] Binance
