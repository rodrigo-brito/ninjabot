import {
  ColorType,
  CrosshairMode,
  createChart,
  type IChartApi,
  type ISeriesApi,
  type LineData,
  LineSeries,
  type Time,
  type UTCTimestamp,
} from "lightweight-charts";
import { useEffect, useRef } from "react";
import type { Point, Snapshot } from "../api/types";
import { palettes, type Theme } from "../lib/theme";

interface Props {
  snapshot: Snapshot;
  theme: Theme;
}

interface ChartState {
  chart: IChartApi;
  equity: ISeriesApi<"Line">;
  asset: ISeriesApi<"Line">;
}

function toLine(points: Point[]): LineData<Time>[] {
  const out: LineData<Time>[] = [];
  let last = Number.NEGATIVE_INFINITY;
  for (const p of points) {
    if (p.time <= last) continue;
    last = p.time;
    out.push({ time: p.time as UTCTimestamp, value: p.value });
  }
  return out;
}

export function EquityChart({ snapshot, theme }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const stateRef = useRef<ChartState | null>(null);

  // biome-ignore lint/correctness/useExhaustiveDependencies: the chart is created once; theme and data are applied by the effects below
  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    const palette = palettes[theme];
    const chart = createChart(container, {
      autoSize: true,
      layout: {
        background: { type: ColorType.Solid, color: palette.background },
        textColor: palette.text,
        fontFamily: palette.fontFamily,
        fontSize: 11,
        attributionLogo: false,
      },
      grid: { vertLines: { color: palette.grid }, horzLines: { color: palette.grid } },
      crosshair: { mode: CrosshairMode.Normal },
      rightPriceScale: { borderColor: palette.border },
      leftPriceScale: { visible: true, borderColor: palette.border },
      timeScale: { borderColor: palette.border, timeVisible: true, secondsVisible: false },
    });
    const equity = chart.addSeries(LineSeries, {
      color: palette.equity,
      lineWidth: 2,
      title: "Equity",
      priceScaleId: "right",
      priceLineVisible: false,
    });
    const asset = chart.addSeries(LineSeries, {
      color: palette.asset,
      lineWidth: 1,
      title: "Asset value",
      priceScaleId: "left",
      priceLineVisible: false,
    });
    stateRef.current = { chart, equity, asset };
    return () => {
      chart.remove();
      stateRef.current = null;
    };
  }, []);

  useEffect(() => {
    const state = stateRef.current;
    if (!state) return;
    const palette = palettes[theme];
    state.chart.applyOptions({
      layout: {
        background: { type: ColorType.Solid, color: palette.background },
        textColor: palette.text,
      },
      grid: { vertLines: { color: palette.grid }, horzLines: { color: palette.grid } },
      rightPriceScale: { borderColor: palette.border },
      leftPriceScale: { borderColor: palette.border },
      timeScale: { borderColor: palette.border },
    });
    state.equity.applyOptions({ color: palette.equity });
    state.asset.applyOptions({ color: palette.asset });
  }, [theme]);

  useEffect(() => {
    const state = stateRef.current;
    if (!state) return;
    state.equity.setData(toLine(snapshot.equity_values));
    state.asset.setData(toLine(snapshot.asset_values));
    state.chart.timeScale().fitContent();
  }, [snapshot.equity_values, snapshot.asset_values]);

  return (
    <section className="card chart-card">
      <div className="card-title">
        <span>
          Equity <small>{snapshot.quote}</small>
        </span>
        <span className="legend">
          <span className="legend-item">
            <span>
              <i style={{ background: palettes[theme].equity }} />
              Equity ({snapshot.quote})
            </span>
            <span>
              <i style={{ background: palettes[theme].asset }} />
              {snapshot.asset} value
            </span>
          </span>
        </span>
      </div>
      <div ref={containerRef} className="chart equity-chart" />
    </section>
  );
}
