import type { ReactNode } from "react";
import type { Snapshot } from "../api/types";
import { formatNumber, formatPercent, signClass } from "../lib/format";
import type { Stats } from "../lib/stats";
import { palettes, type Theme } from "../lib/theme";
import { InfoTip } from "./InfoTip";
import { Sparkline } from "./Sparkline";

interface Props {
  stats: Stats;
  snapshot: Snapshot;
  theme: Theme;
}

interface Tile {
  label: string;
  value: string;
  /** Required: a metric nobody can explain has no place on the page. */
  help: ReactNode;
  unit?: string;
  tone?: string;
  hint?: string;
}

export function StatsBar({ stats, snapshot, theme }: Props) {
  const hasEquity = snapshot.equity_values.length > 0;
  const palette = palettes[theme];

  // The hero answers the one question a backtest is run for: did it pay off?
  // Without a paper wallet there is no equity, so buy & hold takes its place.
  const hero: Tile = hasEquity
    ? {
        label: "Strategy return",
        value: formatPercent(stats.returnPct),
        tone: signClass(stats.returnPct),
        hint: `${formatNumber(stats.initialEquity)} → ${formatNumber(stats.finalEquity)} ${snapshot.quote}`,
        help: "Return of the paper wallet over the whole period: final equity against the initial one, with the trading fees of every fill already charged.",
      }
    : {
        label: "Buy & hold",
        value: formatPercent(stats.buyAndHold),
        tone: signClass(stats.buyAndHold),
        hint: `${snapshot.asset} over ${snapshot.candles.length} candles`,
        help: "What the asset itself did over the same candles, from the first close to the last one. It is the bar the strategy has to clear to be worth its risk.",
      };

  const tiles: Tile[] = [
    ...(hasEquity
      ? [
          {
            label: "Buy & hold",
            value: formatPercent(stats.buyAndHold),
            tone: signClass(stats.buyAndHold),
            hint: `${snapshot.asset} over the period`,
            help: "What the asset itself did over the same candles, from the first close to the last one. A strategy that does not beat it is an expensive way to hold the asset.",
          },
          {
            label: "Max drawdown",
            value: formatPercent(-stats.maxDrawdown),
            tone: stats.maxDrawdown > 0 ? "neg" : "",
            help: "Worst decline of the equity curve reported by the paper wallet, which measures the deepest continuous drop in quote currency. The robustness panel below reports the deepest drop in percent instead, so the two can point at different weeks.",
          },
        ]
      : []),
    {
      label: "Trades",
      value: String(stats.trades),
      hint: `${stats.wins} won · ${stats.losses} lost`,
      help: "Closed round trips: orders that carry a result. A position still open is not counted, and neither is the entry that opened a closed one.",
    },
    {
      label: "Win rate",
      value: stats.trades ? formatPercent(stats.winRate).replace("+", "") : "—",
      tone: stats.trades ? (stats.winRate >= 0.5 ? "pos" : "neg") : "",
      help: "Share of the closed trades that ended in profit. On its own it says little: a strategy can win a third of the time and still make money if its winners are much larger than its losers.",
    },
    {
      label: "Net profit",
      value: formatNumber(stats.totalProfit),
      unit: snapshot.quote,
      tone: signClass(stats.totalProfit),
      hint: stats.trades ? `avg ${formatPercent(stats.avgReturn)} per trade` : undefined,
      help: "Sum of the profit of every closed trade, in quote currency and after fees, with the average return per trade below it.",
    },
    {
      label: "Profit factor",
      value: stats.trades
        ? Number.isFinite(stats.profitFactor)
          ? stats.profitFactor.toFixed(2)
          : "∞"
        : "—",
      tone: stats.trades ? (stats.profitFactor >= 1 ? "pos" : "neg") : "",
      help: "Gross profit divided by gross loss. Below 1 the strategy loses money; the robustness panel below puts a confidence interval around it, since a couple of large trades can carry this number on their own.",
    },
  ];

  return (
    <section className="stats" aria-label="Performance">
      <div className="stat-hero">
        <div className="stat">
          <span className="stat-label">
            {hero.label}
            <InfoTip label={hero.label}>{hero.help}</InfoTip>
          </span>
          <span className={`stat-value ${hero.tone ?? ""}`}>{hero.value}</span>
        </div>
        {hero.hint && <span className="stat-hint">{hero.hint}</span>}
        {hasEquity && (
          <Sparkline
            className="sparkline"
            points={snapshot.equity_values}
            color={stats.returnPct < 0 ? palette.down : palette.accent}
          />
        )}
      </div>
      <div className="stat-strip">
        {tiles.map((tile) => (
          <div key={tile.label} className="stat">
            <span className="stat-label">
              {tile.label}
              <InfoTip label={tile.label}>{tile.help}</InfoTip>
            </span>
            <span className={`stat-value ${tile.tone ?? ""}`}>
              {tile.value}
              {tile.unit && <span className="unit">{tile.unit}</span>}
            </span>
            {tile.hint && <span className="stat-hint">{tile.hint}</span>}
          </div>
        ))}
      </div>
    </section>
  );
}
