/** Colors of the strategy curves, readable on both themes. */
export const STRATEGY_COLORS = [
  "#a78bfa",
  "#f59e0b",
  "#34d399",
  "#f472b6",
  "#60a5fa",
  "#fb923c",
  "#a3e635",
];

export function strategyColor(index: number): string {
  return STRATEGY_COLORS[index % STRATEGY_COLORS.length] ?? "#9ca3af";
}

/**
 * Background of a correlation cell: transparent at zero, teal towards +1 and
 * rose towards -1, the hues of gains and losses.
 */
export function correlationColor(value: number): string {
  if (!Number.isFinite(value)) return "transparent";
  const clamped = Math.max(-1, Math.min(1, value));
  const alpha = Math.abs(clamped) * 0.55;
  return clamped >= 0
    ? `rgba(45, 212, 191, ${alpha.toFixed(3)})`
    : `rgba(251, 113, 133, ${alpha.toFixed(3)})`;
}

/** The average correlation between distinct strategies, a measure of diversification. */
export function averageCorrelation(matrix: number[][]): number | null {
  let total = 0;
  let count = 0;
  for (let i = 0; i < matrix.length; i++) {
    const row = matrix[i] ?? [];
    for (let j = i + 1; j < row.length; j++) {
      total += row[j] ?? 0;
      count++;
    }
  }
  return count === 0 ? null : total / count;
}
