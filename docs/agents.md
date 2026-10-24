# NinjaBot for agents and scripts

This guide is for an AI agent (or any script) that tests trading ideas with NinjaBot without
writing Go: describe a run in a file, execute it with the `ninjabot` command line, read the
results from versioned JSON and CSV files. Everything is deterministic: the same spec and data
always produce the same files.

- [Commands](#commands)
- [Exit codes](#exit-codes)
- [Files](#files)
- [Metrics](#metrics)
- [Recipes](#recipes)
- [Pitfalls](#pitfalls)

## Commands

Build the binary once, so exit codes reach your shell (`go run` always exits with 1 on
failure):

```bash
go build -o ninjabot ./cmd/ninjabot      # or: go install github.com/rodrigo-brito/ninjabot/cmd/ninjabot@latest
```

**Flags go before the positional arguments**: `ninjabot compare -f json runs/a runs/b`, not
`ninjabot compare runs/a runs/b -f json`.

```bash
ninjabot strategies [--json]                       # strategies, parameters, defaults and bounds
ninjabot schema spec|result|strategy <name>        # JSON Schema of a file or of a strategy's params
ninjabot validate   --spec run.json                # check a run, sweep or portfolio spec, params and data files
ninjabot backtest   --spec run.json --out runs/a   # one run
ninjabot sweep      --spec sweep.json --out runs/grid [--metric sharpe_annual] [--top 20] [--full]
ninjabot walkforward --spec sweep.json --train 90d --test 30d [--metric sharpe_annual] [-f json]
ninjabot sensitivity --spec run.json [--fee 0 --fee 0.001 ...] [-f json]
ninjabot portfolio  --spec portfolio.json --out runs/basket [--serve 8080]
ninjabot compare    [-m net_profit -m sharpe_annual] [-f json] runs/a runs/b runs/basket
ninjabot download   --pair BTCUSDT --timeframe 1h --days 365 --output btc-1h.csv
```

- Results go to stdout or `--out`; logs and errors go to stderr. Use `--quiet` / `-q` to keep
  stdout empty, `--out -` to stream `result.json` (or `portfolio.json`) to stdout, and
  `-f json` for machine-readable tables.
- `--no-statistics` skips the robustness reports (`pairs`, `portfolio`), which cost about
  250 ms per run. `metrics` are always computed.
- The `ninjabot` binary knows the example strategies (`emacross`, `turtle`, `ocosell`,
  `trailing`). Their parameters, types, defaults and bounds come from `ninjabot strategies`
  (or `ninjabot schema strategy <name>`): read them before writing `params`. Strategies
  written in Go are exposed with package `cli`; see the readme.
- `sweep` ranks by `sharpe_annual` unless `--metric` says otherwise; it prints the overfitting
  estimate and a table of the top runs (rank, hash prefix, metric, return, drawdown, trades,
  params).

## Exit codes

| Code | Meaning | What to do |
|---|---|---|
| 0 | success | read the outputs |
| 1 | runtime error | read stderr; usually a bug, report it |
| 2 | invalid spec or arguments | fix the field named in stderr; all problems are listed at once |
| 3 | data error | a CSV is missing, empty or too short for the window/timeframe |
| 4 | unknown strategy or invalid parameters | run `ninjabot strategies` and fix the params |

## Files

All specs are JSON (canonical) or YAML. Unknown fields are rejected, so a typo never falls
back to a default. Relative data paths are resolved against the directory of the spec.

### Run spec (`backtest`, `validate`, `sensitivity`)

```json
{
  "schema_version": 1,
  "name": "optional label",
  "data": [{"pair": "BTCUSDT", "file": "btc-1h.csv", "timeframe": "1h", "heikin_ashi": false}],
  "quote": "USDT",
  "capital": 10000,
  "fees": {"maker": 0.001, "taker": 0.001},
  "strategy": "emacross",
  "params": {"fast": 8, "slow": 21, "timeframe": "4h"},
  "pairs": ["BTCUSDT"],
  "timeframe": "4h",
  "window": {"start": "2021-01-01", "end": "2021-03-31"},
  "seed": 1
}
```

- Required: `schema_version`, `data`, `quote`, `capital`, `fees`, `strategy`.
- `pairs` defaults to every pair of `data`; `params` missing from the spec take their defaults.
- **Timeframe.** Set it in `params.timeframe` when the strategy has that parameter (all the
  examples do). A top-level `timeframe` overrides it: the run, `config.timeframe` and
  `config.params.timeframe` then all use the top-level value. The top-level field exists for
  strategies without a timeframe parameter. `seed` seeds the randomized statistics of
  the reports (bootstrap, Monte Carlo, permutation tests); it defaults to 1, so reports are
  reproducible, and changing it shows how much they depend on the draw.
- CSV files have `time,open,close,low,high,volume` (Unix seconds), with or without header;
  `ninjabot download` writes them. Data in a finer timeframe is resampled.
- Relative `file` paths are resolved against the directory of the spec file, not the current
  directory. A spec stored away from its data needs absolute paths.
- `validate` prints `ok <hash>`: the sha256 of the **resolved** spec — pairs explicit, every
  parameter explicit (defaults included), timeframe in `params.timeframe`. The same run has the
  same hash whatever its key order, format, or whether a value was written or defaulted, and
  it is the hash `sweep` records in `runs.jsonl`. `validate` also accepts sweep specs (prints
  the number of runs) and portfolio specs.

### Sweep spec (`sweep`, `walkforward`)

A run spec plus exactly one of:

```json
{"grid": {"params.fast": [5, 8, 13], "params.slow": [21, 34], "timeframe": ["4h", "1d"]}}
{"random": {"samples": 200, "seed": 7, "ranges": {
  "params.fast": {"min": 3, "max": 20, "integer": true},
  "params.size": {"min": 0.1, "max": 0.5},
  "timeframe":   {"values": ["4h", "1d"]}}}}
```

Keys vary a strategy parameter (`params.<name>`) or the spec `timeframe`.

### Portfolio spec (`portfolio`)

```json
{
  "schema_version": 1,
  "data": [...], "quote": "USDT", "capital": 10000, "fees": {"maker": 0.001, "taker": 0.001},
  "mode": "isolated",
  "strategies": [
    {"id": "trend", "strategy": "emacross", "params": {"timeframe": "4h"}, "weight": 0.5},
    {"id": "breakout", "strategy": "turtle", "params": {"timeframe": "1d"}, "pairs": ["BTCUSDT"], "weight": 0.3}
  ]
}
```

- `weight` is the fraction of the capital of each strategy; the weights add up to at most 1
  and the rest stays as cash.
- `"mode": "isolated"` (default) gives each strategy its own wallet; `"shared"` runs them on
  one wallet with the whole capital, each strategy seeing its weight as its own cash and its
  own holdings as its positions.

### `result.json` (schema version 1)

| Field | Content |
|---|---|
| `config` | strategy name, resolved params (with defaults), pairs, timeframe, quote, balances, fees |
| `period` | first and last candle, number of candles processed |
| `wallet` | initial, final, net and gross profit, fees, return, buy and hold, max drawdown (value, start, end), volume per pair |
| `metrics` | the headline numbers below; what sweeps rank and `compare` shows |
| `trades` | closed trades in closing order: pair, side, profit (fraction and quote), duration, times, order id, tag |
| `orders` | every order, by id |
| `equity` | the wallet value, one point per candle time |
| `pairs` | robustness report per pair (absent with `--no-statistics`) |
| `portfolio` | robustness report over every trade and the wallet equity |
| `warnings` | e.g. no trades, fewer than 20 trades |

`trades.csv`, `equity.csv` and `orders.csv` carry the same data with a header; `spec.json` is
the resolved spec (see `validate`), runnable as is.

`period.candles` counts the candles the engine processed over all pairs, including the
partial candles of a resampled timeframe, so it changes with the timeframe.

### Sweep outputs

- `runs.jsonl`: one line per run — `hash`, resolved `params`, `timeframe`, `metrics`, `blocks`
  (bar returns in 16 time blocks) and `error` for a failed run. Runs whose hash is already in
  the file are reused, so rerunning or extending a sweep only runs what is missing.
- `summary.json`: `runs`, `cached`, `failed`, `overfitting` (see below) and `ranking`, the best
  records by `--metric`.
- `runs/<hash>/` with the full artifacts of every run when `--full` is set. Without it there
  are no per-run files: to inspect or `compare` one run, rerun its params with `backtest`.

### Portfolio outputs

`portfolio.json` holds the spec, every strategy's full result and `combined`: aligned `times`,
`equity` and `by_strategy` curves, `unallocated` cash, return, drawdown, annualized Sharpe,
`correlation` (bar returns on the coarsest timeframe), `contribution` (PnL share and marginal
Sharpe per strategy) and a robustness report. In the shared mode, `account` is the shared
wallet. `equity.csv` has one column per strategy; `runs/<id>/` has each strategy's artifacts.

## Metrics

| Metric | Meaning | Better |
|---|---|---|
| `trades` | closed trades | — |
| `net_profit` | final minus initial wallet value, in quote | higher |
| `return_pct` | net profit over the initial value (0.25 = 25%) | higher |
| `buy_and_hold` | average return of holding the traded pairs | reference |
| `max_drawdown` | largest peak-to-trough fall of the equity (0.2 = 20%) | lower |
| `win_rate` | share of trades with a positive return | higher |
| `profit_factor` | gross gains over gross losses of the trade returns (10 when no loss) | higher |
| `payoff` | average gain over average loss | higher |
| `expectancy` | average profit per trade, in quote | higher |
| `sqn` | System Quality Number: √n · mean / std of the trade profits | higher |
| `sharpe_annual` / `sortino_annual` | annualized ratios of the bar returns of the equity | higher |
| `calmar` | annualized return over the max drawdown | higher |
| `fees` | fees paid, in quote | lower |

The robustness reports (`pairs`, `portfolio`) add bootstrap confidence intervals of the mean
return, payoff and profit factor; hypothesis tests with verdicts (`pass`, `warn`,
`inconclusive`): mean return t-test and bootstrap, runs test of independence, stability
between halves; value at risk and expected shortfall; probabilistic Sharpe and minimum track
record; a Monte Carlo of resampled trade sequences (final return and drawdown percentiles,
probability of profit and of ruin at a 20% drawdown); the return histogram and four
equal-trade windows.

Overfitting guards:

- **Probability of backtest overfitting** (`summary.json` → `overfitting.pbo`): the share of
  the 12 870 ways of splitting the period in two halves where the best run of one half ranks
  at or below the median in the other. Near 0 the ranking generalizes; near 0.5 or above it is
  noise. Needs at least two successful runs.
- **Walk-forward efficiency** (`walkforward`): the mean annualized out-of-sample return over
  the mean annualized in-sample return of the chosen parameters. Near 1 the optimization
  generalizes; near 0 or negative it fits noise. The compounded out-of-sample return is the
  honest estimate of the strategy.
- **Fee sensitivity** (`sensitivity`): the metrics at several fee levels. Slippage behaves
  like an extra fee per fill, so the level where the return vanishes is the cost the strategy
  survives. Compare it with `break_even_fee` in the reports.

## Recipes

**Test an idea.** Copy `examples/specs/emacross.json`, change `strategy`/`params`, run
`ninjabot validate` then `ninjabot backtest --out runs/idea`, and read `metrics`, `warnings`
and the `portfolio` report of `result.json`. Distrust results with few trades or
`inconclusive` tests.

**Tune parameters without fooling yourself.** Run a `sweep` over a coarse grid, check
`overfitting.pbo` in `summary.json`, then run `walkforward` with the same spec. Prefer
parameters that rank well in many folds over the single best run.

**Combine strategies.** Write a portfolio spec with the candidates, run `portfolio`, and look
at `combined.max_drawdown` versus each strategy's, the correlation matrix and each strategy's
marginal Sharpe: a strategy with a positive marginal Sharpe improves the basket even if it is
weak alone.

**Compare runs.** `ninjabot compare -f json runs/a runs/b` gives the metrics of each run and
their deltas to the first one (`run - first`; the first delta is all zeros). It compares
`trades`, `net_profit`, `return_pct`, `max_drawdown`, `win_rate`, `profit_factor`,
`sharpe_annual`, `sortino_annual` and `calmar` unless `-m` picks others.

## Pitfalls

- **Windows warm up before they start.** With `window.start`, the engine also loads the
  `WarmupPeriod` of the strategy before it (when the data has it): those candles feed the
  indicators only, and trading, the wallet and every measure start at `window.start`. Walk-forward
  folds work the same way. A window at the very start of the data has nothing to warm up
  with, so its first candles only warm up.
- **Timeframes.** The strategy timeframe must be a multiple of the data timeframe; resampling
  feeds partial candles to high-frequency strategies before each closed one.
- **Shared mode** values short positions at mark-to-market in the attribution and at
  liquidation value in the wallet; the attribution adds up exactly for long positions.
- **Statistics cost.** Reports run thousands of resamples; use `--no-statistics` when only
  `metrics` matter (sweeps do by default unless `--full`).
- **Determinism.** Runs are byte-for-byte reproducible on one machine and build. Floats may
  differ in the last bits across build modes (`-race`) or architectures; compare numbers with
  a tolerance.
