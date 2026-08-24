---
title: "Exchanges"
linkTitle: "Exchanges"
categories: ["Reference"]
weight: 6
description: >
  This page describes how to connect Ninjabot to Binance spot and futures, and the options available for each one.
---

Currently, Ninjabot supports the [Binance](https://www.binance.com/en?ref=35723227) exchange, in the spot and futures markets. Any other exchange can be added by implementing the `service.Exchange` interface - the implementations in the [exchange](https://github.com/rodrigo-brito/ninjabot/tree/main/exchange) directory are good references.

## Binance Spot

```go
binance, err := exchange.NewBinance(ctx,
	exchange.WithBinanceCredentials(apiKey, secretKey),
)
if err != nil {
	log.Fatal(err)
}

bot, err := ninjabot.NewBot(ctx, settings, binance, strategy)
```

| Option | Description |
|---|---|
| `WithBinanceCredentials(key, secret)` | API key and secret of your account. Without them, only public data (candles) is available. |
| `WithBinanceHeikinAshiCandle()` | Convert the candles to [Heikin Ashi]({{< relref "/docs/strategy" >}}) before they reach your strategy. |
| `WithTestNet()` | Use the Binance testnet, with fake money, to validate a bot end to end before going live. |
| `WithMetadataFetcher(fetcher)` | Attach custom metadata to every candle, eg. data from an external API. |
| `WithCustomMainAPIEndpoint(api, ws, combined)` | Point the REST and websocket clients somewhere else, eg. a proxy. |
| `WithCustomTestnetAPIEndpoint(api, ws, combined)` | The same, for the testnet endpoints. |

## Binance Futures

```go
binance, err := exchange.NewBinanceFuture(ctx,
	exchange.WithBinanceFutureCredentials(apiKey, secretKey),
	exchange.WithBinanceFutureLeverage("BTCUSDT", 1, exchange.MarginTypeIsolated),
	exchange.WithBinanceFutureLeverage("ETHUSDT", 1, exchange.MarginTypeIsolated),
)
```

| Option | Description |
|---|---|
| `WithBinanceFutureCredentials(key, secret)` | API key and secret of your futures account. |
| `WithBinanceFutureLeverage(pair, leverage, marginType)` | Leverage and margin type of a pair. The margin type is `exchange.MarginTypeIsolated` or `exchange.MarginTypeCrossed`. |
| `WithBinanceFuturesHeikinAshiCandle()` | Convert the candles to Heikin Ashi. |

In the futures market you can also open **short positions**, selling an asset you do not hold. The same is simulated by the paper wallet in [backtesting]({{< relref "/docs/backtesting" >}}).

{{% pageinfo color="warning" %}}
Leverage multiplies the losses as much as the profits. Validate your strategy in backtesting and paper trading before using real money, and never with money you cannot afford to lose.
{{% /pageinfo %}}

## Trading pairs

The pairs of your bot are informed in the settings, and must exist in the exchange you selected:

```go
settings := ninjabot.Settings{
	Pairs: []string{"BTCUSDT", "ETHUSDT"},
}
```

Ninjabot splits the asset and the quote of a pair with `exchange.SplitAssetQuote(pair)`, using the list of known quote currencies. All the balances, order sizes and results are reported in the quote currency of the pair - `USDT` for `BTCUSDT`.
