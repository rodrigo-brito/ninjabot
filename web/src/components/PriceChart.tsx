import {
  type CandlestickData,
  CandlestickSeries,
  ColorType,
  CrosshairMode,
  createChart,
  createSeriesMarkers,
  type HistogramData,
  HistogramSeries,
  type IChartApi,
  type ISeriesApi,
  type ISeriesMarkersPluginApi,
  type LineData,
  LineSeries,
  type SeriesMarker,
  type SeriesType,
  type Time,
  type UTCTimestamp,
} from "lightweight-charts";
import { useEffect, useRef } from "react";
import type { Candle, Indicator, Metric, Order, Snapshot } from "../api/types";
import { type TradeLine, TradeLinesPrimitive } from "../chart/tradeLines";
import { formatCompact, formatPercent } from "../lib/format";
import { type Palette, palettes, type Theme } from "../lib/theme";

interface Props {
  snapshot: Snapshot;
  theme: Theme;
}

interface MetricSeries {
  series: ISeriesApi<SeriesType>;
  count: number;
}

interface ChartState {
  chart: IChartApi;
  candles: ISeriesApi<"Candlestick">;
  volume: ISeriesApi<"Histogram">;
  markers: ISeriesMarkersPluginApi<Time>;
  tradeLines: TradeLinesPrimitive;
  metrics: Map<string, MetricSeries>;
  paneByIndicator: Map<string, number>;
  pair: string;
  candleCount: number;
}

const t = (unix: number) => unix as UTCTimestamp;

function toCandle(c: Candle): CandlestickData<Time> {
  return { time: t(c.time), open: c.open, high: c.high, low: c.low, close: c.close };
}

function toVolume(c: Candle, palette: Palette): HistogramData<Time> {
  return {
    time: t(c.time),
    value: c.volume,
    color: c.close >= c.open ? palette.volumeUp : palette.volumeDown,
  };
}

function metricKey(indicator: Indicator, metric: Metric, index: number): string {
  return `${indicator.name}::${metric.name || index}`;
}

function metricData(metric: Metric): (LineData<Time> | HistogramData<Time>)[] {
  const out: (LineData<Time> | HistogramData<Time>)[] = [];
  let lastTime = Number.NEGATIVE_INFINITY;
  for (const p of metric.points) {
    if (!Number.isFinite(p.value) || p.time <= lastTime) continue;
    lastTime = p.time;
    out.push({ time: t(p.time), value: p.value });
  }
  return out;
}

function createMetricSeries(
  chart: IChartApi,
  metric: Metric,
  paneIndex: number,
  palette: Palette,
): ISeriesApi<SeriesType> {
  const color = metric.color || palette.accent;
  switch (metric.style) {
    case "bar":
    case "histogram":
    case "waterfall":
      return chart.addSeries(
        HistogramSeries,
        { color, priceLineVisible: false, lastValueVisible: false, title: metric.name },
        paneIndex,
      );
    case "scatter":
      return chart.addSeries(
        LineSeries,
        {
          color,
          lineVisible: false,
          pointMarkersVisible: true,
          pointMarkersRadius: 2,
          priceLineVisible: false,
          lastValueVisible: false,
          title: metric.name,
        },
        paneIndex,
      );
    default:
      return chart.addSeries(
        LineSeries,
        {
          color,
          lineWidth: 1,
          priceLineVisible: false,
          lastValueVisible: false,
          crosshairMarkerVisible: false,
          title: metric.name,
        },
        paneIndex,
      );
  }
}

function buildMarkers(orders: Order[], candles: Candle[], palette: Palette): SeriesMarker<Time>[] {
  const times = new Set(candles.map((c) => c.time));
  const markers: SeriesMarker<Time>[] = [];
  for (const order of orders) {
    if (order.status !== "FILLED" || !times.has(order.candle_time)) continue;
    const buy = order.side === "BUY";
    const profit = order.profit !== 0 ? ` ${formatPercent(order.profit, 1)}` : "";
    markers.push({
      time: t(order.candle_time),
      position: buy ? "belowBar" : "aboveBar",
      shape: buy ? "arrowUp" : "arrowDown",
      color: buy ? palette.up : palette.down,
      text: `${buy ? "B" : "S"} ${formatCompact(order.quantity)}${profit}`,
    });
  }
  markers.sort((a, b) => (a.time as number) - (b.time as number));
  return markers;
}

/**
 * OCO orders (stop loss / limit maker) are drawn as a line from the price
 * where they were created (ref_price) to their trigger price, over the time
 * they were open. Green for take-profit targets, red for stop losses.
 */
function buildTradeLines(orders: Order[], candles: Candle[], palette: Palette): TradeLine[] {
  if (candles.length === 0) return [];
  const first = candles[0]?.time ?? 0;
  const last = candles[candles.length - 1]?.time ?? 0;
  const snap = (time: number) => {
    let result = first;
    for (const c of candles) {
      if (c.time > time) break;
      result = c.time;
    }
    return Math.min(Math.max(result, first), last);
  };

  const lines: TradeLine[] = [];
  for (const order of orders) {
    if (order.type !== "STOP_LOSS" && order.type !== "LIMIT_MAKER") continue;
    if (!order.ref_price || !order.price) continue;
    const isStop = order.type === "STOP_LOSS";
    lines.push({
      fromTime: snap(order.created_at),
      fromPrice: order.ref_price,
      toTime: snap(order.updated_at),
      toPrice: order.price,
      color: isStop ? palette.down : palette.up,
      dashed: order.status !== "FILLED",
    });
  }
  return lines;
}

function chartOptions(palette: Palette) {
  return {
    autoSize: true,
    layout: {
      background: { type: ColorType.Solid, color: palette.background },
      textColor: palette.text,
      attributionLogo: false,
      panes: {
        separatorColor: palette.border,
        separatorHoverColor: palette.accent,
        enableResize: true,
      },
    },
    grid: { vertLines: { color: palette.grid }, horzLines: { color: palette.grid } },
    crosshair: {
      mode: CrosshairMode.Normal,
      vertLine: { color: palette.crosshair, labelBackgroundColor: palette.crosshair },
      horzLine: { color: palette.crosshair, labelBackgroundColor: palette.crosshair },
    },
    // Keep the candles above the volume histogram (bottom ~18% of the pane).
    rightPriceScale: { borderColor: palette.border, scaleMargins: { top: 0.08, bottom: 0.22 } },
    timeScale: {
      borderColor: palette.border,
      timeVisible: true,
      secondsVisible: false,
      rightOffset: 5,
    },
  };
}

export function PriceChart({ snapshot, theme }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const stateRef = useRef<ChartState | null>(null);

  // Create / destroy the chart with the container.
  // biome-ignore lint/correctness/useExhaustiveDependencies: the chart is created once; theme and data are applied by the effects below
  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    const palette = palettes[theme];
    const chart = createChart(container, chartOptions(palette));

    const candles = chart.addSeries(CandlestickSeries, {
      upColor: palette.up,
      downColor: palette.down,
      borderVisible: false,
      wickUpColor: palette.up,
      wickDownColor: palette.down,
    });
    const volume = chart.addSeries(HistogramSeries, {
      priceFormat: { type: "volume" },
      priceScaleId: "volume",
      priceLineVisible: false,
      lastValueVisible: false,
    });
    chart.priceScale("volume").applyOptions({ scaleMargins: { top: 0.82, bottom: 0 } });

    const tradeLines = new TradeLinesPrimitive();
    candles.attachPrimitive(tradeLines);
    const markers = createSeriesMarkers(candles, []);

    stateRef.current = {
      chart,
      candles,
      volume,
      markers,
      tradeLines,
      metrics: new Map(),
      paneByIndicator: new Map(),
      pair: "",
      candleCount: 0,
    };

    return () => {
      chart.remove();
      stateRef.current = null;
    };
  }, []);

  // Theme changes.
  useEffect(() => {
    const state = stateRef.current;
    if (!state) return;
    const palette = palettes[theme];
    state.chart.applyOptions(chartOptions(palette));
    state.candles.applyOptions({
      upColor: palette.up,
      downColor: palette.down,
      wickUpColor: palette.up,
      wickDownColor: palette.down,
    });
    state.volume.setData(snapshot.candles.map((c) => toVolume(c, palette)));
    state.markers.setMarkers(buildMarkers(snapshot.orders, snapshot.candles, palette));
    state.tradeLines.setLines(buildTradeLines(snapshot.orders, snapshot.candles, palette));
  }, [theme, snapshot.candles, snapshot.orders]);

  // Data changes: full reload when the pair changes, incremental otherwise.
  useEffect(() => {
    const state = stateRef.current;
    if (!state) return;
    const palette = palettes[theme];
    const incremental =
      state.pair === snapshot.pair &&
      snapshot.candles.length >= state.candleCount &&
      snapshot.candles.length - state.candleCount <= 5;

    if (!incremental) {
      state.candles.setData(snapshot.candles.map(toCandle));
      state.volume.setData(snapshot.candles.map((c) => toVolume(c, palette)));
    } else {
      for (const c of snapshot.candles.slice(state.candleCount)) {
        state.candles.update(toCandle(c));
        state.volume.update(toVolume(c, palette));
      }
    }

    // Indicators: create missing series, update existing ones.
    const seen = new Set<string>();
    let nextPane =
      Math.max(1, ...state.paneByIndicator.values()) + (state.paneByIndicator.size ? 1 : 0);
    if (!incremental) {
      nextPane = 1;
      for (const { series } of state.metrics.values()) state.chart.removeSeries(series);
      state.metrics.clear();
      state.paneByIndicator.clear();
    }

    for (const indicator of snapshot.indicators) {
      let paneIndex = 0;
      if (!indicator.overlay) {
        const existing = state.paneByIndicator.get(indicator.name);
        if (existing === undefined) {
          paneIndex = nextPane++;
          state.paneByIndicator.set(indicator.name, paneIndex);
        } else {
          paneIndex = existing;
        }
      }

      for (const [index, metric] of indicator.metrics.entries()) {
        const key = metricKey(indicator, metric, index);
        seen.add(key);
        const data = metricData(metric);
        let entry = state.metrics.get(key);
        if (!entry) {
          entry = { series: createMetricSeries(state.chart, metric, paneIndex, palette), count: 0 };
          state.metrics.set(key, entry);
          entry.series.setData(data);
          entry.count = data.length;
          continue;
        }
        if (incremental && data.length >= entry.count && data.length - entry.count <= 5) {
          for (const point of data.slice(entry.count)) entry.series.update(point);
        } else {
          entry.series.setData(data);
        }
        entry.count = data.length;
      }
    }

    for (const [key, entry] of state.metrics) {
      if (!seen.has(key)) {
        state.chart.removeSeries(entry.series);
        state.metrics.delete(key);
      }
    }

    // Give indicator panes a sensible share of the height.
    const panes = state.chart.panes();
    for (const [index, pane] of panes.entries()) pane.setStretchFactor(index === 0 ? 3 : 1);

    state.markers.setMarkers(buildMarkers(snapshot.orders, snapshot.candles, palette));
    state.tradeLines.setLines(buildTradeLines(snapshot.orders, snapshot.candles, palette));

    if (!incremental) {
      state.chart.timeScale().fitContent();
    }
    state.pair = snapshot.pair;
    state.candleCount = snapshot.candles.length;
  }, [snapshot, theme]);

  return (
    <section className="card chart-card">
      <div className="card-title">
        <span>
          {snapshot.pair} <small>price · volume · indicators</small>
        </span>
        <span className="legend">
          {snapshot.indicators.map((indicator) => (
            <span key={indicator.name} className="legend-item">
              {indicator.metrics.map((metric, index) => (
                <span key={metricKey(indicator, metric, index)}>
                  <i style={{ background: metric.color || "var(--accent)" }} />
                  {metric.name ? `${indicator.name} ${metric.name}` : indicator.name}
                </span>
              ))}
            </span>
          ))}
        </span>
      </div>
      <div ref={containerRef} className="chart price-chart" />
    </section>
  );
}
