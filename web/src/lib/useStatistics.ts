import { useEffect, useRef, useState } from "react";
import { fetchStatistics } from "../api/client";
import type { Statistics } from "../api/types";

export interface StatisticsState {
  statistics: Statistics | null;
  loading: boolean;
  error: string | null;
}

/**
 * The report is recomputed when the trade count changes, debounced: a live run
 * would otherwise spend a few thousand resamples on every fill. Switching pairs
 * loads immediately, since there is nothing on screen to keep.
 */
const REFRESH_DELAY = 2000;

export function useStatistics(pair: string | null, trades: number): StatisticsState {
  const [state, setState] = useState<StatisticsState>({
    statistics: null,
    loading: false,
    error: null,
  });
  const loaded = useRef<string | null>(null);

  // `trades` is a trigger, not a value: a closed trade is what makes the report
  // stale, and the effect refetches instead of reading it.
  // biome-ignore lint/correctness/useExhaustiveDependencies: trades triggers the refresh
  useEffect(() => {
    if (!pair) {
      setState({ statistics: null, loading: false, error: null });
      return;
    }

    const controller = new AbortController();
    const delay = loaded.current === pair ? REFRESH_DELAY : 0;
    setState((current) => ({ ...current, loading: true }));

    const timer = window.setTimeout(() => {
      fetchStatistics(pair, controller.signal)
        .then((statistics) => {
          loaded.current = pair;
          setState({ statistics, loading: false, error: null });
        })
        .catch((e) => {
          if (controller.signal.aborted) return;
          setState({
            statistics: null,
            loading: false,
            error: e instanceof Error ? e.message : String(e),
          });
        });
    }, delay);

    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [pair, trades]);

  return state;
}
