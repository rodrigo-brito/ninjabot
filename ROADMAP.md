# Roadmap: portfolio backtesting, deterministic engine, incremental indicators and an agent-friendly platform

This document is the working plan for the next generation of the NinjaBot backtesting stack.
It is split into **groups of small, independently reviewable items**, each with a motivation,
a specification, dependencies and measurable acceptance criteria. The intent is to attack one
item (or a few) per session, validate it, merge it, and move on.

Status legend: `[ ]` not started · `[~]` in progress · `[x]` done (implemented and tested; not yet merged).

- [Goals](#goals)
- [Principles](#principles)
- [Where we are today](#where-we-are-today)
- [Summary table](#summary-table)
- [Group 0 — Foundations: correctness and determinism](#group-0--foundations-correctness-and-determinism)
- [Group 1 — Immutable dataset and deterministic backtest engine](#group-1--immutable-dataset-and-deterministic-backtest-engine)
- [Group 2 — Incremental indicators](#group-2--incremental-indicators)
- [Group 3 — Parameterizable strategies and run specs](#group-3--parameterizable-strategies-and-run-specs)
- [Group 4 — Portfolio backtest](#group-4--portfolio-backtest)
- [Group 5 — Experimentation platform for AI agents](#group-5--experimentation-platform-for-ai-agents)
- [Recommended merge order](#recommended-merge-order)
- [Risks](#risks)
- [Working agreement](#working-agreement)

## Goals

1. **Portfolio backtests.** Run several strategies together (different strategies, or the same
   strategy with different parameters, timeframes or pairs) and report each one individually
   *and* the combined result: equity curve, drawdown, risk-adjusted return, correlation between
   strategies and the contribution of each one. A single strategy is often rough; a basket of
   weakly correlated strategies smooths the curve.
2. **Concurrency-safe and deterministic calculations.** Several backtests must be able to run in
   parallel inside one process without data races, and the same input must always produce the
   same output, byte for byte.
3. **Performance on long histories.** Indicators are currently recomputed from scratch on every
   candle. The engine must reuse work so that evaluating many strategy combinations is cheap.
4. **An agent-friendly platform.** Strategies, data and runs become parameterizable through
   files; outputs become structured, versioned and comparable, so an AI agent (or a script) can
   test ideas, sweep parameters and compare runs without touching Go code.

## Principles

- **Determinism first.** A backtest is a pure function of (data, spec). No timers, no
  goroutine ordering, no wall-clock in the hot path. Every randomized statistic takes an
  explicit seed (`tools/metrics.DefaultSeed` already does this).
- **`go test -race` always green.** Every PR runs the race detector (already in `make test`
  and CI). New concurrent paths ship with a concurrent test.
- **Small PRs.** Each item below should fit in one reviewable PR (roughly under 500 lines of
  non-test code). If it does not, split it.
- **Keep the public API working.** `ninjabot.NewBot(...)`, `WithBacktest`, `strategy.Strategy`,
  `indicator.*` keep working. New capabilities are added through new packages and optional
  interfaces, not by breaking the existing ones.
- **Result changes only with a changelog entry.** Some items fix semantics (synchronous fills,
  one equity point per timestamp, peak-to-trough drawdown, full-history indicators) and will
  change numbers in existing backtests. That is accepted, but every such change is documented
  in the PR and the golden tests are updated deliberately, never silently.
- **Measure before and after.** Benchmarks and golden tests are part of the acceptance
  criteria, not an afterthought.

## Where we are today

Verified in the code at the time of writing (branch `feat/backtest-statistics`).

**Topology.** One bot runs **one strategy and one timeframe**. `ninjabot.go` holds a single
`strategy.Strategy`, one `strategy.Controller` per pair, one `order.Controller` and one
`exchange.PaperWallet` for the whole bot. `model.Settings` only has `Pairs` and `Telegram`.
A backtest is configured only in Go code (`examples/backtesting/backtesting.go`); there is no
`backtest` command in `cmd/ninjabot`.

**Indicators are recomputed from scratch.**

- `strategy/controller.go:83-99`: on every closed candle the controller appends to an
  ever-growing dataframe, calls `Dataframe.Sample(warmupPeriod)` (sub-slices, but a fresh
  `Metadata` map every candle) and then calls `strategy.Indicators()` over the whole window
  of `W = WarmupPeriod()` candles. The computed series are discarded after `OnCandle`.
  Cost is `O(W)` per candle. Recursive indicators (EMA, RSI, ATR) computed over only `W`
  points carry warm-up bias and differ from the same indicator over the full history.
- `strategy/controller.go:47-55`: `OnPartialCandle` (for `HighFrequencyStrategy`) calls
  `Indicators` over the **full** dataframe, `O(N)` per tick.
- `ui/chart.go:217` and `:315-370`: with a dashboard connected, every closed candle runs
  every custom indicator (`Load(df)`) and `strategy.Indicators(df)` over the **entire**
  history and keeps only the last point: `O(N²)` over a backtest.
- `indicator/talib.go` is a thin wrapper over `go-talib`; every function takes and returns
  full `[]float64` slices and has no state. No incremental implementation exists.
- There are no `Benchmark*` functions in the repository and no `strategy/controller_test.go`.

**The backtest loop is not deterministic and has data races.**

- `order/controller.go:495-506`: `Start()` launches a goroutine with a **1-second ticker**
  that calls `updateOrders()` (`:423-467`). Fills of LIMIT, OCO and STOP orders only become
  trades when that ticker fires (or in `Stop()`), i.e. concurrently with the backtest loop.
  `updateOrders` reads storage and calls `exchange.Order()` without holding the lock;
  `PaperWallet.Order` (`exchange/paperwallet.go:865`) iterates `p.orders` without locking
  while `OnCandle` mutates it. Beyond the race, trades are applied out of chronological order
  relative to MARKET orders, which corrupts `Position` averages. This is a correctness bug,
  not only a determinism one.
- `exchange/exchange.go:105-137` and `ninjabot.go:336-357`: the backtest starts one goroutine
  per feed, pushes the **whole dataset** into a mutex-protected priority queue and only then
  pops and processes it in one goroutine. `order.Feed.Publish` is called in a new goroutine
  per order (`order/controller.go:573,599,627,655,680`), so `ui.Chart` receives orders out of
  order with respect to candles.
- `exchange/paperwallet.go:599-629`: `equityValues` gets one point **per closed candle of
  every pair** (two pairs → two points per timestamp). `PaperWallet.MaxDrawdown`
  (`:142-179`) sums consecutive negative differences; it is not a peak-to-trough drawdown.
  A correct implementation already exists in `tools/metrics.MaxDrawdown`.
- `exchange/csvfeed.go:170,291`: `Limit()` and `CandlesByLimit` mutate the feed in place,
  so a `CSVFeed` cannot be shared between parallel runs.
- `storage.FromMemory()` is BuntDB with one JSON document per order; `Orders(filter)` is a
  full scan plus unmarshal of every order. Too slow to call once per candle.
- Global state: logrus level is global (`WithLogLevel`), the progress bar writes to stdout,
  `NewPaperWallet` logs at Info. `exchange.pairAssetQuoteMap` is read-only after `init`.

**Outputs are text.** `NinjaBot.Summary()` prints a table, a histogram and bootstrap
intervals to stdout. `SaveReturns` writes a header-less, non-chronological CSV of returns.
`order.Controller.Results` is an unexported `summary` without JSON tags. The only structured
output is the dashboard HTTP API (`ui/dto.go`); the pure statistics code in
`ui/statistics.go` (Sharpe, Sortino, bootstrap, Monte Carlo, permutation tests built on
`tools/metrics`) is locked inside the `ui` package.

## Summary table

| ID  | Title                                        | Group | Depends on      | Size |
|-----|----------------------------------------------|-------|-----------------|------|
| 0.1 | PaperWallet hygiene                          | 0     | —               | S    |
| 0.2 | Order controller synchronous path            | 0     | 0.1             | S    |
| 0.3 | In-memory storage for backtests              | 0     | —               | S    |
| 0.4 | Baseline benchmarks and determinism probe    | 0     | —               | S    |
| 1.1 | Pure CSV parsing and resampling              | 1     | —               | S    |
| 1.2 | `backtest.Dataset`                           | 1     | 1.1             | S    |
| 1.3 | `backtest.Engine`                            | 1     | 0.1, 0.2, 0.3, 1.2 | L |
| 1.4 | `backtest.Result` and `report` package       | 1     | 1.3             | M    |
| 2.1 | `indicator/stream`                           | 2     | —               | M    |
| 2.2 | `strategy.StreamingStrategy`                 | 2     | 2.1             | M    |
| 2.3 | UI without O(N²)                             | 2     | 2.2             | M    |
| 2.4 | Migrate example strategies                   | 2     | 2.2, 3.1        | S    |
| 3.1 | `strategy.Registry`                          | 3     | —               | S    |
| 3.2 | Run spec                                     | 3     | 1.4, 3.1        | S    |
| 3.3 | `ninjabot backtest` CLI                      | 3     | 3.2             | M    |
| 4.1 | Portfolio with isolated sub-wallets          | 4     | 1.4, 3.2        | M    |
| 4.2 | Portfolio CLI and dashboard                  | 4     | 4.1, 3.3        | M    |
| 4.3 | Shared wallet with per-strategy attribution  | 4     | 4.1             | L    |
| 5.1 | `ninjabot sweep`                             | 5     | 3.3, 4.1        | M    |
| 5.2 | `ninjabot compare`                           | 5     | 3.3             | S    |
| 5.3 | Overfitting guards                           | 5     | 5.1             | M    |
| 5.4 | Agent documentation                          | 5     | 3.3, 5.1, 5.2   | S    |

Size: S ≈ one short session, M ≈ one long session, L ≈ two sessions.

---

## Group 0 — Foundations: correctness and determinism

Fixes the races and semantic bugs in the existing components without changing behaviour for
MARKET-only strategies. Everything later builds on these.

### 0.1 PaperWallet hygiene `[x]`

**Motivation.** The wallet is the exchange and the broker in a backtest. It must be safe to
read from any goroutine and must expose a correct equity curve.

**Specification** (`exchange/paperwallet.go`):

- Take the lock in `Order`, `Account`, `EquityValues`, `AssetValues`, `MaxDrawdown`,
  `Summary` and `ID`.
- `func (p *PaperWallet) TakeFills() []model.Order` — returns and clears the orders whose
  status changed during the last `OnCandle` calls. Lets the engine sync the order controller
  only when something happened.
- `func (p *PaperWallet) Equity() float64` — the computation currently inlined in
  `updateEquityValues`.
- `updateEquityValues`: when the candle `Time` equals the last recorded point, **overwrite**
  it instead of appending. One point per timestamp regardless of the number of pairs.
- `MaxDrawdown()` delegates to `tools/metrics.MaxDrawdown` over the equity values and
  returns the peak-to-trough value with start and end times. The old implementation is
  removed.
- Creation and fill logs move from Info to Debug.

**Acceptance.**

- `make test` green with `-race`; `ninjabot_test.go` golden values unchanged (they only use
  MARKET orders).
- New test: two pairs, same timestamps → `len(EquityValues()) == number of distinct
  timestamps`.
- New test: `MaxDrawdown` on a hand-built curve matches the peak-to-trough value;
  `exchange/paperwallet_test.go:328` updated with the justified new number.
- Changelog entry: equity granularity and drawdown definition changed.

### 0.2 Order controller synchronous path `[x]`

**Motivation.** Fills must be turned into trades synchronously, in candle order, in backtests.
The ticker stays for live trading only.

**Specification** (`order/controller.go`, `order/feed.go`):

- Rename `updateOrders` to exported `Sync()` (keep a private alias for the ticker).
- Record every closed trade: `trades []Result` appended in `updatePosition`, exported through
  `Trades() []Result`. Add JSON tags to `order.Result` and a `Tag string` placeholder is
  **not** added yet (see 4.3).
- `OnCandle` writes `lastPrice` under the lock.
- `order.NewSyncFeed() *Feed` — same type, `Publish` calls the consumers directly on the
  caller's goroutine, `Start` is a no-op. `Controller` calls `Publish` directly (no
  `go ...`) when the feed is synchronous.
- `Start()` is not required for `Sync()` to work (`requireRunning` already accepts the zero
  status for order creation; document it).

**Acceptance.**

- New table-driven test: MARKET buy → LIMIT sell filled by `PaperWallet.OnCandle` →
  `Sync()` → MARKET buy; `Position` average price and the recorded `Result`s are correct and
  chronological.
- `-race` green; existing `order/controller_test.go` passes.

### 0.3 In-memory storage for backtests `[x]`

**Motivation.** The engine will call `Sync()` whenever a fill happens; the storage must answer
`Orders(filter)` in microseconds.

**Specification** (`storage/memory.go`):

- `storage.NewMemory() Storage` backed by a slice plus a `map[int64]int` index; `Orders`
  filters in memory and returns rows ordered by `(UpdatedAt, ID)`, the same order as the
  BuntDB `updated_at` index.
- A shared conformance test suite (`storage/storage_test.go`) run against BuntDB, SQL and
  memory.
- `storage.FromMemory()` keeps returning BuntDB for now; the engine defaults to
  `NewMemory()`. Deprecation of `FromMemory` is decided later.

**Acceptance.**

- Conformance suite green for the three backends.
- `BenchmarkStorage_Orders` shows memory ≥ 10× faster than BuntDB on 1 000 orders.

### 0.4 Baseline benchmarks and determinism probe `[x]`

**Motivation.** Later items promise speed-ups; the baseline must exist first.

**Specification.**

- `BenchmarkBot_Run_2Pairs_1h` (in `ninjabot_test.go`, uses `testdata/btc-1h.csv` and
  `eth-1h.csv`, memory storage), `BenchmarkController_OnCandle` (W = 22 and W = 200, three
  indicators), `BenchmarkChart_OnCandle` with one SSE subscriber.
- `make bench` target running `go test -run '^$' -bench . -benchmem ./...` and a short note
  in `CONTRIBUTING.md` on using `benchstat`.
- `TestBacktest_RunTwice` documenting what is and is not deterministic today (expected to
  pass for MARKET-only strategies and to be skipped with an explanation for OCO until 1.3).

**Acceptance.** Baseline numbers recorded in this file under
[Benchmarks](#benchmarks).

---

## Group 1 — Immutable dataset and deterministic backtest engine

Replaces goroutines, channels, the priority queue and the ticker with a single-threaded,
deterministic loop that reuses the existing components. `ninjabot.NewBot(... WithBacktest)`
becomes a façade over the engine; the live path is untouched.

### 1.1 Pure CSV parsing and resampling `[x]`

**Specification** (`exchange/csvfeed.go`):

- `func ParseCSV(path, pair string, heikinAshi bool) ([]model.Candle, error)` and
  `func Resample(candles []model.Candle, from, to string) ([]model.Candle, error)` extracted
  from `NewCSVFeed` and `resample`, with no shared state.
- `NewCSVFeed` becomes a wrapper over them.

**Acceptance.** `exchange/csvfeed_test.go` passes unchanged; new unit tests for both functions.

### 1.2 `backtest.Dataset` `[x]`

**Motivation.** Load and resample once, share read-only between parallel runs.

**Specification** (`backtest/dataset.go`):

```go
type Dataset struct { /* raw per pair+timeframe, resample cache under sync.RWMutex */ }
func LoadDataset(feeds ...exchange.PairFeed) (*Dataset, error)
func (d *Dataset) Candles(pair, timeframe string) ([]model.Candle, error) // cached, read-only
func (d *Dataset) Pairs() []string
func (d *Dataset) Window(start, end time.Time) *Dataset // sub-slice view, no copy
```

A small `service.Feeder` adapter with its own cursor per run implements `CandlesByLimit`
without mutating the dataset.

**Acceptance.** Test with 8 goroutines calling `Candles` and running engines over the same
dataset under `-race`; returned slices are documented as read-only.

### 1.3 `backtest.Engine` `[x]`

**Motivation.** The heart of the roadmap: one deterministic loop per run.

**Specification** (`backtest/engine.go`, `backtest/loop.go`):

```go
type Config struct {
    Pairs             []string
    Strategy          strategy.Strategy
    Timeframe         string            // default Strategy.Timeframe()
    Quote             string            // e.g. "USDT"
    Balances          map[string]float64
    MakerFee, TakerFee float64
    Start, End        time.Time         // optional window
    Storage           storage.Storage   // default storage.NewMemory()
    Progress          func(done, total int)
    CandleSubscribers []func(model.Candle)
    OrderSubscribers  []func(model.Order)
}
func New(ds *Dataset, cfg Config) (*Engine, error)
func (e *Engine) Run(ctx context.Context) (*Result, error)
```

Loop, single goroutine:

1. k-way merge of the per-pair candle slices ordered by `(Time, UpdatedAt, Pair)` — the
   same ordering as `model.Candle.Less`, so the current processing order is preserved.
2. Per candle: `wallet.OnCandle(c)`; if `wallet.TakeFills()` is non-empty →
   `orders.Sync()`; `controller.OnPartialCandle(c)`; if `c.Complete` →
   `controller.OnCandle(c)` and `orders.OnCandle(c)`; then the synchronous subscribers.
3. When the next candle has a different `Time`, record one equity point.
4. At the end: final `orders.Sync()`, build `Result`.

`ninjabot.Run` in backtest mode builds the engine from the bot's existing `PaperWallet`,
`order.Controller` and `strategy.Controller`s (`backtest.NewWithComponents(...)`), replaces
the progress bar with `Progress`, and stores the result for `bot.Result()` (1.4).
`DataFeedSubscription.Start(true)`, `backtestCandles` and the priority queue are no longer
used in backtest mode. The live path (`processCandles`, ticker) is untouched.

**Acceptance.**

- `ninjabot_test.go` golden values unchanged.
- New golden test `backtest/engine_test.go` running `OCOSell`-like LIMIT/OCO flows on
  `testdata/btc-1h.csv`; numbers recorded once and justified in the PR (synchronous fills).
- `TestEngine_Deterministic`: `sha256(result.json)` identical across two sequential runs and
  across four parallel runs over the same `Dataset`, under `-race`.
- Two pairs → `len(result.Equity) == distinct timestamps`.
- `runtime.NumGoroutine()` identical before and after `Run`.
- `BenchmarkEngine_2Pairs_1h` ≥ 5× faster than the 0.4 baseline of `bot.Run`.
- Changelog entry: LIMIT/OCO/STOP fills are now applied synchronously per candle.

### 1.4 `backtest.Result` and `report` package `[x]`

**Motivation.** A versioned, serializable result is the contract every later item (CLI,
portfolio, sweep, agents) depends on. The robustness statistics already exist; they just need
to leave the `ui` package.

**Specification.**

- New package `report`: move the pure part of `ui/statistics.go` (`buildStatistics` and
  helpers) to `report.Build(Input) Statistics` with
  `Input{Pair, Quote string; Trades []Trade; Equity []Point; Candles []Candle}`. Types keep
  their JSON tags; `ui/dto.go` keeps them as aliases and `ui/tygo.yaml` includes `report` so
  `web/src/api/types.ts` keeps being generated. `ui/statistics.go` becomes an adapter.
- `backtest.Result`:

```go
type Result struct {
    SchemaVersion int                         `json:"schema_version"` // 1
    Config        ConfigEcho                  `json:"config"`
    Period        Period                      `json:"period"`   // start, end, candles
    Trades        []Trade                     `json:"trades"`   // chronological
    Orders        []model.Order               `json:"orders"`
    Equity        []Point                     `json:"equity"`   // one per timestamp
    Wallet        WalletSummary               `json:"wallet"`   // initial, final, net/gross profit, fees, volume, buy&hold, drawdown
    Pairs         map[string]report.Statistics `json:"pairs"`
    Portfolio     report.Statistics           `json:"portfolio"` // all trades + wallet equity
    Warnings      []string                    `json:"warnings"`
}
func (r *Result) WriteJSON(w io.Writer) error
func (r *Result) WriteTradesCSV(w io.Writer) error // with header
func (r *Result) WriteEquityCSV(w io.Writer) error
```

- `bot.Result() *backtest.Result`; `Summary()` renders from the result (same text output);
  `SaveReturns` kept for compatibility, documented as superseded by `WriteTradesCSV`.
- Serialization is deterministic: slices sorted explicitly by `(time, id)`; maps are sorted
  by `encoding/json`.

**Acceptance.** `make generate-check` green; `TestResult_JSONDeterministic`; UI statistics
test moved to `report` and passing; `Summary()` output identical to before for the example
backtest.

**Implementation notes.** tygo turns cross-package aliases into `any`, so `ui/tygo.yaml`
now also generates `web/src/api/report.ts` and maps the aliases to its types. The reports
cost ~260 ms per run (25x the run itself): `Config.NoStatistics` skips them, and the
`NinjaBot` façade skips them by default so existing backtests do not slow down. `Summary()`
keeps reading the order controller, so its output is unchanged.

---

## Group 2 — Incremental indicators

Adds stateful, O(1)-per-candle indicators and a strategy style that uses them, while the
current `Indicators(df)` API keeps working.

### 2.1 `indicator/stream` `[x]`

**Specification** (new package, one file per indicator):

```go
type Line interface {          // one value per bar (Close, HL2, ...)
    Update(v float64) float64
    Value() float64; Ready() bool; Warmup() int
    Preview(v float64) float64 // value if v were the current bar, without mutating
    Reset()
}
type Bar interface { Update(c model.Candle) float64; /* same methods */ }
type Multi interface { Outputs() []string; Update(c model.Candle) []float64; Values() []float64; /* ... */ }
type Source func(model.Candle) float64 // Close, Open, High, Low, Volume, HL2, HLC3, OHLC4
func FromLine(l Line, src Source) Bar
```

Implementations with arithmetic identical to go-talib so parity is exact: `SMA`, `EMA`,
`RMA` (Wilder), `RSI`, `ATR`, `Max`, `Min` (monotonic deque), `StdDev`, `OBV`, `MACD`, `BB`,
`Stoch` (SMA smoothing in v1), `SuperTrend` (parity with `indicator/supertrend.go`).
`Preview` is implemented as clone + update in v1.

**Acceptance.**

- `indicator/stream/parity_test.go`: seeded random walk (`rand.NewSource(1)`), N ∈ {30, 100,
  1 000, 5 000}, periods ∈ {2, 3, 5, 9, 14, 21, 50}; every value for `i ≥ Warmup()` matches
  `talib.X(full)[i]` within `1e-9` relative. `Preview` does not mutate (`Value()` before and
  after).
- `BenchmarkStream_*` for each indicator; no allocation per `Update` for Line indicators.

**Implementation notes.** Values are bit-exact with talib in a regular build. Go may fuse
multiply-adds across statements, and does so differently with `-race` on arm64, so the
parity tests allow a relative error of 1e-12. Every indicator updates in 8–45 ns with zero
allocations; `Preview` is checked against `Update` on every bar.

### 2.2 `strategy.StreamingStrategy` `[x]`

**Specification** (`strategy/streaming.go`, `strategy/controller.go`):

```go
type StreamingStrategy interface {
    Strategy
    Setup(pair string, set *IndicatorSet) // once per pair, before the first candle
}
type IndicatorSet struct { /* registration order preserved */ }
func (s *IndicatorSet) Line(name string, l stream.Line, src stream.Source)
func (s *IndicatorSet) Bar(name string, b stream.Bar)
func (s *IndicatorSet) Multi(prefix string, m stream.Multi) // keys prefix_output
func (s *IndicatorSet) Plot(spec ChartSpec)                // what the dashboard draws
```

Controller behaviour for streaming strategies: `Update` only on `Complete` candles, appending
one value to `df.Metadata[name]`; `Preview` on partial candles, overwriting the last value;
`Indicators()` is never called (streaming strategies return `nil`). `Dataframe.Sample` reuses
one map instead of allocating per candle. New options `strategy.WithFullHistory()` (legacy
strategies receive the whole dataframe) and `strategy.WithMaxHistory(n)` (compacts slices
when `len > 2n`).

**Acceptance.**

- New `strategy/controller_test.go`: legacy default reproduces the `ninjabot_test.go` golden;
  legacy + `WithFullHistory` and the streaming equivalent produce the identical sequence of
  broker calls; partial → complete candle with the same `Time` calls `Update` exactly once.
- `BenchmarkController_Streaming` ≥ 20× faster than legacy at W = 200 with ≤ 3 allocs/op;
  `BenchmarkOnPartialCandle` constant between N = 10k and N = 100k.

**Outcome.** Goal met. Partial candles are constant time (78–88 ns at 10k and at 100k candles
of history, against 0.12 ms and 1.1 ms for legacy). A closed candle costs 84 ns with zero
allocations against 2.36 µs for legacy at W = 200 over 100k candles: **28×**
(`go test -run '^$' -bench 'Controller_' -benchtime=100000x ./strategy/`). Getting there took
handing streaming strategies the history instead of a rebuilt window, indicators writing
straight into their series, `*model.Candle` in the `indicator/stream` API (no copies), and
`strategy.WithCapacity`, which the engine sets so the history never grows. Also added:
`strategy.IndicatorObserver` (used by 2.3), `ninjabot.WithStrategyOptions` and
`backtest.Config.ControllerOptions`.

### 2.3 UI without O(N²) `[x]`

**Specification.**

- `strategy.IndicatorObserver{ OnIndicators(pair string, t time.Time, ind []ChartIndicator) }`
  and `Controller.Subscribe(o)`. Legacy strategies deliver the `Indicators()` result that is
  discarded today; streaming strategies deliver the `Plot` spec over `df.Metadata`.
- `ui.Chart` implements the observer and appends one `Point` per metric per candle (O(1));
  `indicatorsByPair` returns the accumulated points; `strategy.Indicators(df)` leaves the hot
  path. `ninjabot.WithCandleSubscription` registers the subscriber as observer when it
  implements the interface (type assertion, no API change).
- `ui/indicator/*` rewritten on `indicator/stream` through an optional
  `ui.StreamIndicator{ Update(model.Candle) []ui.Point }`; legacy `CustomIndicator.Load` is
  only called on `snapshot()` requests.

**Acceptance.** `BenchmarkChart_OnCandle` constant in N; the dashboard shows exactly the values
the strategy saw; `make ui-check` green.

**Implementation notes.** `ui.StreamIndicator` is `CustomIndicator` plus `NewStream()`, one
incremental computation per pair; the eleven built-in dashboard indicators implement it (CCI,
Williams %R and EMA-based Bollinger Bands were added to `indicator/stream` for that, with
parity tests). Third-party `CustomIndicator`s keep the reload path. The chart registers itself
as `IndicatorObserver` through `WithCandleSubscription` (bot) or
`backtest.Config.IndicatorObservers` (engine); without an observer it falls back to
recomputing the strategy indicators. A candle event now costs ~470 ns at 1k and at 10k candles
of history (4.1 µs and 21.5 µs before).

### 2.4 Migrate example strategies `[x]`

**Specification.** `examples/strategies/*` become `StreamingStrategy` with parameter structs
(`CrossEMAParams{Fast, Slow int; Timeframe string}` ...), registered in the registry (3.1).
`examples/strategies/strategies_test.go` holds the new golden numbers. `docs/indicators.md`
explains legacy window semantics versus full-history semantics.

**Acceptance.** Golden tests recorded once and justified; readme snippets updated.

**Implementation notes.** Done right after 3.1, since its dependencies (2.2, 3.1) were in.
`strategy.WithDefaults` keeps the zero values (`new(strategies.CrossEMA)`) working with the
default parameters. Golden numbers (BTC+ETH 1h sample, 0.1% fees, 10 000 USDT):
emacross 26 trades / 43 225.27, turtle 24 / 20 199.19, ocosell 43 / 14 251.40,
trailing 31 / 13 451.10.

---

## Group 3 — Parameterizable strategies and run specs

The first half of the agent interface: describe a run in a file and execute it from the CLI.

### 3.1 `strategy.Registry` `[x]`

**Specification** (`strategy/registry.go`):

```go
type Factory func(params map[string]any) (Strategy, error)
type ParamSpec struct { Name, Type, Description string; Default any; Min, Max *float64; Enum []string }
type Registry struct { /* RWMutex + entries */ }
func NewRegistry() *Registry
var Default = NewRegistry()
func Register(name string, f Factory, params ...ParamSpec)
func (r *Registry) New(name string, params map[string]any) (Strategy, error)
func (r *Registry) Describe() []Description // feeds `ninjabot strategies --json` and JSON Schema
func DecodeParams(params map[string]any, into any) error // strict, applies `default:"..."` tags
```

Example strategies register themselves in `init()` (`emacross`, `turtle`, `ocosell`,
`trailing`); registration only happens when the package is imported.

**Acceptance.** Unknown parameter → error naming it; `Describe()` output validates as JSON
Schema draft 2020-12; `-race` test registering from several goroutines.

**Implementation notes.** Besides `Register`, `strategy.RegisterParams[P]` derives the specs
from the tags of a parameter struct (`json`, `default`, `min`, `max`, `enum`, `description`)
and hands the factory a decoded `P`. `Definition.JSONSchema()` produces the schema;
`ErrUnknownStrategy` and `ErrInvalidParams` back the CLI exit codes.

### 3.2 Run spec `[x]`

**Specification** (`backtest/spec.go`):

```go
type Spec struct {
    SchemaVersion int            `json:"schema_version"`
    Name          string         `json:"name"`
    Data          []DataSource   `json:"data"`      // pair, file, timeframe, heikin_ashi
    Quote         string         `json:"quote"`
    Capital       float64        `json:"capital"`
    Fees          Fees           `json:"fees"`      // maker, taker
    Window        *Window        `json:"window,omitempty"`
    Strategy      string         `json:"strategy"`
    Params        map[string]any `json:"params"`
    Pairs         []string       `json:"pairs"`
    Timeframe     string         `json:"timeframe,omitempty"`
    Seed          int64          `json:"seed"`
}
func ReadSpec(r io.Reader) (Spec, error)      // JSON canonical, DisallowUnknownFields; YAML accepted via conversion
func (s Spec) Validate() error
func (s Spec) Hash() string                    // sha256 of the normalized spec
func FromSpec(ds *Dataset, s Spec, reg *strategy.Registry) (*Engine, error)
```

**Acceptance.** Table-driven validation tests with actionable messages; `Hash()` stable across
key order and whitespace; YAML and JSON of the same spec hash identically.

### 3.3 `ninjabot backtest` CLI `[x]`

**Specification.**

- New reusable package `cli`: `cli.NewApp(reg *strategy.Registry) *cli.App` and
  `cli.Main(reg)`. `cmd/ninjabot` uses it with the example strategies registered. A user with
  private strategies writes `func main() { strategy.Register("mine", ...); cli.Main(strategy.Default) }`
  and gets every command without forking.
- Commands: `backtest --spec <file> [--out <dir>] [--format json|table] [--quiet] [--seed]`,
  `strategies [--json]`, `validate --spec`, `schema spec|result`.
- Artifacts in `--out`: `result.json`, `trades.csv`, `equity.csv`, `orders.csv`, `spec.json`
  (normalized). `--out -` streams `result.json` to stdout; logs always go to stderr.
- Exit codes: `0` ok · `1` runtime error · `2` invalid spec or usage · `3` data error (missing
  CSV, insufficient candles, unknown pair) · `4` unknown strategy or invalid params. Zero
  trades is a `warnings` entry, not a failure.

**Acceptance.** End-to-end test running the example spec over `testdata` and comparing
`result.json` to a golden file; exit code tests for each failure class; readme section.

**Implementation notes.** The end-to-end test checks the golden numbers of `result.json`
(trades, final equity) rather than its bytes: report floats differ in the last bits under
`-race` (see 2.1). `ninjabot schema` is generated from the Go types by
`tools/jsonschema`. Example specs live in `examples/specs`.

---

## Group 4 — Portfolio backtest

### 4.1 Portfolio with isolated sub-wallets `[x]`

**Motivation.** Each strategy trades its own allocated capital in its own `PaperWallet`; the
portfolio equity is the time-aligned sum. Zero changes to the core, trivially parallel,
exact attribution, and it already delivers the smoothing effect we want to measure.

**Specification** (`portfolio` package):

```go
type Spec struct {
    SchemaVersion int            `json:"schema_version"`
    Name          string         `json:"name"`
    Data          []DataSource   `json:"data"`
    Quote         string         `json:"quote"`
    Capital       float64        `json:"capital"`
    Fees          Fees           `json:"fees"`
    Mode          string         `json:"mode"`      // "isolated" (4.1) | "shared" (4.3)
    Window        *Window        `json:"window,omitempty"`
    Strategies    []StrategySpec `json:"strategies"`
}
type StrategySpec struct {
    ID        string         `json:"id"`        // unique, used as tag
    Strategy  string         `json:"strategy"`
    Params    map[string]any `json:"params"`
    Pairs     []string       `json:"pairs"`
    Timeframe string         `json:"timeframe,omitempty"`
    Weight    float64        `json:"weight"`    // fraction of capital; sum ≤ 1
}
type Result struct {
    SchemaVersion int              `json:"schema_version"`
    Spec          Spec             `json:"spec"`
    Strategies    []StrategyResult `json:"strategies"` // id, weight, backtest.Result, contribution
    Combined      Combined         `json:"combined"`
}
type Combined struct {
    Times        []int64              `json:"times"`
    Equity       []float64            `json:"equity"`
    ByStrategy   map[string][]float64 `json:"by_strategy"`  // aligned to Times (forward-fill)
    Statistics   report.Statistics    `json:"statistics"`   // all trades + combined equity
    Correlation  Matrix               `json:"correlation"`  // ids + values, bar returns on the coarsest grid
    Contribution []Contribution       `json:"contribution"` // id, pnl, pnl_share, marginal_sharpe
}
func Run(ctx context.Context, ds *backtest.Dataset, spec Spec, reg *strategy.Registry) (*Result, error)
func AlignForwardFill(curves map[string][]Point) (times []int64, matrix map[string][]float64)
```

`Run` uses `errgroup` limited to `runtime.NumCPU()`; results are assembled in spec order, so
output does not depend on completion order. Correlation uses gonum over bar returns
resampled to the coarsest timeframe among the strategies. `MarginalSharpe_i =
Sharpe(combined) − Sharpe(combined − curve_i)`.

**Acceptance.** Test with two strategies on different timeframes with forward-fill verified
against hand-computed values; `Σ equity_i(t) == combined(t)`; parallel and serial runs hash
identically; `-race` green.

**Implementation notes.** `portfolio.Run(ctx, ds, spec, registry, Options)`; the combined
curve adds the unallocated cash. Besides the spec'd fields, `Combined` carries the drawdown,
the annualized Sharpe of the combined curve and the robustness report over every trade.
Example: `examples/specs/portfolio.json`. Building it surfaced a paper wallet bug: trimming
orders by their fee left balance dust (7e-15 ETH, -1e-12 USDT) that strategies tried to sell,
failing with insufficient funds and recording zero-profit "trades". The wallet now sweeps
balances below 1e-9.

### 4.2 Portfolio CLI and dashboard `[x]`

**Specification.** `ninjabot portfolio --spec <file> --out <dir>` writing `portfolio.json`
plus one `runs/<id>/` folder per strategy. Dashboard: portfolio view with combined and
per-strategy equity curves, per-strategy table, correlation heatmap and contribution bars,
served from `/api/portfolio` with DTOs generated through tygo.

**Acceptance.** `make ui-check` and `make generate-check` green; dashboard screenshot in the PR.

**Implementation notes.** `ninjabot portfolio --spec <file> [--out <dir>|-] [--serve <port>]`
prints the per-strategy table (weight, trades, return, drawdown, Sharpe, PnL share, marginal
Sharpe) and the correlation matrix. `ui.WithPortfolio` serves `GET /api/portfolio`
(`PortfolioView`); the dashboard shows the summary, combined and per-strategy curves, the table
and a correlation heatmap above the pair view, or alone when there is no pair. The panel is
covered by a static-render test; the screenshot is `docs/images/portfolio-dashboard.png`
(headless Chrome with `--timeout`, since the SSE stream never lets the page go idle). In
portfolio-only mode the pair selector is hidden.

### 4.3 Shared wallet with per-strategy attribution (advanced) `[x]`

**Motivation.** Realistic cash sharing between strategies on the same account, including
netting of positions on the same pair. More invasive; built on 4.1.

**Specification.**

- `model.Order.Tag string` (`db`, `json`, `gorm:"index"`; `AutoMigrate` adds the column).
- `order.Controller`: positions and results keyed by `(tag, pair)`; `Trades()` carries the
  tag; `Scoped(tag string) service.Broker` returns a broker that stamps the tag on every
  order, answers `Position(pair)` with the tag's own tracked position and `Account()` with the
  shared balance.
- Engine: one `strategy.Controller` per `(strategyID, pair)`, candle streams per
  `(pair, timeframe)`, dispatch by timeframe; the wallet is priced by the finest stream of
  each pair (documented as a change in LIMIT fill timing; optional `PricingTimeframe`).
- Per-tag equity: allocated cash ± fills + positions × price.

**Acceptance.** `|Σ equity_tag − wallet.Equity()| < 1e-6` over the whole run; migration test on
SQLite; two strategies on the same pair with opposite signals produce the netted position.

**Implementation notes.** `order.Controller.SetBudget` keeps a ledger per tag (cash moved by
its fills, quantity per pair); a scoped broker with a budget reports that ledger as its
position, so strategies size orders from their own allocation. `backtest.RunShared` runs the
strategies with one controller per (tag, pair) and samples the account and every tag at the
same moment; `portfolio` uses it for `"mode": "shared"`. The attribution invariant holds at
every bar for long positions (shorts are valued mark-to-market by the ledgers and at
liquidation value by the wallet). It exposed a wallet bug: selling a position whose balance
was a rounding error below the order size (0.7 + 0.1 BTC, sold as 0.8) opened a short for the
residue and debited the asset twice; such sells are now liquidations.

---

## Group 5 — Experimentation platform for AI agents

The second half of the agent interface: run many experiments, compare them, and guard against
fooling ourselves.

### 5.1 `ninjabot sweep` `[x]`

**Specification.** `sweep.json` = a run spec plus either
`"grid": {"params.fast": [5, 8, 13], "params.slow": [21, 34], "timeframe": ["1h", "4h"]}`
(cartesian product) or `"random": {"samples": 200, "seed": 1, "ranges": {...}}`. Runs execute
in parallel over one shared `Dataset`. Output: `runs.jsonl` (one line per run: params, key
metrics, spec hash), `summary.json` (ranking by `--metric`, default annualized Sharpe),
`runs/<hash>/` with full artifacts when `--full`. Runs whose hash already exists in `--out`
are skipped (cache).

**Acceptance.** Re-running a finished sweep executes nothing; ranking is deterministic; test
with a 2×2 grid over `testdata`.

**Implementation notes.** Ranking uses `backtest.Metrics`, headline numbers every result now
carries (computed in microseconds, unlike the ~260 ms reports, and checked against them).
A failing parameter set is recorded with its error and does not stop the sweep; extending a
grid only runs the new sets. Example: `examples/specs/sweep.json` (3×2×2 grid).

### 5.2 `ninjabot compare` `[x]`

**Specification.** `ninjabot compare <runA> <runB> ... [--metric ...] [--format table|json]`
reads `result.json` or `portfolio.json` from each directory and prints a side-by-side table
with deltas, plus an optional `compare.json`.

**Acceptance.** Golden output test over two example runs.

**Implementation notes.** Package `compare` loads result.json, portfolio.json or their
directories; portfolios are summarized with the same `backtest.Metrics`.

### 5.3 Overfitting guards `[x]`

**Specification.** Spec `window` accepts train/test splits and walk-forward schedules (refit
on a rolling window, score only the following period); PBO/CSCV computed over a sweep's
`runs.jsonl`; fee and slippage sensitivity curves in the report; `warnings` for small samples.
These are the items already listed in the readme roadmap, now anchored on 3.2 and 5.1.

**Acceptance.** Unit tests on synthetic data where the expected PBO is known; documentation of
each statistic in `docs/agents.md`.

**Implementation notes.** Every sweep record keeps its bar returns in 16 time blocks and
`summary.json` reports the PBO over C(16, 8) = 12 870 splits (`metrics.PBO`, tested at ≈0.5
on noise and <0.05 with one real edge). `ninjabot walkforward` rolls train/test windows given
as durations (one fold is a train/test split) and reports the compounded out-of-sample return
and the walk-forward efficiency; `ninjabot sensitivity` reruns a spec over fee levels, which
also stand for slippage. It surfaced one more wallet bug: with zero fees, orders sized with
the whole balance were rejected for float rounding. Windows warm up before they start:
`FromSpec` loads `WarmupPeriod` candles before `window.start` and `backtest.Config.TradeFrom`
feeds them to the indicators only, so a test window shorter than the warmup still trades from
its first candle. On the example sweep the walk-forward efficiency went from 0.16 (folds that
never warmed up) to 0.81.

### 5.4 Agent documentation `[x]`

**Specification.** `docs/agents.md`: the file contract (spec, result, portfolio and sweep
schemas with `schema_version`), recipes (run, sweep, compare, interpret metrics), exit codes
and known pitfalls. An `AGENTS.md` at the repository root with the `make` targets,
conventions and a pointer to this roadmap.

**Acceptance.** A fresh agent session can run a backtest, a sweep and a compare following only
the docs.

**Implementation notes.** Checked with a fresh agent that read only `docs/agents.md`: it ran
a backtest, a 3×3 sweep and a compare. Its report led to fixes: `backtest.Spec.Resolve` makes
specs canonical (defaults explicit, the timeframe in `params.timeframe` with the top-level
field overriding it), used by `validate`, `spec.json` and the sweep hashes, so one run has one
hash; `validate` accepts sweep and portfolio specs; the guide documents flag order, exit codes
with a built binary, relative paths and the compare deltas.

---

## Recommended merge order

```
0.1 → 0.2 → 0.3 → 0.4 → 1.1 → 1.2 → 2.1 → 1.3 → 2.2 → 1.4 → 3.1 → 3.2 → 3.3
→ 4.1 → 2.3 → 5.1 → 5.2 → 4.2 → 4.3 → 2.4 → 5.3 → 5.4
```

2.1 (`indicator/stream`) and 3.1 (`strategy.Registry`) have no dependencies and can be
developed in parallel with Group 0 and Group 1.

## Risks

- **tygo and cross-package aliases (1.4).** The TypeScript types are generated from
  `ui/dto.go`; aliases to `report` types may not expand. Verify with `make generate-check`
  at the start of the PR, and fall back to including `report` in `ui/tygo.yaml`.
- **Result changes (0.1, 1.3, 2.4).** Peak-to-trough drawdown, one equity point per
  timestamp, synchronous LIMIT/OCO fills and full-history indicators all change numbers.
  Each is a correction; each is communicated in the changelog with the golden diff.
- **Double commit on partial candles (2.2).** The rule "`Update` only on complete candles,
  `Preview` on partial ones" must have an explicit test, or `HighFrequencyStrategy` users
  will see indicators advance twice.
- **Global logrus level.** Parallel sweeps share one logger; the CLI sets the level once and
  backtest code paths log at Debug.
- **Dataset memory.** Many pairs × many timeframes can be large; resample results are cached
  per `(pair, timeframe)` key, never per run, and `Window` is a view.
- **Scope creep.** Items in Group 4.3 and Group 5.3 are the most tempting to grow. Keep each
  PR to its acceptance criteria and open follow-up items here instead.

## Benchmarks

Filled in by 0.4 and updated by every item that claims a speed-up. Machine, Go version and
command line are recorded with each row.

Baseline: Apple M1, Go 1.25.3, `go test -run '^$' -bench <name> -benchmem`.

| Benchmark                                  | Baseline (0.4)                     | After 1.3 | After 2.2 | After 2.3 |
|--------------------------------------------|------------------------------------|-----------|-----------|-----------|
| `BenchmarkBot_Run_2Pairs_1h`               | 134.5 ms · 336 MB · 699k allocs    | 40.9 ms · 27 MB · 114k allocs |           |           |
| `BenchmarkController_OnCandle/W=22`        | 553 ns · 1.5 KB · 6 allocs         |           | 365 ns legacy · 88 ns streaming |           |           |
| `BenchmarkController_OnCandle/W=200`       | 2 642 ns · 6.3 KB · 6 allocs       |           | 2 360 ns legacy · 84 ns streaming, 0 allocs (100k candles) |           |           |
| `BenchmarkEngine_2Pairs_1h`                | (added in 1.3)                     | 10.3 ms · 16 MB · 54k allocs (8.3 ms · 10 MB after 2.2) |
| `BenchmarkOnPartialCandle` (10k / 100k)    | (added in 2.2)                     |           | legacy 122 µs / 1.1 ms · streaming 78 ns / 88 ns |           |           |
| `BenchmarkChart_OnCandle/history=1000`     | 4.1 µs · 17 KB                     |           |           | 505 ns · 1 KB |
| `BenchmarkChart_OnCandle/history=10000`    | 21.5 µs · 165 KB                   |           |           | 437 ns · 1 KB |
| `BenchmarkStorage_Orders/buntdb/1000`      | 2.94 ms · 809 KB · 8 010 allocs    |           |           |           |
| `BenchmarkStorage_Orders/memory/1000`      | 19.3 µs · 7 KB · 31 allocs (0.3)   |           |           |           |

`BenchmarkBot_Run` includes CSV parsing and BuntDB storage on every iteration;
`BenchmarkEngine` reuses one `Dataset` and the memory storage, which is how sweeps run.

The chart cost grows linearly with the history on every candle, which makes a backtest with
the dashboard connected quadratic. The bot allocates ~39 KB per candle.

## Working agreement

- Pick the next unchecked item in the merge order unless a dependency is missing or the
  session has a specific need.
- Start a session by reading the item's section and the files it names; end it with the
  acceptance criteria run and the status flipped to `[~]` or `[x]` in this file.
- Each item becomes a GitHub issue when work on it starts (title `ROADMAP <id>: <title>`,
  linking back here), so discussion lives in the issue and the specification lives here.
- Commits follow Conventional Commits (`CONTRIBUTING.md`); result-changing PRs include a
  `BREAKING CHANGE` or `Changelog:` footer describing the semantic change.
