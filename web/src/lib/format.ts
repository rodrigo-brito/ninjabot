const numberFormat = new Intl.NumberFormat("en-US", { maximumFractionDigits: 2 });
const priceFormat = new Intl.NumberFormat("en-US", { maximumFractionDigits: 8 });
const compactFormat = new Intl.NumberFormat("en-US", { maximumSignificantDigits: 4 });

export function formatNumber(value: number): string {
  return numberFormat.format(value);
}

export function formatPrice(value: number): string {
  if (Math.abs(value) >= 1000) return numberFormat.format(value);
  return priceFormat.format(value);
}

export function formatPercent(fraction: number, digits = 2): string {
  if (!Number.isFinite(fraction)) return "∞";
  const sign = fraction > 0 ? "+" : "";
  return `${sign}${(fraction * 100).toFixed(digits)}%`;
}

export function formatQuantity(value: number): string {
  return priceFormat.format(value);
}

/** Short form for chart labels, e.g. 0.4485 or 1,234. */
export function formatCompact(value: number): string {
  return compactFormat.format(value);
}

export function formatTime(unixSeconds: number): string {
  const date = new Date(unixSeconds * 1000);
  return date.toISOString().replace("T", " ").slice(0, 16);
}

export function signClass(value: number): string {
  if (value > 0) return "pos";
  if (value < 0) return "neg";
  return "";
}
