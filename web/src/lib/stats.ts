import type { CandleEvent, Order, OrderEvent, Point, Snapshot } from "../api/types";

export interface Stats {
  /** Closed trades (orders that carry a profit). */
  trades: number;
  wins: number;
  losses: number;
  /** 0..1 */
  winRate: number;
  /** Sum of profit_value, in quote currency. */
  totalProfit: number;
  /** Gross profit / gross loss; Infinity when there are no losses. */
  profitFactor: number;
  /** Average profit per closed trade (fraction, e.g. 0.012). */
  avgReturn: number;
  initialEquity: number;
  finalEquity: number;
  /** (final - initial) / initial, or 0 without equity data. */
  returnPct: number;
  /** Largest peak-to-trough decline of the equity curve (fraction). */
  maxDrawdown: number;
  /** Buy-and-hold return of the asset over the same candles (fraction). */
  buyAndHold: number;
}

export function isClosedTrade(order: Order): boolean {
  return order.status === "FILLED" && (order.profit !== 0 || order.profit_value !== 0);
}

export function maxDrawdown(points: Point[]): number {
  let peak = Number.NEGATIVE_INFINITY;
  let worst = 0;
  for (const p of points) {
    if (p.value > peak) peak = p.value;
    if (peak > 0) {
      const dd = (peak - p.value) / peak;
      if (dd > worst) worst = dd;
    }
  }
  return worst;
}

export function computeStats(snapshot: Snapshot): Stats {
  const closed = snapshot.orders.filter(isClosedTrade);
  const wins = closed.filter((o) => o.profit_value > 0 || (o.profit_value === 0 && o.profit > 0));
  const losses = closed.filter((o) => o.profit_value < 0 || (o.profit_value === 0 && o.profit < 0));
  const grossProfit = wins.reduce((sum, o) => sum + o.profit_value, 0);
  const grossLoss = Math.abs(losses.reduce((sum, o) => sum + o.profit_value, 0));
  const totalProfit = closed.reduce((sum, o) => sum + o.profit_value, 0);
  const avgReturn = closed.length
    ? closed.reduce((sum, o) => sum + o.profit, 0) / closed.length
    : 0;

  const equity = snapshot.equity_values;
  const initialEquity = equity[0]?.value ?? 0;
  const finalEquity = equity[equity.length - 1]?.value ?? 0;
  const returnPct = initialEquity > 0 ? (finalEquity - initialEquity) / initialEquity : 0;

  const firstClose = snapshot.candles[0]?.close ?? 0;
  const lastClose = snapshot.candles[snapshot.candles.length - 1]?.close ?? 0;
  const buyAndHold = firstClose > 0 ? (lastClose - firstClose) / firstClose : 0;

  // The API reports the drawdown as a negative fraction (e.g. -0.117).
  const drawdown =
    snapshot.max_drawdown && snapshot.max_drawdown.value !== 0
      ? Math.abs(snapshot.max_drawdown.value)
      : maxDrawdown(equity);

  return {
    trades: closed.length,
    wins: wins.length,
    losses: losses.length,
    winRate: closed.length ? wins.length / closed.length : 0,
    totalProfit,
    profitFactor:
      grossLoss > 0 ? grossProfit / grossLoss : grossProfit > 0 ? Number.POSITIVE_INFINITY : 0,
    avgReturn,
    initialEquity,
    finalEquity,
    returnPct,
    maxDrawdown: drawdown,
    buyAndHold,
  };
}

/** Appends a point when its time is newer, or replaces the last one when equal. */
export function upsertPoint(points: Point[], point: Point): Point[] {
  const last = points[points.length - 1];
  if (!last || point.time > last.time) return [...points, point];
  if (point.time === last.time) return [...points.slice(0, -1), point];
  return points;
}

/** Applies an SSE candle event to a snapshot immutably. */
export function applyCandleEvent(snapshot: Snapshot, event: CandleEvent): Snapshot {
  if (event.pair !== snapshot.pair) return snapshot;
  const last = snapshot.candles[snapshot.candles.length - 1];
  if (last && event.candle.time <= last.time) return snapshot;

  const updates = new Map(event.indicators.map((i) => [i.name, i.metrics]));
  const indicators = snapshot.indicators.map((indicator) => {
    const metricUpdates = updates.get(indicator.name);
    if (!metricUpdates) return indicator;
    return {
      ...indicator,
      metrics: indicator.metrics.map((metric) => {
        const update = metricUpdates.find((m) => m.name === metric.name);
        return update ? { ...metric, points: upsertPoint(metric.points, update.point) } : metric;
      }),
    };
  });

  return {
    ...snapshot,
    candles: [...snapshot.candles, event.candle],
    indicators,
    equity_values: event.equity
      ? upsertPoint(snapshot.equity_values, event.equity)
      : snapshot.equity_values,
    asset_values: event.asset
      ? upsertPoint(snapshot.asset_values, event.asset)
      : snapshot.asset_values,
  };
}

/** Applies an SSE order event to a snapshot immutably (insert or replace by id). */
export function applyOrderEvent(snapshot: Snapshot, event: OrderEvent): Snapshot {
  if (event.pair !== snapshot.pair) return snapshot;
  const index = snapshot.orders.findIndex((o) => o.id === event.order.id);
  const orders = [...snapshot.orders];
  if (index >= 0) orders[index] = event.order;
  else orders.push(event.order);
  return { ...snapshot, orders };
}
