# Ninja Bot

[![tests](https://github.com/rodrigo-brito/ninjabot/actions/workflows/ci.yaml/badge.svg)](https://github.com/rodrigo-brito/ninjabot/actions/workflows/ci.yaml)
[![Go Reference](https://pkg.go.dev/badge/github.com/rodrigo-brito/ninjabot.svg)](https://pkg.go.dev/github.com/rodrigo-brito/ninjabot)

A fast cryptocurrency bot implemented in Go

:warning: **Caution:** Working in progress :construction:

## Instalation

`go get -u github.com/rodrigo-brito/ninjabot`

## Example of Usage

Check [example](example) directory:

- Paper Wallet (Live Simultation)
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
