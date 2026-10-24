# Changelog

All notable changes to this project are documented here. Changes that alter the
numbers of an existing backtest are listed under **Result changes**, with the
reason, so a different result can be traced back to a deliberate correction.

## Unreleased

### Added

- `PaperWallet.TakeFills`, `PaperWallet.Equity` ([ROADMAP 0.1](ROADMAP.md#01-paperwallet-hygiene-x)).
- `order.Controller.Sync`, `order.Controller.Trades` and `order.NewSyncFeed`; `order.Result` gains
  JSON tags, `ClosedAt` and `OrderID` ([ROADMAP 0.2](ROADMAP.md#02-order-controller-synchronous-path-x)).
- `storage.NewMemory`, a slice-backed storage roughly 150x faster than BuntDB on `Orders()`
  ([ROADMAP 0.3](ROADMAP.md#03-in-memory-storage-for-backtests-x)).
- `exchange.ParseCSV` and `exchange.Resample`, side-effect free and safe for concurrent use
  ([ROADMAP 1.1](ROADMAP.md#11-pure-csv-parsing-and-resampling-x)).
- Package `backtest`: `Dataset` (load once, share between parallel runs, cached resampling,
  zero-copy `Window`) and `Engine`, a deterministic single-goroutine backtest loop
  ([ROADMAP 1.2](ROADMAP.md#12-backtestdataset-x), [1.3](ROADMAP.md#13-backtestengine-x)).
- Package `report`: the robustness statistics of the dashboard (`report.Build`), usable without
  the UI; IDs and verdicts are exported constants. `backtest.Result` is versioned
  (`schema_version`), carries the wallet summary, per pair and portfolio reports and warnings, and
  writes JSON and trades/equity/orders CSV ([ROADMAP 1.4](ROADMAP.md#14-backtestresult-and-report-package-x)).
- `PaperWallet.InitialValue`, `FeesPaid` and `Volume`.
- Package `indicator/stream`: incremental SMA, EMA, RSI, ATR, Max, Min, Variance, StdDev, OBV,
  MACD, Bollinger Bands, slow Stochastic and SuperTrend, reproducing go-talib values
  ([ROADMAP 2.1](ROADMAP.md#21-indicatorstream-x)).
- `strategy.StreamingStrategy`: register indicators once in `Setup` and the controller updates
  them in constant time, over the whole history; partial candles preview them without advancing.
  `strategy.WithFullHistory`, `strategy.WithMaxHistory`, `strategy.IndicatorObserver`,
  `ninjabot.WithStrategyOptions` ([ROADMAP 2.2](ROADMAP.md#22-strategystreamingstrategy-x)).
- `strategy.Registry`, `strategy.Register`, `strategy.RegisterParams`, `strategy.DecodeParams`,
  `strategy.ParamsOf` and `strategy.WithDefaults`: create strategies by name from parameters,
  validated against typed specs with JSON Schema ([ROADMAP 3.1](ROADMAP.md#31-strategyregistry-x)).
- Run specs (`backtest.Spec`, JSON or YAML, strict, validated, hashed) and the CLI commands
  `backtest`, `strategies`, `validate` and `schema`, in the reusable package `cli`, with exit
  codes per failure class ([ROADMAP 3.2](ROADMAP.md#32-run-spec-x), [3.3](ROADMAP.md#33-ninjabot-backtest-cli-x)).
- `ui.StreamIndicator` and `ui.Chart.OnIndicators`: the dashboard updates its indicators once per
  candle and shows the values the strategy computed; the built-in dashboard indicators are
  incremental. `stream.NewCCI`, `stream.NewWillR`, `stream.NewBBEMA`
  ([ROADMAP 2.3](ROADMAP.md#group-2--incremental-indicators)).
- Package `portfolio`: backtest several strategies with isolated sub-wallets and weights, in
  parallel; combined equity, drawdown, Sharpe, correlation between strategies, contribution of
  each one and a robustness report over the whole portfolio
  ([ROADMAP 4.1](ROADMAP.md#41-portfolio-with-isolated-sub-wallets-x)).
- CLI commands `portfolio`, `sweep` (grid or seeded random sample, cached by spec hash, ranked by
  any metric) and `compare`; packages `sweep` and `compare`; `backtest.Metrics` in every result
  ([ROADMAP 5.1](ROADMAP.md#51-ninjabot-sweep-x), [5.2](ROADMAP.md#52-ninjabot-compare-x)).
- Portfolio view in the dashboard (`ui.WithPortfolio`, `GET /api/portfolio`) and
  `ninjabot portfolio --serve` ([ROADMAP 4.2](ROADMAP.md#42-portfolio-cli-and-dashboard-x)).
- Shared-wallet portfolios: `model.Order.Tag` (new indexed column, added by `AutoMigrate`),
  positions and results per tag, `order.Controller.SetBudget`, `Scoped`, `TagEquity`,
  `backtest.RunShared` and `"mode": "shared"` in portfolio specs
  ([ROADMAP 4.3](ROADMAP.md#43-shared-wallet-with-per-strategy-attribution-advanced-x)).
- Overfitting guards: probability of backtest overfitting (CSCV) in every sweep summary
  (`metrics.PBO`), `ninjabot walkforward` and `ninjabot sensitivity`
  ([ROADMAP 5.3](ROADMAP.md#53-overfitting-guards-x)).
- `docs/agents.md` (file contracts, commands, metrics and recipes for agents and scripts) and
  `AGENTS.md`; `backtest.Spec.Resolve`, the canonical form of a run spec
  ([ROADMAP 5.4](ROADMAP.md#54-agent-documentation-x)).
- `backtest.Config.TradeFrom` and warmup before spec windows; `strategy.WithCapacity`, set by the
  engine to the size of each history. `indicator/stream` takes `*model.Candle`.
- `docs/indicators.md`: legacy windows versus streaming strategies.
- `NinjaBot.Result()` returns the result of a backtest run; `order.Controller.Orders()`.
- `make bench` and baseline benchmarks ([ROADMAP 0.4](ROADMAP.md#04-baseline-benchmarks-and-determinism-probe-x)).

### Fixed

- `PaperWallet` takes its lock in `Order`, `Account`, `ID`, `Pairs`, `EquityValues`, `AssetValues`,
  `MaxDrawdown` and `Summary`, which raced with `OnCandle` while the order controller polled fills.
- `NewPaperWallet` no longer panics when the base coin has no initial balance.
- `order.Controller` publishes orders outside its lock when the feed is asynchronous, and applies
  the fills found by one sync in the order they happened.

- `NewCSVFeed` closes the files it reads.
- The order controller only builds the per-trade summary table when a notifier is set or the
  log level is Info or lower; backtests run ~3x faster end to end.

### Changed

- With a dashboard connected, a closed candle no longer recomputes every indicator over the
  whole history: the cost per candle is constant instead of growing with the backtest.

- The strategy controller reuses one window per pair instead of allocating a dataframe map on
  every candle.

- `NewBot(..., WithBacktest(wallet))` runs on `backtest.Engine`: no goroutine per feed, no
  priority queue holding the whole dataset, no 1-second order ticker. Candle subscribers (the
  dashboard) receive each candle synchronously right after the strategy processed it, and order
  subscribers receive orders inline, in the order they happen.

- Reports never contain NaN or infinite numbers (they used to break the JSON encoding of
  `/api/{pair}/statistics` for tiny samples); a test that cannot run reports a p-value of 1.

### Result changes

- **Windowed runs warm up before the window.** A spec `window` (and every walk-forward fold)
  used to start the strategy from scratch at `window.start`, so short windows traded nothing
  until the indicators were ready. The engine now loads the warmup before the window and only
  feeds it to the indicators; trading and every measure still start at `window.start`.

- **All-in orders work without fees.** With zero fees, an order sized with the whole balance
  could exceed it by float rounding and was rejected; the rounding is now trimmed like it was
  with fees.

- **Selling a whole position never opens a short for a rounding residue.** When the balance
  was a float rounding error below the sold quantity, the wallet treated the sale as a short
  entry and debited the asset twice. Such sells are now liquidations.

- **The paper wallet sweeps balance dust.** Trimming an order by its fee could leave residues of
  float rounding (7e-15 ETH, -1e-12 USDT). Strategies saw an open position, tried to sell it,
  failed with "insufficient funds", and some of those attempts were recorded as zero-profit
  trades. Balances below 1e-9 are now zeroed after every fill: logs lose the spurious errors and
  trade counts lose the spurious trades (the example `emacross` run goes from 26 to 23 trades
  with the same final equity).

- **Example strategies are streaming strategies with parameters.** `CrossEMA`, `Turtle`,
  `OCOSell` and the trailing stop strategy take their parameters from a struct (defaults match
  the old hardcoded values), register themselves as `emacross`, `turtle`, `ocosell` and
  `trailing`, and compute their indicators over the whole history instead of a window of
  `WarmupPeriod` candles. Their EMA-based signals change accordingly
  ([ROADMAP 2.4](ROADMAP.md#24-migrate-example-strategies-x)). The `ema8`/`sma21` metadata keys of
  `CrossEMA` are now `fast`/`slow`, and the stochastic keys of `OCOSell` are `stoch_k`/`stoch_d`.

- **Asynchronous fills are applied on the candle they happen.** LIMIT, OCO and STOP fills used to
  become trades only when the order controller's 1-second ticker fired, concurrently with the
  backtest loop: they were applied late and out of order with the MARKET orders sent meanwhile,
  which corrupted position averages and the trade list. The engine syncs the order controller
  right after the wallet reports a fill, before the strategy sees the candle. Strategies that only
  send MARKET orders produce the same numbers as before.

- **Equity curve: one point per timestamp.** With several pairs the paper wallet used to record
  one equity point per pair candle; it now replaces the point of the same timestamp, priced with
  the latest candle of every pair. `EquityValues`, `AssetValues` and the dashboard equity chart
  get shorter, and every statistic computed per bar changes accordingly.
- **Max drawdown is peak-to-trough.** `PaperWallet.MaxDrawdown` used to add up consecutive
  declines and restart at the first rising candle, so a fall interrupted by a small rebound was
  under-reported. It now returns the largest decline from a running peak (as a negative
  fraction, as before), the same definition used by `tools/metrics.MaxDrawdown`.
- The paper wallet logs its setup and realized profit at Debug instead of Info.
