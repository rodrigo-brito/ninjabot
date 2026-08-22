import type { Theme } from "../lib/theme";

interface Props {
  pairs: string[];
  pair: string | null;
  onPairChange: (pair: string) => void;
  theme: Theme;
  onThemeToggle: () => void;
  connected: boolean;
  version: string;
}

export function Header({
  pairs,
  pair,
  onPairChange,
  theme,
  onThemeToggle,
  connected,
  version,
}: Props) {
  return (
    <header className="header">
      <div className="brand">
        <span className="logo" aria-hidden="true">
          🥷
        </span>
        <span className="title">NinjaBot</span>
        <span className="version">{version}</span>
      </div>
      <div className="controls">
        <label className="pair-select">
          <span>Pair</span>
          <select
            value={pair ?? ""}
            onChange={(e) => onPairChange(e.target.value)}
            disabled={pairs.length === 0}
          >
            {pairs.length === 0 && <option value="">waiting for data…</option>}
            {pairs.map((p) => (
              <option key={p} value={p}>
                {p}
              </option>
            ))}
          </select>
        </label>
        <span
          className={`status ${connected ? "on" : "off"}`}
          title={connected ? "Live updates connected" : "Reconnecting…"}
        >
          <i />
          {connected ? "live" : "offline"}
        </span>
        <button type="button" className="theme-toggle" onClick={onThemeToggle} title="Toggle theme">
          {theme === "dark" ? "☀︎" : "☾"}
        </button>
      </div>
    </header>
  );
}
