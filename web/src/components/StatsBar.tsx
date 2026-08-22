import type { Snapshot } from "../api/types";
import { formatNumber, formatPercent, signClass } from "../lib/format";
import type { Stats } from "../lib/stats";
import { palettes, type Theme } from "../lib/theme";
import { Sparkline } from "./Sparkline";

interface Props {
  stats: Stats;
  snapshot: Snapshot;
  theme: Theme;
}

interface Tile {
  label: string;
  value: string;
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
      }
    : {
        label: "Buy & hold",
        value: formatPercent(stats.buyAndHold),
        tone: signClass(stats.buyAndHold),
        hint: `${snapshot.asset} over ${snapshot.candles.length} candles`,
      };

  const tiles: Tile[] = [
    ...(hasEquity
      ? [
          {
            label: "Buy & hold",
            value: formatPercent(stats.buyAndHold),
            tone: signClass(stats.buyAndHold),
            hint: `${snapshot.asset} over the period`,
          },
          {
            label: "Max drawdown",
            value: formatPercent(-stats.maxDrawdown),
            tone: stats.maxDrawdown > 0 ? "neg" : "",
          },
        ]
      : []),
    {
      label: "Trades",
      value: String(stats.trades),
      hint: `${stats.wins} won · ${stats.losses} lost`,
    },
    {
      label: "Win rate",
      value: stats.trades ? formatPercent(stats.winRate).replace("+", "") : "—",
      tone: stats.trades ? (stats.winRate >= 0.5 ? "pos" : "neg") : "",
    },
    {
      label: "Net profit",
      value: formatNumber(stats.totalProfit),
      unit: snapshot.quote,
      tone: signClass(stats.totalProfit),
      hint: stats.trades ? `avg ${formatPercent(stats.avgReturn)} per trade` : undefined,
    },
    {
      label: "Profit factor",
      value: stats.trades
        ? Number.isFinite(stats.profitFactor)
          ? stats.profitFactor.toFixed(2)
          : "∞"
        : "—",
      tone: stats.trades ? (stats.profitFactor >= 1 ? "pos" : "neg") : "",
    },
  ];

  return (
    <section className="stats" aria-label="Performance">
      <div className="stat-hero">
        <div className="stat">
          <span className="stat-label">{hero.label}</span>
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
            <span className="stat-label">{tile.label}</span>
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
