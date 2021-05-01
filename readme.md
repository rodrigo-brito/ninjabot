# Ninja Bot

A fast cryptocurrency bot implemented in Go

:warning: **Caution:** Working in progress :construction:

## Instalation

`go get -u github.com/rodrigo-brito/ninjabot`

## Example of Usage

Check example folder for a complete example:

```bash
go run example/main.go
```

### CLI

- `ninjabot download` - Download historical data
    - Example: `ninjabot download --symbol BTCUSDT --timeframe 1h --limit 100 --output ./btc.csv`


## Roadmap

### Features:
- [x] Exchange Feed Data
- [x] Custom Strategy
- [x] Order Management
    - [ ] Update Status
    - [ ] Order report
- [ ] Strategy Backtesting
- [x] Bot CLI
  - [x] Download
  - [ ] Plot

### Exchanges:
- [x] Binance
- [ ] FTX
