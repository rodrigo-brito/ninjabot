# Ninja Bot

A fast cryptocurrency bot implemented in Go

:warning: **Caution:** Working in progress :construction:

Features:
- [x] Exchange Feed Data
- [x] Custom Strategy
- [ ] Order Management
- [ ] Strategy Backtesting

Exchanges:
- [x] Binance
- [ ] FTX

## Instalation

`go get -u github.com/rodrigo-brito/ninjabot`

## Example of Usage

Check example folder for a complete example:

```bash
go run example/main.go
```

```go
type Example struct{}

func (e Example) Init(settings ninjabot.Settings) {}

func (e Example) Timeframe() string {
	return "1m"
}

func (e Example) WarmupPeriod() int {
	return 14
}

func (e Example) Indicators(dataframe *ninjabot.Dataframe) {
	dataframe.Metadata["rsi"] = talib.Rsi(dataframe.Close, 14)
	dataframe.Metadata["ema"] = talib.Ema(dataframe.Close, 9)
}

func (e Example) OnCandle(dataframe *ninjabot.Dataframe, broker ninjabot.Broker) {
	fmt.Println("New Candle = ", dataframe.LastUpdate, ninjabot.Last(dataframe.Close, 0))

	if ninjabot.Last(dataframe.Metadata["rsi"], 0) < 30 {
		broker.OrderMarket(ninjabot.BuyOrder, dataframe.Pair, 1)
	}

	if ninjabot.Last(dataframe.Metadata["rsi"], 0) > 70 {
		broker.OrderMarket(ninjabot.SellOrder, dataframe.Pair, 1)
	}
}
```