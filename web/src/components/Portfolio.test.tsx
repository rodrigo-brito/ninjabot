import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { PortfolioView } from "../api/types";
import { PortfolioPanel } from "./Portfolio";

function view(partial: Partial<PortfolioView> = {}): PortfolioView {
  return {
    name: "trend + breakout",
    quote: "USDT",
    initial: 10000,
    final: 27069,
    net_profit: 17069,
    return_pct: 1.7069,
    max_drawdown: 0.1442,
    sharpe_annual: 4.449,
    unallocated: 2000,
    equity: [
      { time: 1, value: 10000 },
      { time: 2, value: 27069 },
    ],
    strategies: [
      {
        id: "trend",
        strategy: "emacross",
        weight: 0.5,
        capital: 5000,
        trades: 23,
        net_profit: 16612,
        return_pct: 3.3225,
        max_drawdown: 0.1832,
        sharpe_annual: 4.151,
        pnl_share: 0.973,
        marginal_sharpe: 3.696,
        equity: [],
      },
      {
        id: "breakout",
        strategy: "turtle",
        weight: 0.3,
        capital: 3000,
        trades: 6,
        net_profit: 456,
        return_pct: 0.1521,
        max_drawdown: 0.2531,
        sharpe_annual: 0.967,
        pnl_share: 0.027,
        marginal_sharpe: -0.161,
        equity: [],
      },
    ],
    correlation: [
      [1, 0.02],
      [0.02, 1],
    ],
    ...partial,
  };
}

describe("PortfolioPanel", () => {
  it("renders the summary, the strategies and the correlation", () => {
    const html = renderToStaticMarkup(<PortfolioPanel portfolio={view()} theme="dark" />);
    expect(html).toContain("trend + breakout");
    expect(html).toContain("+170.69%");
    expect(html).toContain("-14.42%");
    expect(html).toContain("emacross");
    expect(html).toContain("+3.696");
    expect(html).toContain("-0.161");
    expect(html).toContain("Correlation of returns");
    expect(html).toContain("0.02");
    expect(html).toContain("Unallocated cash: 2,000 USDT");
  });

  it("hides the correlation of a single strategy", () => {
    const single = view({
      strategies: view().strategies.slice(0, 1),
      correlation: [[1]],
      unallocated: 0,
    });
    const html = renderToStaticMarkup(<PortfolioPanel portfolio={single} theme="light" />);
    expect(html).not.toContain("Correlation of returns");
    expect(html).not.toContain("Unallocated");
    expect(html).toContain("—");
  });
});
