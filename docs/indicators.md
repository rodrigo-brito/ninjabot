# Indicators: legacy windows and streaming strategies

NinjaBot supports two ways of computing indicators. Both work with every part of the bot
(live trading, paper wallet, backtests, the dashboard); they differ in cost and in the values
recursive indicators produce.

## Legacy strategies: `Indicators(df)`

A legacy strategy recomputes its indicators on every closed candle:

```go
func (e CrossEMA) WarmupPeriod() int { return 22 }

func (e CrossEMA) Indicators(df *ninjabot.Dataframe) []strategy.ChartIndicator {
	df.Metadata["ema8"] = indicator.EMA(df.Close, 8)
	df.Metadata["sma21"] = indicator.SMA(df.Close, 21)
	return nil
}
```

The controller hands `Indicators` and `OnCandle` a **window of the last `WarmupPeriod()`
candles**, not the whole history. That keeps the cost bounded (`O(WarmupPeriod)` per candle),
but recursive indicators — EMA, RSI, ATR, MACD — are then seeded from the start of the window
every time. Their value depends on the window size and differs from the same indicator
computed over the full history, as TradingView or talib over the whole series would show.

`strategy.WithFullHistory()` (through `ninjabot.WithStrategyOptions` or
`backtest.Config.ControllerOptions`) hands the whole history instead. Values are then exact,
at a cost that grows with the history: `O(N)` per candle, `O(N²)` over a backtest.

`HighFrequencyStrategy.OnPartialCandle` always receives the whole history, and its indicators
are recomputed over it on every partial candle.

## Streaming strategies: `Setup(pair, set)`

A streaming strategy registers incremental indicators from `indicator/stream` once per pair:

```go
func (e *CrossEMA) Indicators(*ninjabot.Dataframe) []strategy.ChartIndicator { return nil }

func (e *CrossEMA) Setup(_ string, set *strategy.IndicatorSet) {
	set.Line("fast", stream.NewEMA(8), stream.Close)
	set.Line("slow", stream.NewSMA(21), stream.Close)
	set.Plot(strategy.ChartSpec{GroupName: "MA's", Overlay: true, Metrics: []strategy.MetricSpec{
		{Key: "fast", Name: "EMA 8", Color: "red"},
		{Key: "slow", Name: "SMA 21", Color: "blue"},
	}})
}

func (e *CrossEMA) OnCandle(df *ninjabot.Dataframe, broker service.Broker) {
	if df.Metadata["fast"].Crossover(df.Metadata["slow"]) { /* ... */ }
}
```

- The controller updates each indicator once per closed candle, in constant time, and appends
  its value to `df.Metadata[name]`. Values are computed over the **whole history** and match
  talib over the full series.
- Partial candles preview the indicators (`Preview`) without advancing them: the last value of
  `df.Metadata[name]` reflects the partial candle, and the next closed candle commits it once.
- `Indicators` is never called; the dashboard draws what `Plot` declares.
- `WarmupPeriod()` still gates `OnCandle`, which receives the whole history (no window is
  rebuilt per candle). `strategy.WithCapacity(n)` pre-allocates the history; the backtest
  engine sets it to the number of candles of each pair.

| Indicator | Constructor | Kind | `df.Metadata` keys |
|---|---|---|---|
| Simple / exponential moving average | `stream.NewSMA(n)`, `stream.NewEMA(n)` | Line | name |
| Relative strength index | `stream.NewRSI(n)` | Line | name |
| Rolling max / min | `stream.NewMax(n)`, `stream.NewMin(n)` | Line | name |
| Variance / standard deviation | `stream.NewVariance(n)`, `stream.NewStdDev(n, k)` | Line | name |
| Average true range | `stream.NewATR(n)` | Bar | name |
| On-balance volume | `stream.NewOBV()` | Bar | name |
| SuperTrend | `stream.NewSuperTrend(n, factor)` | Bar | name |
| MACD | `stream.NewMACD(fast, slow, signal)` | Multi | `prefix_macd`, `prefix_signal`, `prefix_hist` |
| Bollinger Bands | `stream.NewBB(n, k)` | Multi | `prefix_upper`, `prefix_middle`, `prefix_lower` |
| Slow stochastic | `stream.NewStoch(fastK, slowK, slowD)` | Multi | `prefix_k`, `prefix_d` |

Line indicators read one value per candle through a `stream.Source` (`Close`, `Open`, `High`,
`Low`, `Volume`, `HL2`, `HLC3`, `OHLC4`, or any `func(model.Candle) float64`). Any type
implementing `stream.Line`, `stream.Bar` or `stream.Multi` can be registered.

## Choosing

| | Legacy (default window) | Legacy + `WithFullHistory` | Streaming |
|---|---|---|---|
| Cost per closed candle | `O(WarmupPeriod)` | `O(N)` | `O(1)` |
| Cost per partial candle | `O(N)` | `O(N)` | `O(1)` |
| Recursive indicators | biased by the window | exact | exact |
| Any talib function | yes | yes | the ones in `indicator/stream` |

`strategy.WithMaxHistory(n)` bounds the memory of long-running bots for both styles.

The example strategies in `examples/strategies` are streaming strategies with parameters, and
are registered by name (`emacross`, `turtle`, `ocosell`, `trailing`) for the CLI.
