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
}

export const palettes: Record<Theme, Palette> = {
  dark: {
    background: "#0f1115",
    text: "#d7dae0",
    muted: "#8b919c",
    grid: "#1c2029",
    border: "#262b36",
    up: "#26a69a",
    down: "#ef5350",
    volumeUp: "rgba(38, 166, 154, 0.35)",
    volumeDown: "rgba(239, 83, 80, 0.35)",
    accent: "#7aa2f7",
    equity: "#7aa2f7",
    asset: "#e0af68",
    crosshair: "#5c6370",
  },
  light: {
    background: "#ffffff",
    text: "#1f2328",
    muted: "#6b7280",
    grid: "#eef0f3",
    border: "#d9dde3",
    up: "#089981",
    down: "#f23645",
    volumeUp: "rgba(8, 153, 129, 0.3)",
    volumeDown: "rgba(242, 54, 69, 0.3)",
    accent: "#2962ff",
    equity: "#2962ff",
    asset: "#d97706",
    crosshair: "#9aa0a6",
  },
};
