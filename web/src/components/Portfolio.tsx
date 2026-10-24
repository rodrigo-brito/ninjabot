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
import type { Point, PortfolioView } from "../api/types";
import { formatNumber, formatPercent, signClass } from "../lib/format";
import { averageCorrelation, correlationColor, strategyColor } from "../lib/portfolio";
import { palettes, type Theme } from "../lib/theme";

interface Props {
  portfolio: PortfolioView;
  theme: Theme;
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

function PortfolioChart({ portfolio, theme }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const seriesRef = useRef<ISeriesApi<"Line">[]>([]);

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
      timeScale: { borderColor: palette.border, timeVisible: true, secondsVisible: false },
    });
    chartRef.current = chart;
    return () => {
      chart.remove();
      chartRef.current = null;
      seriesRef.current = [];
    };
  }, []);

  useEffect(() => {
    const chart = chartRef.current;
    if (!chart) return;
    const palette = palettes[theme];
    chart.applyOptions({
      layout: {
        background: { type: ColorType.Solid, color: palette.background },
        textColor: palette.text,
      },
      grid: { vertLines: { color: palette.grid }, horzLines: { color: palette.grid } },
      rightPriceScale: { borderColor: palette.border },
      timeScale: { borderColor: palette.border },
    });

    for (const series of seriesRef.current) chart.removeSeries(series);
    const combined = chart.addSeries(LineSeries, {
      color: palette.equity,
      lineWidth: 2,
      title: "Portfolio",
      priceLineVisible: false,
    });
    combined.setData(toLine(portfolio.equity));
    const strategies = portfolio.strategies.map((strategy, index) => {
      const series = chart.addSeries(LineSeries, {
        color: strategyColor(index),
        lineWidth: 1,
        title: strategy.id,
        priceLineVisible: false,
      });
      series.setData(toLine(strategy.equity));
      return series;
    });
    seriesRef.current = [combined, ...strategies];
    chart.timeScale().fitContent();
  }, [portfolio, theme]);

  return <div ref={containerRef} className="chart equity-chart" />;
}

export function PortfolioPanel({ portfolio, theme }: Props) {
  const diversification = averageCorrelation(portfolio.correlation);
  const ids = portfolio.strategies.map((s) => s.id);

  return (
    <>
      <section className="stats" aria-label="Portfolio">
        <div className="stat">
          <span className="stat-label">Portfolio</span>
          <span className="stat-value">{portfolio.name || "Portfolio"}</span>
        </div>
        <div className="stat">
          <span className="stat-label">Return</span>
          <span className={`stat-value ${signClass(portfolio.return_pct)}`}>
            {formatPercent(portfolio.return_pct)}
          </span>
        </div>
        <div className="stat">
          <span className="stat-label">Net profit ({portfolio.quote})</span>
          <span className={`stat-value ${signClass(portfolio.net_profit)}`}>
            {formatNumber(portfolio.net_profit)}
          </span>
        </div>
        <div className="stat">
          <span className="stat-label">Max drawdown</span>
          <span className="stat-value">{formatPercent(-portfolio.max_drawdown)}</span>
        </div>
        <div className="stat">
          <span className="stat-label">Sharpe (annual)</span>
          <span className="stat-value">{portfolio.sharpe_annual.toFixed(2)}</span>
        </div>
        <div className="stat">
          <span className="stat-label">Avg. correlation</span>
          <span className="stat-value">
            {diversification === null ? "—" : diversification.toFixed(2)}
          </span>
        </div>
      </section>

      <section className="card chart-card">
        <div className="card-title">
          <span>
            Equity <small>{portfolio.quote}</small>
          </span>
          <span className="legend">
            <span className="legend-item">
              <span>
                <i style={{ background: palettes[theme].equity }} />
                Portfolio
              </span>
              {portfolio.strategies.map((strategy, index) => (
                <span key={strategy.id}>
                  <i style={{ background: strategyColor(index) }} />
                  {strategy.id}
                </span>
              ))}
            </span>
          </span>
        </div>
        <PortfolioChart portfolio={portfolio} theme={theme} />
      </section>

      <section className="card">
        <div className="card-title">
          <span>Strategies</span>
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Strategy</th>
                <th className="num">Weight</th>
                <th className="num">Trades</th>
                <th className="num">Return</th>
                <th className="num">Max DD</th>
                <th className="num">Sharpe</th>
                <th className="num">PnL share</th>
                <th className="num">Marginal Sharpe</th>
              </tr>
            </thead>
            <tbody>
              {portfolio.strategies.map((strategy, index) => (
                <tr key={strategy.id}>
                  <td>
                    <span className="swatch" style={{ background: strategyColor(index) }} />
                    {strategy.id} <small className="muted">{strategy.strategy}</small>
                  </td>
                  <td className="num mono">{formatPercent(strategy.weight, 0).replace("+", "")}</td>
                  <td className="num mono">{strategy.trades}</td>
                  <td className={`num mono ${signClass(strategy.return_pct)}`}>
                    {formatPercent(strategy.return_pct)}
                  </td>
                  <td className="num mono">{formatPercent(-strategy.max_drawdown)}</td>
                  <td className="num mono">{strategy.sharpe_annual.toFixed(2)}</td>
                  <td className="num mono">{formatPercent(strategy.pnl_share, 1)}</td>
                  <td className={`num mono ${signClass(strategy.marginal_sharpe)}`}>
                    {strategy.marginal_sharpe >= 0 ? "+" : ""}
                    {strategy.marginal_sharpe.toFixed(3)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {portfolio.unallocated > 0 && (
          <p className="empty">
            Unallocated cash: {formatNumber(portfolio.unallocated)} {portfolio.quote}
          </p>
        )}
      </section>

      {ids.length > 1 && (
        <section className="card">
          <div className="card-title">
            <span>Correlation of returns</span>
          </div>
          <div className="table-wrap">
            <table className="heatmap">
              <thead>
                <tr>
                  <th />
                  {ids.map((id) => (
                    <th key={id} className="num">
                      {id}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {ids.map((id, i) => (
                  <tr key={id}>
                    <th>{id}</th>
                    {ids.map((other, j) => {
                      const value = portfolio.correlation[i]?.[j] ?? 0;
                      return (
                        <td
                          key={other}
                          className="num mono"
                          style={{ background: correlationColor(value) }}
                        >
                          {value.toFixed(2)}
                        </td>
                      );
                    })}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      )}
    </>
  );
}
