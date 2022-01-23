---
title: "Plotting"
linkTitle: "Plotting"
categories: ["Reference"]
weight: 2
description: >
    This page describes how to set up Ninjabot chart.
---

## Basic Usage

To create a chart, you need to import `plot` packages.

```go
import (
	"github.com/rodrigo-brito/ninjabot/plot"
	"github.com/rodrigo-brito/ninjabot/plot/indicator"
)
```

Then, you can create a chart using `plot.NewChart`. The following example creates a chart with 3 indicators. To include indicators, you must pass the options `WithIndicators`, in which receives one or more idicators.

Currently, Ninjabot supports the following indicators in charts:

- Exponential Moving Average (EMA)
- Relative Strength Index (RSI)
- Stochastic Oscillator (STOCH)
- Bollinger Bands
- Supertrend

For each indicator, you need to inform the parameters that are necessary and colors. We accept the color name and HEX code as bellow.
```go
chart, err := plot.NewChart(
	plot.WithIndicators( // Optional parameter to include indicators
        indicator.EMA(8, "red"),
        indicator.EMA(21, "#000"),
        indicator.RSI(14, "purple"),
        indicator.Stoch(8, 3, 3, "red", "blue"),
    ),
    plot.WithPaperWallet(wallet), // Optional parameter to include portfolio results (drawdown, equity evolution, etc)
    plot.WithPort(8080), // Optional parameter to customize the port number
)
if err != nil {
    log.Fatal(err)
}
```

Then, we need to connect our chart to Ninjabot data feed. The chart needs to receive candles and orders processed by ninjabot. We use a pattern called `pub/sub`. Then, to receive this data, we need to include the chart object in the **Order Subscription** and **Candle Subscription**

```go
bot, err := ninjabot.NewBot(
    ctx,
    settings,
    wallet,
    strategy,
    ninjabot.WithBacktest(wallet),
    ninjabot.WithStorage(storage),
    ninjabot.WithLogLevel(log.WarnLevel),
    
    // chart settings
    ninjabot.WithCandleSubscription(chart),
    ninjabot.WithOrderSubscription(chart),
)
```

In this way, when Ninjabot receives a candle or process an order, it will be sent to the chart. Finally, we need to start the bot. This command will start a HTTP server and display the result in the browser.
The default address are `http://localhost:8080`. But you can change it by passing the `plot.WithPort(portNumber)` option.

```go
err := chart.Start()
if err != nil {
    log.Fatal(err)
}
```

### Final Result

![Chart Result](https://user-images.githubusercontent.com/7620947/150690553-1d1db358-2b05-42eb-8909-2bf254a2460b.png)

## Custom Indicators

You can create custom indicators. An indicator is a `struct` that implements the `plot.Indicator` interface.

```go
type Indicator interface {
	Name() string // indicator name
	Overlay() bool // set if the indicator overlay the candlestick chart
	Metrics() []IndicatorMetric // returns the indicator metrics (lines, bars, etc) and styles
	Load(dataframe *model.Dataframe) // constructor that initialize the indicator with a dataframe
}
```

The following example creates a custom indicator called `EMA`.

```go
package indicator

import (
	"fmt"
	"time"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/plot"

	"github.com/markcheno/go-talib"
)

func EMA(period int, color string) plot.Indicator {
	return &ema{
		Period: period,
		Color:  color,
	}
}

type ema struct {
	Period int
	Color  string
	Values model.Series
	Time   []time.Time
}

func (e ema) Name() string {
	return fmt.Sprintf("EMA(%d)", e.Period)
}

func (e ema) Overlay() bool {
	return true
}

func (e *ema) Load(dataframe *model.Dataframe) {
	if len(dataframe.Time) < e.Period {
		return
	}

	e.Values = talib.Ema(dataframe.Close, e.Period)[e.Period:]
	e.Time = dataframe.Time[e.Period:]
}

func (e ema) Metrics() []plot.IndicatorMetric {
	return []plot.IndicatorMetric{
		{
			Style:  "line",
			Color:  e.Color,
			Values: e.Values,
			Time:   e.Time,
		},
	}
}
```
