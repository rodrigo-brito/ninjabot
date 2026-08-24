---
title: "Backtesting"
linkTitle: "Backtesting"
categories: ["Reference"]
weight: 3
description: >
  This page describes the paper wallet used in backtesting and paper trading, the trading fees, the short positions and how to read the results.
---

## Paper Wallet

The paper wallet is a virtual exchange: it fills your orders against the candles of a data feed, keeping balances, positions and trade results, without touching a real account. It powers both **backtesting** (historical data from CSV) and **paper trading** (live data, fake money).

```go
wallet := exchange.NewPaperWallet(
	ctx,
	"USDT", // quote currency
	exchange.WithPaperAsset("USDT", 10000), // initial balance
	exchange.WithPaperFee(0.001, 0.001),    // maker and taker fees
	exchange.WithDataFeed(csvFeed),         // csvFeed for backtest, or a Binance instance for paper trading
)
```

For backtesting, the wallet must also be informed to the bot with `ninjabot.WithBacktest(wallet)`, which optimizes the CSV reading and processes the candles in chronological order. In paper trading, use `ninjabot.WithPaperWallet(wallet)` instead.

## Trading Fees

`WithPaperFee(maker, taker)` charges the exchange fees on every fill, so your results are net of costs. `0.001` (0.1%) is the Binance spot default.

- Orders that rest on the book - **limit** and take profit orders - pay the **maker** fee.
- Orders that cross the spread - **market** orders and triggered stops - pay the **taker** fee.

The fee is charged in quote currency, stored in `Order.Fee`, and deducted from the profit of each trade. The results table, the dashboard and the `NET PROFIT` line all report returns after costs.

An order sized with the entire available balance leaves no room for its fee. Instead of rejecting it, the wallet fills a slightly smaller quantity.

{{% pageinfo %}}
Without `WithPaperFee` no fee is charged, and the simulation will look better than reality. Always backtest with the fees of the exchange you intend to use.
{{% /pageinfo %}}

## Short positions

A sell bigger than your asset balance opens a **short position**, so strategies for the futures market can be simulated. The short is modelled as a negative asset balance backed by collateral taken from the quote balance:

```go
// with no BTC in the wallet, this opens a short of 1 BTC
_, err := broker.CreateOrderMarket(ninjabot.SideTypeSell, "BTCUSDT", 1)
```

The collateral is `price * quantity`, so a wallet with 200 USDT can short 2 BTC at 100 USDT - the quote balance goes to zero while the position is open. The buy that covers the short liquidates the position, returning the collateral plus the profit (or minus the loss) of the move: covering those 2 BTC at 50 USDT returns 300 USDT.

Limit and stop orders open and close shorts the same way, locking the collateral until they fill. An order that only partially covers an open short is settled in two parts: the covering share against the short, the remainder as a new position. The equity displayed by the dashboard, the `/balance` command and the final summary all account for the open short at the current price.

## Reading the results

`bot.Summary()` prints the results table, the distribution of the trade returns and a bootstrapped 95% confidence interval for the return, payoff and profit factor of each pair.

```
+---------+--------+-----+------+--------+--------+----------+-----+----------+-----------+
|  PAIR   | TRADES | WIN | LOSS | % WIN  | PAYOFF | PR FACT. | SQN |  PROFIT  |  VOLUME   |
+---------+--------+-----+------+--------+--------+----------+-----+----------+-----------+
| BTCUSDT |     21 |   7 |   14 | 33.3 % |  5.743 |    2.871 | 1.4 | 12759.66 | 437832.11 |
| ETHUSDT |      9 |   5 |    4 | 55.6 % |  4.803 |    6.004 | 1.3 | 20454.68 | 393786.29 |
+---------+--------+-----+------+--------+--------+----------+-----+----------+-----------+
|   TOTAL |     30 |  12 |   18 | 40.0 % |  5.461 |    3.811 | 1.3 | 33214.34 | 831618.40 |
+---------+--------+-----+------+--------+--------+----------+-----+----------+-----------+
```

| Column | Meaning |
|---|---|
| `TRADES`, `WIN`, `LOSS`, `% WIN` | Number of closed trades and the hit rate. |
| `PAYOFF` | Average win divided by the average loss, in percentage of the position. |
| `PR FACT.` | Profit factor: sum of the winning returns divided by the sum of the losing ones. |
| `SQN` | System Quality Number (Van Tharp): `sqrt(trades) * mean / stddev` of the trade results. Needs at least two trades with distinct results, and is `0` otherwise. |
| `PROFIT` | Net profit of the pair, after fees, in quote currency. |
| `VOLUME` | Total traded volume, in quote currency. |

The wallet summary that follows reports the final balances, and the returns of the portfolio:

```
----- RETURNS -----
START PORTFOLIO     = 10000.00 USDT
FINAL PORTFOLIO     = 43214.33 USDT
GROSS PROFIT        =  34045.951221 USDT (340.46%)
TRADING FEES        =  831.618401 USDT
NET PROFIT          =  33214.332820 USDT (332.14%)
MARKET CHANGE (B&H) =  407.09%

------ RISK -------
MAX DRAWDOWN = -11.76 %
```

`MARKET CHANGE (B&H)` is the buy and hold return of the same period - if your strategy is below it, simply buying the asset at the start would have performed better. `MAX DRAWDOWN` is the largest drop from a peak of the equity curve.

### Exporting the returns

The raw data of every trade is available in `bot.Controller().Results`, and `bot.SaveReturns(dir)` writes one file per pair with the return of each trade, one per line, ready to be analysed elsewhere. The directory must already exist.

```go
bot.Summary()

// ./returns/BTCUSDT.csv, ./returns/ETHUSDT.csv
if err := bot.SaveReturns("./returns"); err != nil {
	log.Fatal(err)
}
```

The orders of each pair can also be downloaded from the dashboard, in `GET /api/{pair}/orders.csv`.
