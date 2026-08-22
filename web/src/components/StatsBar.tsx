import type { Snapshot } from "../api/types";
import { formatNumber, formatPercent, signClass } from "../lib/format";
import type { Stats } from "../lib/stats";

interface Props {
  stats: Stats;
  snapshot: Snapshot;
}

interface Tile {
  label: string;
  value: string;
  tone?: string;
  hint?: string;
}

export function StatsBar({ stats, snapshot }: Props) {
  const hasEquity = snapshot.equity_values.length > 0;
  const tiles: Tile[] = [
    ...(hasEquity
      ? [
          {
            label: "Return",
            value: formatPercent(stats.returnPct),
            tone: signClass(stats.returnPct),
            hint: `${formatNumber(stats.initialEquity)} → ${formatNumber(stats.finalEquity)} ${snapshot.quote}`,
          },
          {
            label: "Max drawdown",
            value: formatPercent(-stats.maxDrawdown),
            tone: stats.maxDrawdown > 0 ? "neg" : "",
          },
        ]
      : []),
    {
      label: "Buy & hold",
      value: formatPercent(stats.buyAndHold),
      tone: signClass(stats.buyAndHold),
      hint: `${snapshot.asset} over the period`,
    },
    {
      label: "Trades",
      value: String(stats.trades),
      hint: `${stats.wins} wins · ${stats.losses} losses`,
    },
    {
      label: "Win rate",
      value: stats.trades ? formatPercent(stats.winRate).replace("+", "") : "—",
      tone: stats.trades ? (stats.winRate >= 0.5 ? "pos" : "neg") : "",
    },
    {
      label: "Profit",
      value: `${formatNumber(stats.totalProfit)} ${snapshot.quote}`,
      tone: signClass(stats.totalProfit),
      hint: stats.trades ? `avg ${formatPercent(stats.avgReturn)} / trade` : undefined,
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
    <section className="stats">
      {tiles.map((tile) => (
        <div key={tile.label} className="stat">
          <span className="stat-label">{tile.label}</span>
          <span className={`stat-value ${tile.tone ?? ""}`}>{tile.value}</span>
          {tile.hint && <span className="stat-hint">{tile.hint}</span>}
        </div>
      ))}
    </section>
  );
}
