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
| ETHUSDT |      8 |   6 |    2 | 75.0 % | 12.215 | 5641.9314 |
| BTCUSDT |     12 |   5 |    7 | 41.7 % |  5.137 | 3373.9726 |
+---------+--------+-----+------+--------+--------+-----------+
|   TOTAL |     20 |  11 |    9 | 55.0 % |  7.968 | 9015.9040 |
+---------+--------+-----+------+--------+--------+-----------+
--------------
WALLET SUMMARY
--------------
0.000000 BTC
1.723774 ETH
15015.904020 USDT
--------------
START PORTFOLIO =  10000 USDT
FINAL PORTFOLIO =  19015.904019955597 USDT
GROSS PROFIT    =  9015.904020 USDT (90.16%)
MARKET CHANGE   =  396.71%
--------------
Chart available at http://localhost:8080
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
  - [ ] Stop Orders
  
- [x] Bot CLI - Utilities to support studies
  - [x] Download
  - [x] Plot (Candles + Sell / Buy orders)

### Exchanges:

Currently, we only support Binance exchange. If you want to include support for other exchanges, you need to implement a new `struct` that implements the interface [`Exchange`](https://github.com/rodrigo-brito/ninjabot/blob/main/pkg/exchange/exchange.go#L22-L41). You can check some examples in [exchange](./pkg/exchange) directory.
