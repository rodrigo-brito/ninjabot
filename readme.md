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

To download historical data you can download ninjabot CLI from [release page]

#### Commands
```text
NAME:
   download - download historical data

USAGE:
   cli [global options] command [command options] [arguments...]

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --pair value, -p value       eg. BTCUSDT
   --days value, -d value       eg. 100 (default 30 days) (default: 0)
   --start value, -s value      eg. 2021-12-01
   --end value, -e value        eg. 2020-12-31
   --timeframe value, -t value  eg. 1h
   --output value, -o value     eg. ./btc.csv
   --help, -h                   show help (default: false)
```
#### Examples

- Download 30 days: `ninjabot download --pair BTCUSDT --timeframe 1d --days 30 --output ./btc.csv`

### Backtesting Example

- Backtesting from [example](example) directory:
```
go run example/backtesting/main.go
```

Output:

```
INFO[2021-05-16 13:22] [SETUP] Using paper wallet                   
INFO[2021-05-16 13:22] [SETUP] Initial Portfolio = 10000.000000 USDT 
+---------+--------+-----+------+--------+--------+------------+
|  PAIR   | TRADES | WIN | LOSS | % WIN  | PAYOFF |   PROFIT   |
+---------+--------+-----+------+--------+--------+------------+
| BTCUSDT |      9 |   4 |    5 | 44.4 % |  1.080 | 10074.0928 |
+---------+--------+-----+------+--------+--------+------------+
|   TOTAL |      9 |   4 |    5 | 44.4 % |  1.080 | 10074.0928 |
+---------+--------+-----+------+--------+--------+------------+
--------------
WALLET SUMMARY
--------------
0.000000 BTC
13757.142338 USDT
--------------
START PORTFOLIO =  10000 USDT
FINAL PORTFOLIO =  13757.142338196232 USDT
PROFIT = 3757.142338 USDT (37.57%)
--------------

```

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
  - [ ] Plot

### Exchanges:
- [x] Binance
