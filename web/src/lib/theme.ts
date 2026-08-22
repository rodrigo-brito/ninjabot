export type Theme = "dark" | "light";

const STORAGE_KEY = "ninjabot.theme";

export function loadTheme(): Theme {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored === "dark" || stored === "light") return stored;
  } catch {
    // storage unavailable
  }
  return window.matchMedia?.("(prefers-color-scheme: light)").matches ? "light" : "dark";
}

export function saveTheme(theme: Theme): void {
  try {
    localStorage.setItem(STORAGE_KEY, theme);
  } catch {
    // storage unavailable
  }
}

/** Colors handed to lightweight-charts; must mirror the CSS tokens in styles.css. */
export interface Palette {
  background: string;
  text: string;
  muted: string;
  grid: string;
  border: string;
  up: string;
  down: string;
  volumeUp: string;
  volumeDown: string;
  accent: string;
  equity: string;
  asset: string;
  crosshair: string;
  fontFamily: string;
}

const fontFamily =
  '"JetBrains Mono Variable", "JetBrains Mono", ui-monospace, SFMono-Regular, Menlo, monospace';

export const palettes: Record<Theme, Palette> = {
  dark: {
    background: "#0f131b",
    text: "#aeb6c4",
    muted: "#7c8596",
    grid: "rgba(148, 163, 184, 0.06)",
    border: "rgba(148, 163, 184, 0.14)",
    up: "#2dd4bf",
    down: "#fb7185",
    volumeUp: "rgba(45, 212, 191, 0.28)",
    volumeDown: "rgba(251, 113, 133, 0.28)",
    accent: "#22d3ee",
    equity: "#22d3ee",
    asset: "#a78bfa",
    crosshair: "rgba(34, 211, 238, 0.55)",
    fontFamily,
  },
  light: {
    background: "#ffffff",
    text: "#3b4250",
    muted: "#5b6473",
    grid: "rgba(15, 23, 42, 0.05)",
    border: "rgba(15, 23, 42, 0.12)",
    up: "#0d9488",
    down: "#e11d48",
    volumeUp: "rgba(13, 148, 136, 0.25)",
    volumeDown: "rgba(225, 29, 72, 0.25)",
    accent: "#0891b2",
    equity: "#0891b2",
    asset: "#7c3aed",
    crosshair: "rgba(8, 145, 178, 0.55)",
    fontFamily,
  },
};

/**
 * Strategies pass CSS color names ("red", "blue"). Pure named colors clash
 * with the palette, so the most common ones are mapped to softer tones with
 * the same hue. Anything else (hex, rgb…) is used verbatim.
 */
const namedColors: Record<string, string> = {
  red: "#f87171",
  blue: "#60a5fa",
  green: "#4ade80",
  purple: "#c084fc",
  orange: "#fb923c",
  yellow: "#facc15",
  pink: "#f472b6",
  cyan: "#22d3ee",
  teal: "#2dd4bf",
  gray: "#9ca3af",
  grey: "#9ca3af",
  white: "#e5e7eb",
  black: "#111827",
  brown: "#a16207",
  magenta: "#e879f9",
  lime: "#a3e635",
  navy: "#3b82f6",
  gold: "#eab308",
};

export function seriesColor(color: string, fallback: string): string {
  if (!color) return fallback;
  return namedColors[color.trim().toLowerCase()] ?? color;
}
