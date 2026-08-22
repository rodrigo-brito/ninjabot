import { useEffect, useRef, useState } from "react";
import { createOrder, startBot, stopBot } from "../api/client";
import type { ControlsResponse } from "../api/types";
import { formatQuantity } from "../lib/format";

interface Props {
  pair: string;
  asset: string;
  quote: string;
  controls: ControlsResponse;
  onControlsChange: (controls: ControlsResponse) => void;
}

type Unit = "quote" | "percent";
type Side = "buy" | "sell";
type Confirm = { kind: "stop" } | { kind: "order"; side: Side };
type Feedback = { tone: "ok" | "error"; text: string };

export function ControlPanel({ pair, asset, quote, controls, onControlsChange }: Props) {
  const [amount, setAmount] = useState("");
  const [unit, setUnit] = useState<Unit>("quote");
  const [confirm, setConfirm] = useState<Confirm | null>(null);
  const [busy, setBusy] = useState(false);
  const [feedback, setFeedback] = useState<Feedback | null>(null);
  const feedbackTimer = useRef<number | undefined>(undefined);

  const running = controls.status === "running";
  const parsedAmount = Number(amount);
  const validAmount =
    Number.isFinite(parsedAmount) && parsedAmount > 0 && (unit === "quote" || parsedAmount <= 100);

  // Changing the pair invalidates a pending confirmation.
  // biome-ignore lint/correctness/useExhaustiveDependencies: reset on pair change only
  useEffect(() => setConfirm(null), [pair]);

  useEffect(() => {
    return () => window.clearTimeout(feedbackTimer.current);
  }, []);

  const showFeedback = (next: Feedback) => {
    setFeedback(next);
    window.clearTimeout(feedbackTimer.current);
    feedbackTimer.current = window.setTimeout(() => setFeedback(null), 8000);
  };

  const run = async (action: () => Promise<void>) => {
    setBusy(true);
    try {
      await action();
    } catch (e) {
      showFeedback({ tone: "error", text: e instanceof Error ? e.message : String(e) });
    } finally {
      setBusy(false);
      setConfirm(null);
    }
  };

  const handleStart = () =>
    run(async () => {
      onControlsChange(await startBot());
      showFeedback({ tone: "ok", text: "Bot started." });
    });

  const handleStop = () =>
    run(async () => {
      onControlsChange(await stopBot());
      showFeedback({
        tone: "ok",
        text: "Bot stopped. New orders (including protective stops) are rejected until start.",
      });
    });

  const handleOrder = (side: Side) =>
    run(async () => {
      const order = await createOrder({
        pair,
        side,
        amount: parsedAmount,
        percent: unit === "percent",
      });
      showFeedback({
        tone: "ok",
        text: `${side === "buy" ? "Buy" : "Sell"} order #${order.id} created — ${formatQuantity(
          order.quantity,
        )} ${asset}`,
      });
      setAmount("");
    });

  const amountLabel = unit === "percent" ? `${parsedAmount}%` : `${parsedAmount} ${quote}`;

  return (
    <section className="card control-panel" aria-label="Bot controls">
      <div className="card-title">
        Controls
        <span className={`status-chip ${running ? "running" : "stopped"}`}>
          <i />
          {controls.status || "unknown"}
        </span>
      </div>

      <div className="control-row">
        <div className="control-group">
          <span className="control-label">Bot</span>
          {running ? (
            confirm?.kind === "stop" ? (
              <>
                <button
                  type="button"
                  className="control-btn danger"
                  disabled={busy}
                  onClick={handleStop}
                >
                  Confirm stop
                </button>
                <button type="button" disabled={busy} onClick={() => setConfirm(null)}>
                  Cancel
                </button>
              </>
            ) : (
              <button
                type="button"
                className="control-btn danger-outline"
                disabled={busy}
                onClick={() => setConfirm({ kind: "stop" })}
              >
                Stop
              </button>
            )
          ) : (
            <button
              type="button"
              className="control-btn accent"
              disabled={busy}
              onClick={handleStart}
            >
              Start
            </button>
          )}
        </div>

        <div className="control-divider" aria-hidden="true" />

        <div className="control-group order-form">
          <span className="control-label">
            Market order · <strong>{pair}</strong>
          </span>
          <input
            type="number"
            inputMode="decimal"
            min="0"
            step="any"
            placeholder="amount"
            value={amount}
            disabled={busy}
            onChange={(e) => {
              setAmount(e.target.value);
              setConfirm(null);
            }}
            aria-label="Order amount"
          />
          <select
            value={unit}
            disabled={busy}
            onChange={(e) => {
              setUnit(e.target.value as Unit);
              setConfirm(null);
            }}
            aria-label="Amount unit"
          >
            <option value="quote">{quote}</option>
            <option value="percent">% of balance</option>
          </select>
          {confirm?.kind === "order" ? (
            <>
              <button
                type="button"
                className={`control-btn ${confirm.side === "buy" ? "buy" : "sell"}`}
                disabled={busy}
                onClick={() => handleOrder(confirm.side)}
              >
                Confirm {confirm.side} {amountLabel}
              </button>
              <button type="button" disabled={busy} onClick={() => setConfirm(null)}>
                Cancel
              </button>
            </>
          ) : (
            <>
              <button
                type="button"
                className="control-btn buy"
                disabled={busy || !validAmount}
                onClick={() => setConfirm({ kind: "order", side: "buy" })}
              >
                Buy
              </button>
              <button
                type="button"
                className="control-btn sell"
                disabled={busy || !validAmount}
                onClick={() => setConfirm({ kind: "order", side: "sell" })}
              >
                Sell
              </button>
            </>
          )}
        </div>
      </div>

      {feedback && <p className={`control-feedback ${feedback.tone}`}>{feedback.text}</p>}
    </section>
  );
}
