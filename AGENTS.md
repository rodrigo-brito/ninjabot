# Working on NinjaBot

Guidance for AI agents and new contributors. Human-oriented details live in
[CONTRIBUTING.md](CONTRIBUTING.md).

## Using NinjaBot to test strategies

Read [docs/agents.md](docs/agents.md): run specs, the `ninjabot` commands, the result files,
the metrics and the overfitting guards. Example specs are in [examples/specs](examples/specs).

## Changing the code

```bash
make test            # go test -race -cover ./... — must stay green
make lint            # golangci-lint
make bench           # benchmarks; compare runs with benchstat
make generate        # regenerate web/src/api/types.ts and report.ts after changing ui/dto.go or report/
make ui-check        # web: biome, tsc, vitest (requires bun)
```

- The plan of the backtesting work, with the status of every item, is
  [ROADMAP.md](ROADMAP.md). Changes that alter the numbers of an existing backtest are listed
  under **Result changes** in [CHANGELOG.md](CHANGELOG.md), with the reason.
- Determinism first: a backtest is a pure function of its spec and data. No timers or
  goroutine ordering in the backtest path; randomized statistics take an explicit seed.
- Golden tests (`ninjabot_test.go`, `backtest/engine_test.go`, `examples/strategies`) pin
  backtest numbers. Update them only for a deliberate, documented correction.
- Compare floats with a tolerance: `-race` on arm64 changes multiply-add fusion and the last
  bits of some results.

## Map

| Package | Role |
|---|---|
| `backtest` | `Dataset` (shared, cached), deterministic `Engine`, `Spec`, `Result`, `Metrics`, `RunShared` |
| `portfolio` | several strategies, isolated or shared wallet, combined curve, correlation, contribution |
| `sweep` | grids and random samples, cache by spec hash, PBO, walk-forward, fee sensitivity |
| `compare` | side-by-side metrics of result files |
| `cli`, `cmd/ninjabot` | the command line, reusable with custom strategies |
| `strategy` | `Strategy`, `StreamingStrategy`, controller, `Registry` |
| `indicator`, `indicator/stream` | talib wrappers; incremental indicators matching talib |
| `report`, `tools/metrics` | robustness statistics |
| `exchange` | paper wallet, CSV parsing/resampling, Binance |
| `order` | order controller, positions, trades, tags and ledgers |
| `ui`, `web` | dashboard API and React front-end |
