import type { CandleEvent, OrderEvent, PairsResponse, Snapshot } from "./types";

declare global {
  interface Window {
    __NINJABOT__?: { version?: string; api?: string };
  }
}

export const API_BASE = window.__NINJABOT__?.api ?? "/api";
export const UI_VERSION = window.__NINJABOT__?.version ?? "dev";

async function getJSON<T>(path: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, { signal });
  if (!response.ok) {
    throw new Error(`${path}: ${response.status} ${response.statusText}`);
  }
  return (await response.json()) as T;
}

export function fetchPairs(signal?: AbortSignal): Promise<string[]> {
  return getJSON<PairsResponse>("/pairs", signal).then((r) => r.pairs);
}

export function fetchSnapshot(pair: string, signal?: AbortSignal): Promise<Snapshot> {
  return getJSON<Snapshot>(`/${encodeURIComponent(pair)}/snapshot`, signal);
}

export function ordersCSVUrl(pair: string): string {
  return `${API_BASE}/${encodeURIComponent(pair)}/orders.csv`;
}

export interface EventHandlers {
  onCandle?: (event: CandleEvent) => void;
  onOrder?: (event: OrderEvent) => void;
  onStatus?: (connected: boolean) => void;
}

/**
 * Subscribes to the Server-Sent Events stream. EventSource reconnects by
 * itself; the returned function closes the connection.
 */
export function subscribeEvents(handlers: EventHandlers): () => void {
  const source = new EventSource(`${API_BASE}/events`);
  source.onopen = () => handlers.onStatus?.(true);
  source.onerror = () => handlers.onStatus?.(false);
  source.addEventListener("candle", (e) => {
    handlers.onCandle?.(JSON.parse((e as MessageEvent<string>).data) as CandleEvent);
  });
  source.addEventListener("order", (e) => {
    handlers.onOrder?.(JSON.parse((e as MessageEvent<string>).data) as OrderEvent);
  });
  return () => source.close();
}
