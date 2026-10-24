import type { ReactNode } from "react";
import type { HistogramBin, Interval, Percentiles, TradeWindow } from "../api/types";
import { formatPercent } from "../lib/format";
import {
  crossesThreshold,
  formatIntervalValue,
  formatPercentile,
  formatProbability,
} from "../lib/statistics";
import { InfoTip } from "./InfoTip";

/**
 * The charts of the robustness panel. They are plain elements positioned in
 * percent instead of SVG: every one of them is a value on a line, and a line
 * that reflows with the card beats one that scales with a viewBox.
 */

/** Maps a value of the domain to a 0..100 position. */
function position(value: number, min: number, max: number): number {
  const span = max - min || 1;
  return Math.min(100, Math.max(0, ((value - min) / span) * 100));
}

interface ConfidenceBarProps {
  interval: Interval;
  name: string;
  thresholdLabel: string;
  help: ReactNode;
}

/**
 * One bootstrap confidence interval. The reading that matters is whether the
 * bar still covers the threshold -- an interval around a mean return that
 * includes zero cannot rule out a strategy with no edge at all.
 */
export function ConfidenceBar({ interval, name, thresholdLabel, help }: ConfidenceBarProps) {
  const crosses = crossesThreshold(interval);
  const min = Math.min(interval.lower, interval.threshold);
  const max = Math.max(interval.upper, interval.threshold);
  const pad = (max - min || 1) * 0.12;
  const from = min - pad;
  const to = max + pad;

  const left = position(interval.lower, from, to);
  const right = position(interval.upper, from, to);

  return (
    <div className="interval">
      <div className="interval-head">
        <span className="interval-name">
          {name}
          <InfoTip label={name}>{help}</InfoTip>
        </span>
        <span className={`interval-value ${crosses ? "warn" : "pos"}`}>
          {formatIntervalValue(interval, interval.mean)}
        </span>
      </div>
      <div
        className="interval-track"
        role="img"
        aria-label={`${name}: 95% confidence interval from ${formatIntervalValue(
          interval,
          interval.lower,
        )} to ${formatIntervalValue(interval, interval.upper)}`}
      >
        <span
          className={`interval-range ${crosses ? "warn" : "pos"}`}
          style={{ left: `${left}%`, width: `${Math.max(right - left, 1)}%` }}
        />
        <span
          className="interval-mean"
          style={{ left: `${position(interval.mean, from, to)}%` }}
          aria-hidden="true"
        />
        <span
          className="interval-threshold"
          style={{ left: `${position(interval.threshold, from, to)}%` }}
          aria-hidden="true"
        />
      </div>
      <div className="interval-foot">
        <span>{formatIntervalValue(interval, interval.lower)}</span>
        <span className="interval-note">
          {crosses ? `still contains ${thresholdLabel}` : `clears ${thresholdLabel}`}
        </span>
        <span>{formatIntervalValue(interval, interval.upper)}</span>
      </div>
    </div>
  );
}

interface HistogramProps {
  bins: HistogramBin[];
}

/** Distribution of the trade returns, the chart the terminal draws in ASCII. */
export function ReturnHistogram({ bins }: HistogramProps) {
  const peak = bins.reduce((max, bin) => Math.max(max, bin.count), 0);
  if (peak === 0) return <p className="empty">No closed trades yet.</p>;

  const first = bins[0];
  const last = bins[bins.length - 1];

  return (
    <div className="histogram">
      <div className="histogram-bars" role="img" aria-label="Distribution of the trade returns">
        {bins.map((bin) => (
          <span
            key={bin.start}
            className={`histogram-bar ${bin.end <= 0 ? "neg" : "pos"}`}
            style={{ height: `${(bin.count / peak) * 100}%` }}
            title={`${formatPercent(bin.start)} to ${formatPercent(bin.end)}: ${bin.count} trades`}
          />
        ))}
      </div>
      <div className="histogram-axis">
        <span>{first ? formatPercent(first.start) : ""}</span>
        <span className="histogram-peak">{peak} trades in the tallest bin</span>
        <span>{last ? formatPercent(last.end) : ""}</span>
      </div>
    </div>
  );
}

interface PercentileRangeProps {
  name: string;
  percentiles: Percentiles;
  /** Value the backtest actually produced, drawn against the simulations. */
  observed?: number;
  tone?: "pos" | "neg";
  /** A drawdown is a magnitude: a leading "+" on it only confuses. */
  signed?: boolean;
  help: ReactNode;
}

/**
 * The spread of a simulated measure: the light band is the 5th to 95th
 * percentile, the solid one the interquartile range, the tick the median. The
 * diamond is what the backtest produced -- when it sits at the top of the
 * band, the reported result was the lucky end of the distribution.
 */
export function PercentileRange({
  name,
  percentiles,
  observed,
  tone,
  signed = true,
  help,
}: PercentileRangeProps) {
  const format = signed ? formatPercentile : (value: number) => formatProbability(value, 1);
  const values = [percentiles.p5, percentiles.p95, ...(observed === undefined ? [] : [observed])];
  const min = Math.min(...values);
  const max = Math.max(...values);
  const pad = (max - min || 1) * 0.1;
  const from = min - pad;
  const to = max + pad;

  const p5 = position(percentiles.p5, from, to);
  const p95 = position(percentiles.p95, from, to);
  const p25 = position(percentiles.p25, from, to);
  const p75 = position(percentiles.p75, from, to);

  return (
    <div className="percentile">
      <div className="percentile-head">
        <span className="interval-name">
          {name}
          <InfoTip label={name}>{help}</InfoTip>
        </span>
        <span className={`interval-value ${tone ?? ""}`}>{format(percentiles.median)}</span>
      </div>
      <div
        className="percentile-track"
        role="img"
        aria-label={`${name}: 5th percentile ${formatPercentile(
          percentiles.p5,
        )}, median ${format(percentiles.median)}, 95th percentile ${formatPercentile(
          percentiles.p95,
        )}`}
      >
        <span
          className="percentile-outer"
          style={{ left: `${p5}%`, width: `${Math.max(p95 - p5, 1)}%` }}
        />
        <span
          className="percentile-inner"
          style={{ left: `${p25}%`, width: `${Math.max(p75 - p25, 1)}%` }}
        />
        <span
          className="percentile-median"
          style={{ left: `${position(percentiles.median, from, to)}%` }}
          aria-hidden="true"
        />
        {observed !== undefined && (
          <span
            className="percentile-observed"
            style={{ left: `${position(observed, from, to)}%` }}
            title={`backtest: ${format(observed)}`}
          />
        )}
      </div>
      <div className="interval-foot">
        <span>p5 {format(percentiles.p5)}</span>
        {observed !== undefined && (
          <span className="interval-note">backtest {format(observed)}</span>
        )}
        <span>p95 {format(percentiles.p95)}</span>
      </div>
    </div>
  );
}

interface WindowBarsProps {
  windows: TradeWindow[];
}

/**
 * Average return of each slice of the history. A strategy that only makes
 * money in one window was fitted to it, however good the total looks.
 */
export function WindowBars({ windows }: WindowBarsProps) {
  const peak = windows.reduce((max, window) => Math.max(max, Math.abs(window.mean_return)), 0);
  if (peak === 0) return null;

  return (
    <div className="windows">
      {windows.map((window) => {
        const height = (Math.abs(window.mean_return) / peak) * 100;
        const negative = window.mean_return < 0;
        return (
          <div className="window" key={window.index}>
            <div className="window-plot">
              <span
                className={`window-bar ${negative ? "neg" : "pos"}`}
                style={{
                  height: `${Math.max(height / 2, 1)}%`,
                  [negative ? "top" : "bottom"]: "50%",
                }}
                title={`${window.trades} trades, ${formatPercent(window.mean_return)} per trade`}
              />
              <span className="window-zero" aria-hidden="true" />
            </div>
            <span className={`window-value ${negative ? "neg" : "pos"}`}>
              {formatPercent(window.mean_return)}
            </span>
            <span className="window-label">
              {window.trades} trades · {Math.round(window.win_rate * 100)}% won
            </span>
          </div>
        );
      })}
    </div>
  );
}
