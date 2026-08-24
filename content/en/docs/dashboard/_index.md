---
title: "Dashboard"
linkTitle: "Dashboard"
categories: ["Reference"]
weight: 4
aliases:
  - /docs/ploting/
description: >
  This page describes how to set up the Ninjabot web dashboard, its indicators, the live control panel and the HTTP API.
---

## Basic Usage

The dashboard displays candles, indicators, orders, the equity curve and the trade statistics of your bot. In backtesting it is a report of the simulation; in paper and live trading it updates in real time.

To create a chart, you need to import the `ui` packages.

```go
import (
	"github.com/rodrigo-brito/ninjabot/ui"
	"github.com/rodrigo-brito/ninjabot/ui/indicator"
)
```

Then, you can create a chart using `ui.New`. The following example creates a chart with 3 indicators. To include indicators, you must pass the option `WithCustomIndicators`, which receives one or more indicators.

Currently, Ninjabot supports the following indicators in charts:

| Indicator | Function |
|---|---|
| Exponential Moving Average (EMA) | `indicator.EMA(period, color)` |
| Simple Moving Average (SMA) | `indicator.SMA(period, color)` |
| Relative Strength Index (RSI) | `indicator.RSI(period, color)` |
| Moving Average Convergence Divergence (MACD) | `indicator.MACD(fast, slow, signal, colorMACD, colorSignal, colorHist)` |
| Percentage Price Oscillator (PPO) | `indicator.PPO(fast, slow, signal, colorPPO, colorSignal, colorHist)` |
| Stochastic Oscillator (STOCH) | `indicator.Stoch(fastK, slowK, slowD, colorK, colorD)` |
| Bollinger Bands | `indicator.BollingerBands(period, stdDeviation, upDnBandColor, midBandColor)` |
| Supertrend | `indicator.Spertrend(period, factor, color)` |
| Commodity Channel Index (CCI) | `indicator.CCI(period, color)` |
| Williams' %R | `indicator.WillR(period, color)` |
| On Balance Volume (OBV) | `indicator.OBV(color)` |

For each indicator, you need to inform the parameters that are necessary and colors. We accept the color name and HEX code as bellow.

```go
chart, err := ui.New(
	ui.WithCustomIndicators( // Optional parameter to include indicators
		indicator.EMA(8, "red"),
		indicator.EMA(21, "#000"),
		indicator.RSI(14, "purple"),
		indicator.Stoch(8, 3, 3, "red", "blue"),
	),
	ui.WithStrategyIndicators(strategy), // Optional parameter to include indicators from your strategy
	ui.WithPaperWallet(wallet), // Optional parameter to include portfolio results (drawdown, equity evolution, etc)
	ui.WithPort(8080), // Optional parameter to customize the port number
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
The default address are `http://localhost:8080`. But you can change it by passing the `ui.WithPort(portNumber)` option.

```go
err := chart.Start()
if err != nil {
    log.Fatal(err)
}
```

In live and paper trading, `chart.Start()` blocks, so run it in a goroutine before `bot.Run(ctx)`:

```go
go func() {
	if err := chart.Start(); err != nil {
		log.Fatal(err)
	}
}()

err = bot.Run(ctx)
```

### Final Result

![Chart Result](https://user-images.githubusercontent.com/7620947/150690553-1d1db358-2b05-42eb-8909-2bf254a2460b.png)

## The dashboard bundle

The dashboard front-end is **not embedded** in your binary. On the first start it is downloaded from the GitHub release that matches the Ninjabot version in your `go.mod`, verified against the release checksums and cached in your user cache directory:

| OS | Cache directory |
|---|---|
| macOS | `~/Library/Caches/ninjabot/ui` |
| Linux | `~/.cache/ninjabot/ui` |
| Windows | `%LocalAppData%\ninjabot\ui` |

After the first download it works offline. The behaviour can be customized with:

| Override | Effect |
|---|---|
| `NINJABOT_UI_DIR=/path/to/web/dist` or `ui.WithUIDir(dir)` | Serve a local build of the dashboard, no download at all. |
| `NINJABOT_UI_VERSION=v1.2.3` / `latest` or `ui.WithUIVersion(v)` | Pin the release tag to download. |
| `ui.WithCacheDir(dir)` | Change where the bundles are cached. |
| `ui.WithHTTPClient(client)` | Use your own HTTP client (proxy, timeouts) for the download. |

By default the tag is the Ninjabot version in your `go.mod`. For a commit hash (pseudo-version) the nearest release before it is used, and inside a local checkout (`go run ./examples/...` or a `replace` directive) the nearest `git` tag. When no release can be inferred, the latest one is downloaded and a warning is logged, since that bundle may not match the API of your build.

If the bundle cannot be fetched, the bot keeps running, the HTTP API stays up, and `http://localhost:8080` explains how to fix it.

## Bot control panel

The dashboard can also drive the bot: start/stop and manual buy/sell orders, mirroring the [Telegram commands]({{< relref "/docs/telegram" >}}). Pass the bot controller to the chart before starting it:

```go
bot, err := ninjabot.NewBot(ctx, settings, paperWallet, strategy /* , options... */)
if err != nil {
	log.Fatal(err)
}

// enable the control panel
chart.SetOrderController(bot.Controller())
```

Without a controller, the control endpoints report `{"enabled": false}` and the panel is hidden in the interface.

{{% pageinfo color="warning" %}}
Anyone with access to the dashboard port can operate your bot once the controller is set. Keep the port private - bind it to localhost, a VPN, or put an authenticating proxy in front of it.
{{% /pageinfo %}}

## HTTP API

The JSON API is served even when the dashboard bundle is unavailable, so you can build your own interface or monitoring on top of it.

| Endpoint | Description |
|---|---|
| `GET /api/health` | Liveness probe. |
| `GET /api/pairs` | Pairs being traded. |
| `GET /api/{pair}/snapshot` | Candles, indicators, orders, equity curve and statistics of a pair. |
| `GET /api/{pair}/orders.csv` | Orders of a pair, in CSV. |
| `GET /api/events` | Server-Sent Events stream (`candle`, `order`, `controls`). |
| `GET /api/controls` | Control panel state, eg. `{"enabled": true, "status": "running"}`. |
| `POST /api/controls/start` | Resume order placement. |
| `POST /api/controls/stop` | Pause order placement. |
| `POST /api/controls/order` | Create a manual market order. |

The order payload takes the amount in quote currency, or a percentage of the available balance (quote balance for buys, asset balance for sells):

```bash
# buy 100 USDT of BTC
curl -X POST localhost:8080/api/controls/order \
  -d '{"pair": "BTCUSDT", "side": "buy", "amount": 100}'

# sell 50% of the BTC balance
curl -X POST localhost:8080/api/controls/order \
  -d '{"pair": "BTCUSDT", "side": "sell", "amount": 50, "percent": true}'
```

Orders sent while the bot is stopped are rejected with `409 Conflict`.

## Custom Indicators

You can create custom indicators. An indicator is a `struct` that implements the `ui.CustomIndicator` interface.

```go
type CustomIndicator interface {
	Name() string // indicator name
	Overlay() bool // set if the indicator overlays the candlestick chart
	Warmup() int // number of candles required before the indicator has values
	Metrics() []IndicatorMetric // returns the indicator metrics (lines, bars, etc) and styles
	Load(dataframe *model.Dataframe) // initializes the indicator with a dataframe
}
```

The following example creates a custom indicator called `EMA`.

```go
package indicator

import (
	"fmt"
	"time"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/ui"

	"github.com/markcheno/go-talib"
)

func EMA(period int, color string) ui.CustomIndicator {
	return &ema{
		Period: period,
		Color:  color,
	}
}

type ema struct {
	Period int
	Color  string
	Values model.Series[float64]
	Time   []time.Time
}

func (e ema) Name() string {
	return fmt.Sprintf("EMA(%d)", e.Period)
}

func (e ema) Overlay() bool {
	return true
}

func (e ema) Warmup() int {
	return e.Period
}

func (e *ema) Load(dataframe *model.Dataframe) {
	if len(dataframe.Time) < e.Period {
		return
	}

	e.Values = talib.Ema(dataframe.Close, e.Period)[e.Period:]
	e.Time = dataframe.Time[e.Period:]
}

func (e ema) Metrics() []ui.IndicatorMetric {
	return []ui.IndicatorMetric{
		{
			Style:  "line",
			Color:  e.Color,
			Values: e.Values,
			Time:   e.Time,
		},
	}
}
```
