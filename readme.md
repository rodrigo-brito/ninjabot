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

- `ninjabot download` - Download historical data
    - Example: `ninjabot download --symbol BTCUSDT --timeframe 1h --limit 100 --output ./btc.csv`


## Roadmap

### Features:
- [x] Order Limit, Market, OCO, and Stop
- [x] Custom Strategy
- [x] Paper Wallet
- [x] Strategy Backtesting (Only for market orders)
- [x] Bot CLI
  - [x] Download
  - [ ] Plot

### Exchanges:
- [x] Binance
