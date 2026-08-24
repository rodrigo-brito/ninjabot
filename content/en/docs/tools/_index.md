---
title: "Tools"
linkTitle: "Tools"
categories: ["Reference"]
weight: 7
description: >
  This page describes the helpers available in the tools package - trailing stop and order scheduler.
---

The `tools` package has helpers commonly needed by strategies. They are plain structs, so you can keep one per pair inside your strategy.

## Trailing Stop

A trailing stop follows the price while it moves in your favour, and reports when the price falls back to the stop.

```go
import "github.com/rodrigo-brito/ninjabot/tools"

trailing := tools.NewTrailingStop()

// when a position is opened: current price and the initial stop
trailing.Start(currentPrice, currentPrice*0.95)

// on every candle
if trailing.Active() && trailing.Update(candle.Close) {
	// the stop was hit, close the position
	_, err := broker.CreateOrderMarket(ninjabot.SideTypeSell, pair, assetPosition)
	if err != nil {
		log.Error(err)
	}
	trailing.Stop()
}
```

`Update` returns `true` only when the price reaches the stop. While the price rises, the stop rises with it, keeping the same distance. Combined with `OnPartialCandle`, the stop can be checked without waiting for the candle to close.

A complete strategy using it is available in [examples/strategies/trailingstop.go](https://github.com/rodrigo-brito/ninjabot/blob/main/examples/strategies/trailingstop.go).

## Order Scheduler

The scheduler places a market order as soon as a condition over the dataframe becomes true. Once the order is created, the condition is removed from the queue.

```go
scheduler := tools.NewScheduler("BTCUSDT")

// buy 1 BTC when the price drops below 30k
scheduler.BuyWhen(1, func(df *ninjabot.Dataframe) bool {
	return df.Close.Last(0) < 30000
})

// sell 1 BTC when the price goes above 50k
scheduler.SellWhen(1, func(df *ninjabot.Dataframe) bool {
	return df.Close.Last(0) > 50000
})

// on every candle, inside OnCandle
scheduler.Update(df, broker)
```

This is useful to prepare an exit without keeping the state in your strategy, or to break a large position into conditional orders.
