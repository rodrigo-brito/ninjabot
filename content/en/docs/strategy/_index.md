---
title: "Streategy"
linkTitle: "Custom Strategy"
categories: ["Reference"]
weight: 2
description: >
  This page describes how to create a custom strategy in Ninjabot
---

## Strategy Functions

To create a custom strategy, you need to create a `Struct` that implements the following methods:

```go
type Strategy interface {
	Timeframe() string
	WarmupPeriod() int
	Indicators(dataframe *model.Dataframe) []ChartIndicator
	OnCandle(dataframe *model.Dataframe, broker service.Broker)
	OnPartialCandle(df *model.Dataframe, broker service.Broker) // Optional
}
```

- `Timeframe`: specifies the strategy timeframe, eg: "15m", "1h", "1d", "1w".
- `WarmupPeriod`: specifies the number of candles necessary to pre-load before the bot start. For example, if you use a 9-period moving average strategy, the `WarmupPeriod` should be 9.
- `Indicators`: this function creates custom indicators, it is called for each new candle received. You can also return a list of indicators to display in the chart.
- `OnCandle`: this function is also called for each new **closed candle**, after `Indicators` execution. This function should contain your buy and sell rules. `Dataframe` object contains indicators and indicators from candles. The buy and sell operations can be performed through the `Broker` operator.
- `OnPartialCandle`: this functions is optional, it will be called with high frequency, usually called every 2 seconds with partial data of current candle.

## Example

The following code presents a strategy with a single indicator. We defined an Exponential Moving Average (EMA) of 9 periods. For each candle, we create a buy order when the price closes above the EMA, and sell when the price closes under the EMA.

```go
import (
    "github.com/rodrigo-brito/ninjabot"
    "github.com/rodrigo-brito/ninjabot/service"

    "github.com/markcheno/go-talib"
    log "github.com/sirupsen/logrus"
)

type CrossEMA struct{}

func (e CrossEMA) Timeframe() string {
	return "1d" // examples: 1m, 5m, 15m, 30m, 1h, 4h, 12h, 1d, 1w
}

func (e CrossEMA) WarmupPeriod() int {
	return 9 // warmup period, to preload indicators
}

func (e CrossEMA) Indicators(df *ninjabot.Dataframe) []strategy.ChartIndicator {
	// define a custom indicator, Exponential Moving Average of 9 periods
	df.Metadata["ema9"] = talib.Ema(df.Close, 9)


	// (Optional) you can return a list of indicators to include in the final chart
	return []strategy.ChartIndicator{
		{
			Overlay:   true,
			Time:      df.Time,
			GroupName: "EMA",
			Metrics: []strategy.IndicatorMetric{
				{
					Values: df.Metadata["ema9"],
					Name:   "EMA 9",
					Color:  "red",
					Style:  strategy.StyleLine,
				},
			},
		},
	}
}

func (e *CrossEMA) OnCandle(df *ninjabot.Dataframe, broker service.Broker) {
	// Get the quote and assets information
	assetPosition, quotePosition, err := broker.Position(df.Pair)
	if err != nil {
		log.Error(err)
	}

	// Check if we have more than 10 USDT available in the wallet and the buy signal is triggered
	if quotePosition > 10 && df.Close.Crossover(df.Metadata["ema9"]) {
		_, err := broker.CreateOrderMarketQuote(ninjabot.SideTypeBuy, df.Pair, quotePosition*0.99)
		if err != nil {
			log.Error(err)
		}
	}

	// Check if we have position in the pair and the sell signal is triggered
	if assetPosition > 0 &&
		df.Close.Crossunder(df.Metadata["ema9"]) {
		_, err := broker.CreateOrderMarket(ninjabot.SideTypeSell, df.Pair, assetPosition)
		if err != nil {
			log.Error(err)
		}
	}
}
```

### Heikin Ashi candle type support

<img width="100%"  src="https://i.ibb.co/N6sTVd6/Screenshot-2022-05-01-at-08-02-05.png" />

- CSV Feed exchange
  ```go
  csvFeed, err := exchange.NewCSVFeed(
      strategy.Timeframe(),
      exchange.PairFeed{
          Pair:      "FTMUSDT",
          File:      "testdata/ftm-1d.csv",
          Timeframe: "1d",
          HeikinAshi: true,
  },
  ```
- Binance
  ```go
  binance, err := exchange.NewBinance(ctx, exchange.WithBinanceHeikinAshiCandle())
  ```

### High Frequency Trading (HFT)

You also have access to partial candle updates through the function `OnPartialCandle`. This can be useful for handling high frequency logic, such as using trailing stop or scalping techniques. See an example of usage below. This function is usually called every 2 seconds and may have small time variations.

```go
func (e *CrossEMA) OnPartialCandle(df *model.Dataframe, broker service.Broker) {
	// my logic here...
}
```
