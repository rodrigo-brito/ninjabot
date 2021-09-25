---
title: "Streategy"
linkTitle: "Strategy"
categories: ["Examples", "Guides"]
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
	Indicators(dataframe *model.Dataframe)
	OnCandle(dataframe *model.Dataframe, broker service.Broker)
}
```

- `Timeframe`: specifies the strategy timeframe, eg: "15m", "1h", "1d", "1w".
- `WarmupPeriod`: specifies the number of candles necessary to pre-load before the bot start. For example, if you use a 9-period moving average strategy, the `WarmupPeriod` should be 9.
- `Indicators`: this function creates custom indicators, it is called for each new candle received.
- `OnCandle`: this function is also called for each new candle, after `Indicators` execution. This function should contain your buy and sell rules. `Dataframe` object contains indicators and indicators from candles. The buy and sell operations can be performed through the `Broker` operator.

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
	return "1d"
}

func (e CrossEMA) WarmupPeriod() int {
	return 9
}

func (e CrossEMA) Indicators(df *ninjabot.Dataframe) {
	// define a custom indicator, Exponential Moving Average of 9 periods
	df.Metadata["ema9"] = talib.Ema(df.Close, 9)
}

func (e *CrossEMA) OnCandle(df *ninjabot.Dataframe, broker service.Broker) {
	// Get the quote and assets information
	assetPosition, quotePosition, err := broker.Position(df.Pair)
	if err != nil {
		log.Error(err)
	}
	
	// Check if we have more than 10 USDT available in the wallet and the buy signal is triggered
	if quotePosition > 10 && df.Close.Crossover(df.Metadata["ema9"]) {
		_, err := broker.CreateOrderMarketQuote(ninjabot.SideTypeBuy, df.Pair, quotePosition/2)
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
