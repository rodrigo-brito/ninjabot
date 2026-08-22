import { describe, expect, it } from "vitest";
import type { Order, Snapshot } from "../api/types";
import { applyCandleEvent, applyOrderEvent, computeStats, maxDrawdown, upsertPoint } from "./stats";

function order(partial: Partial<Order>): Order {
  return {
    id: 1,
    exchange_id: 1,
    pair: "BTCUSDT",
    side: "BUY",
    type: "MARKET",
    status: "FILLED",
    price: 100,
    quantity: 1,
    created_at: 0,
    updated_at: 0,
    candle_time: 0,
    ref_price: 0,
    profit: 0,
    profit_value: 0,
    ...partial,
  };
}

function snapshot(partial: Partial<Snapshot> = {}): Snapshot {
  return {
    pair: "BTCUSDT",
    asset: "BTC",
    quote: "USDT",
    candles: [],
    indicators: [],
    orders: [],
    equity_values: [],
    asset_values: [],
    ...partial,
  };
}

describe("maxDrawdown", () => {
  it("returns 0 for flat or rising curves", () => {
    expect(maxDrawdown([])).toBe(0);
    expect(
      maxDrawdown([
        { time: 1, value: 100 },
        { time: 2, value: 110 },
      ]),
    ).toBe(0);
  });

  it("finds the largest peak to trough", () => {
    const dd = maxDrawdown([
      { time: 1, value: 100 },
      { time: 2, value: 80 },
      { time: 3, value: 120 },
      { time: 4, value: 90 },
      { time: 5, value: 130 },
    ]);
    expect(dd).toBeCloseTo(0.25);
  });
});

describe("computeStats", () => {
  it("handles an empty snapshot", () => {
    const s = computeStats(snapshot());
    expect(s.trades).toBe(0);
    expect(s.winRate).toBe(0);
    expect(s.profitFactor).toBe(0);
    expect(s.returnPct).toBe(0);
    expect(s.maxDrawdown).toBe(0);
  });

  it("aggregates closed trades and equity", () => {
    const s = computeStats(
      snapshot({
        candles: [
          { time: 1, open: 1, high: 1, low: 1, close: 100, volume: 1 },
          { time: 2, open: 1, high: 1, low: 1, close: 150, volume: 1 },
        ],
        orders: [
          order({ id: 1 }), // opening order, no profit
          order({ id: 2, side: "SELL", profit: 0.1, profit_value: 10 }),
          order({ id: 3, side: "SELL", profit: -0.05, profit_value: -5 }),
          order({ id: 4, side: "SELL", profit: 0.2, profit_value: 20 }),
          order({ id: 5, side: "SELL", status: "CANCELED", profit: 1, profit_value: 100 }),
        ],
        equity_values: [
          { time: 1, value: 1000 },
          { time: 2, value: 900 },
          { time: 3, value: 1200 },
        ],
        max_drawdown: { value: -0.1, start: 1, end: 2 },
      }),
    );
    expect(s.trades).toBe(3);
    expect(s.wins).toBe(2);
    expect(s.losses).toBe(1);
    expect(s.winRate).toBeCloseTo(2 / 3);
    expect(s.totalProfit).toBe(25);
    expect(s.profitFactor).toBeCloseTo(6);
    expect(s.avgReturn).toBeCloseTo(0.25 / 3);
    expect(s.initialEquity).toBe(1000);
    expect(s.finalEquity).toBe(1200);
    expect(s.returnPct).toBeCloseTo(0.2);
    expect(s.maxDrawdown).toBeCloseTo(0.1);
    expect(s.buyAndHold).toBeCloseTo(0.5);
  });

  it("falls back to the equity curve when the API has no drawdown", () => {
    const s = computeStats(
      snapshot({
        equity_values: [
          { time: 1, value: 100 },
          { time: 2, value: 50 },
        ],
      }),
    );
    expect(s.maxDrawdown).toBeCloseTo(0.5);
  });
});

describe("upsertPoint", () => {
  it("appends, replaces or ignores by time", () => {
    const base = [{ time: 1, value: 1 }];
    expect(upsertPoint(base, { time: 2, value: 2 })).toEqual([
      { time: 1, value: 1 },
      { time: 2, value: 2 },
    ]);
    expect(upsertPoint(base, { time: 1, value: 9 })).toEqual([{ time: 1, value: 9 }]);
    expect(upsertPoint(base, { time: 0, value: 9 })).toBe(base);
    expect(upsertPoint([], { time: 0, value: 9 })).toEqual([{ time: 0, value: 9 }]);
  });
});

describe("applyCandleEvent", () => {
  const base = snapshot({
    candles: [{ time: 10, open: 1, high: 1, low: 1, close: 1, volume: 1 }],
    indicators: [
      {
        name: "EMA",
        overlay: true,
        metrics: [{ name: "ema", color: "red", style: "line", points: [{ time: 10, value: 1 }] }],
      },
    ],
    equity_values: [{ time: 10, value: 100 }],
  });

  it("ignores other pairs and stale candles", () => {
    const candle = { time: 20, open: 1, high: 1, low: 1, close: 2, volume: 1 };
    expect(applyCandleEvent(base, { pair: "ETHUSDT", candle, indicators: [] })).toBe(base);
    expect(
      applyCandleEvent(base, { pair: "BTCUSDT", candle: { ...candle, time: 10 }, indicators: [] }),
    ).toBe(base);
  });

  it("appends candle, indicator points and equity", () => {
    const next = applyCandleEvent(base, {
      pair: "BTCUSDT",
      candle: { time: 20, open: 1, high: 1, low: 1, close: 2, volume: 1 },
      indicators: [{ name: "EMA", metrics: [{ name: "ema", point: { time: 20, value: 1.5 } }] }],
      equity: { time: 20, value: 110 },
    });
    expect(next.candles).toHaveLength(2);
    expect(next.indicators[0]?.metrics[0]?.points).toEqual([
      { time: 10, value: 1 },
      { time: 20, value: 1.5 },
    ]);
    expect(next.equity_values).toHaveLength(2);
    expect(next.asset_values).toHaveLength(0);
    expect(base.candles).toHaveLength(1); // immutable
  });
});

describe("applyOrderEvent", () => {
  it("inserts new orders and replaces existing ones", () => {
    const base = snapshot({ orders: [order({ id: 1, status: "NEW" })] });
    const inserted = applyOrderEvent(base, { pair: "BTCUSDT", order: order({ id: 2 }) });
    expect(inserted.orders.map((o) => o.id)).toEqual([1, 2]);

    const replaced = applyOrderEvent(inserted, {
      pair: "BTCUSDT",
      order: order({ id: 1, status: "FILLED" }),
    });
    expect(replaced.orders[0]?.status).toBe("FILLED");
    expect(replaced.orders).toHaveLength(2);

    expect(applyOrderEvent(base, { pair: "ETHUSDT", order: order({ id: 3 }) })).toBe(base);
  });
});
