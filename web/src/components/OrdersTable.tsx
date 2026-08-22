import { useMemo, useState } from "react";
import { ordersCSVUrl } from "../api/client";
import type { Order } from "../api/types";
import {
  formatNumber,
  formatPercent,
  formatPrice,
  formatQuantity,
  formatTime,
  signClass,
} from "../lib/format";

interface Props {
  pair: string;
  quote: string;
  orders: Order[];
}

const PAGE = 50;

export function OrdersTable({ pair, quote, orders }: Props) {
  const [limit, setLimit] = useState(PAGE);
  const sorted = useMemo(
    () => [...orders].sort((a, b) => b.updated_at - a.updated_at || b.id - a.id),
    [orders],
  );
  const visible = sorted.slice(0, limit);

  return (
    <section className="card">
      <div className="card-title">
        <span>
          Orders <small>{orders.length}</small>
        </span>
        <a className="button" href={ordersCSVUrl(pair)} download>
          Download CSV
        </a>
      </div>
      {orders.length === 0 ? (
        <p className="empty">No orders yet.</p>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Time</th>
                <th>ID</th>
                <th>Side</th>
                <th>Type</th>
                <th>Status</th>
                <th className="num">Price</th>
                <th className="num">Quantity</th>
                <th className="num">Total ({quote})</th>
                <th className="num">Profit</th>
              </tr>
            </thead>
            <tbody>
              {visible.map((order) => (
                <tr key={order.id}>
                  <td className="mono">{formatTime(order.updated_at)}</td>
                  <td className="mono">{order.id}</td>
                  <td className={order.side === "BUY" ? "pos" : "neg"}>{order.side}</td>
                  <td>{order.type}</td>
                  <td>
                    <span className={`badge ${order.status.toLowerCase()}`}>{order.status}</span>
                  </td>
                  <td className="num mono">{formatPrice(order.price)}</td>
                  <td className="num mono">{formatQuantity(order.quantity)}</td>
                  <td className="num mono">{formatNumber(order.price * order.quantity)}</td>
                  <td className={`num mono ${signClass(order.profit)}`}>
                    {order.profit !== 0 || order.profit_value !== 0
                      ? `${formatPercent(order.profit)} (${formatNumber(order.profit_value)})`
                      : "—"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {sorted.length > limit && (
        <button type="button" className="button ghost" onClick={() => setLimit((l) => l + PAGE)}>
          Show more ({sorted.length - limit} remaining)
        </button>
      )}
    </section>
  );
}
