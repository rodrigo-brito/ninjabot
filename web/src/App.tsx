import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  fetchControls,
  fetchPairs,
  fetchSnapshot,
  subscribeEvents,
  UI_VERSION,
} from "./api/client";
import type { ControlsResponse, Snapshot } from "./api/types";
import { ControlPanel } from "./components/ControlPanel";
import { EquityChart } from "./components/EquityChart";
import { Header } from "./components/Header";
import { OrdersTable } from "./components/OrdersTable";
import { PriceChart } from "./components/PriceChart";
import { StatsBar } from "./components/StatsBar";
import { applyCandleEvent, applyOrderEvent, computeStats } from "./lib/stats";
import { loadTheme, saveTheme, type Theme } from "./lib/theme";

function pairFromURL(): string | null {
  return new URLSearchParams(window.location.search).get("pair");
}

function setPairInURL(pair: string) {
  const url = new URL(window.location.href);
  url.searchParams.set("pair", pair);
  window.history.replaceState(null, "", url);
}

export function App() {
  const [theme, setTheme] = useState<Theme>(loadTheme);
  const [pairs, setPairs] = useState<string[]>([]);
  const [pair, setPair] = useState<string | null>(pairFromURL);
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [connected, setConnected] = useState(false);
  const [controls, setControls] = useState<ControlsResponse | null>(null);
  const pairRef = useRef<string | null>(pair);
  pairRef.current = pair;

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    saveTheme(theme);
  }, [theme]);

  // Load the pair list; keep polling while the bot has no data yet.
  useEffect(() => {
    const controller = new AbortController();
    let timer: number | undefined;
    const load = async () => {
      try {
        const list = await fetchPairs(controller.signal);
        setPairs(list);
        if (list.length === 0) {
          timer = window.setTimeout(load, 3000);
          return;
        }
        setPair((current) => (current && list.includes(current) ? current : (list[0] ?? null)));
      } catch (e) {
        if (controller.signal.aborted) return;
        setError(e instanceof Error ? e.message : String(e));
        timer = window.setTimeout(load, 5000);
      }
    };
    load();
    return () => {
      controller.abort();
      if (timer) window.clearTimeout(timer);
    };
  }, []);

  // Load the snapshot of the selected pair.
  useEffect(() => {
    if (!pair) return;
    setPairInURL(pair);
    const controller = new AbortController();
    setLoading(true);
    fetchSnapshot(pair, controller.signal)
      .then((s) => {
        setSnapshot(s);
        setError(null);
      })
      .catch((e) => {
        if (!controller.signal.aborted) setError(e instanceof Error ? e.message : String(e));
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [pair]);

  // Bot control state: SSE pushes changes made from this dashboard, polling
  // catches changes made elsewhere (Telegram, another browser).
  useEffect(() => {
    const controller = new AbortController();
    const load = () => {
      fetchControls(controller.signal)
        .then(setControls)
        .catch(() => {
          // leave the last known state; polling retries
        });
    };
    load();
    const timer = window.setInterval(load, 10000);
    return () => {
      controller.abort();
      window.clearInterval(timer);
    };
  }, []);

  // Live updates.
  useEffect(() => {
    return subscribeEvents({
      onStatus: setConnected,
      onCandle: (event) => {
        setPairs((list) => (list.includes(event.pair) ? list : [...list, event.pair].sort()));
        setSnapshot((s) => (s ? applyCandleEvent(s, event) : s));
      },
      onOrder: (event) => {
        setSnapshot((s) => (s ? applyOrderEvent(s, event) : s));
      },
      onControls: setControls,
    });
  }, []);

  const stats = useMemo(() => (snapshot ? computeStats(snapshot) : null), [snapshot]);
  const toggleTheme = useCallback(() => setTheme((t) => (t === "dark" ? "light" : "dark")), []);

  return (
    <div className="app">
      <Header
        pairs={pairs}
        pair={pair}
        onPairChange={setPair}
        theme={theme}
        onThemeToggle={toggleTheme}
        connected={connected}
        version={UI_VERSION}
      />
      <main>
        {error && <div className="banner error">{error}</div>}
        {!snapshot && !error && (
          <div className="banner">
            {pairs.length === 0 ? "Waiting for the first candle…" : "Loading…"}
          </div>
        )}
        {snapshot && stats && (
          <div className={loading ? "content loading" : "content"}>
            <StatsBar stats={stats} snapshot={snapshot} theme={theme} />
            {controls?.enabled && (
              <ControlPanel
                pair={snapshot.pair}
                asset={snapshot.asset}
                quote={snapshot.quote}
                controls={controls}
                onControlsChange={setControls}
              />
            )}
            <PriceChart snapshot={snapshot} theme={theme} />
            {snapshot.equity_values.length > 0 && <EquityChart snapshot={snapshot} theme={theme} />}
            <OrdersTable pair={snapshot.pair} quote={snapshot.quote} orders={snapshot.orders} />
          </div>
        )}
      </main>
    </div>
  );
}
