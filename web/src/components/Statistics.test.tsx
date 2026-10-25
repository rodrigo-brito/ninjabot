import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { Statistics } from "../api/types";
import { StatisticsPanel } from "./Statistics";

/**
 * The panel is rendered to static markup: no browser needed to check that
 * every section reaches the page, and that the ones without data degrade into
 * an explanation instead of an empty box.
 */

function report(partial: Partial<Statistics> = {}): Statistics {
  return {
    pair: "BTCUSDT",
    quote: "USDT",
    sample: {
      trades: 60,
      wins: 40,
      losses: 20,
      bars: 2154,
      bars_per_year: 2190,
      iterations: 5000,
      seed: 1,
    },
    performance: {
      net_profit: 12756.38,
      return_pct: 3.32,
      buy_and_hold: 1.82,
      win_rate: 0.66,
      mean_return: 0.069,
      median_return: 0.017,
      expectancy: 911.17,
      payoff: 5.81,
      profit_factor: 4.36,
      sqn: 1.37,
      sharpe_per_trade: 0.385,
      sharpe_annual: 2.92,
      sortino_annual: 4.59,
      calmar: 18.45,
      alpha: 0.98,
      beta: 0.52,
    },
    risk: {
      max_drawdown: 0.183,
      value_at_risk: -0.077,
      expected_shortfall: -0.077,
      worst_trade: -0.077,
      best_trade: 0.553,
      longest_losing_streak: 2,
      skew: 1.89,
      excess_kurtosis: 3.27,
      autocorrelation: -0.19,
      break_even_fee: 0.034,
      probabilistic_sharpe: 0.978,
      minimum_track_record: 9.5,
    },
    intervals: [
      {
        id: "mean_return",
        mean: 0.069,
        lower: -0.008,
        upper: 0.169,
        std_dev: 0.045,
        threshold: 0,
        format: "percent",
      },
      {
        id: "profit_factor",
        mean: 5.36,
        lower: 1.67,
        upper: 16.48,
        std_dev: 4.91,
        threshold: 1,
        format: "ratio",
      },
    ],
    tests: [
      {
        id: "mean_return_t",
        statistic: 3.2,
        statistic_label: "t",
        p_value: 0.0004,
        reject_is_good: true,
        verdict: "pass",
      },
      {
        id: "independence_runs",
        statistic: -2.1,
        statistic_label: "z",
        p_value: 0.03,
        reject_is_good: false,
        verdict: "warn",
      },
    ],
    monte_carlo: {
      iterations: 5000,
      final_return: {
        mean: 1.2,
        min: -0.2,
        p5: 0.1,
        p25: 0.7,
        median: 1.2,
        p75: 1.7,
        p95: 2.4,
        max: 3.9,
      },
      max_drawdown: {
        mean: 0.16,
        min: 0.02,
        p5: 0.05,
        p25: 0.11,
        median: 0.15,
        p75: 0.21,
        p95: 0.32,
        max: 0.5,
      },
      profit_probability: 0.946,
      ruin_probability: 0.2596,
      ruin_threshold: 0.2,
      observed: 1.2756,
    },
    distribution: [
      { start: -0.08, end: -0.02, count: 8 },
      { start: -0.02, end: 0.04, count: 40 },
      { start: 0.04, end: 0.1, count: 12 },
    ],
    windows: [
      { index: 0, trades: 15, mean_return: 0.04, net_profit: 900, win_rate: 0.6, start: 1, end: 2 },
      {
        index: 1,
        trades: 15,
        mean_return: -0.01,
        net_profit: -200,
        win_rate: 0.4,
        start: 3,
        end: 4,
      },
    ],
    ...partial,
  };
}

describe("StatisticsPanel", () => {
  it("renders every section of the report", () => {
    const html = renderToStaticMarkup(
      <StatisticsPanel statistics={report()} loading={false} error={null} />,
    );

    expect(html).toContain("Robustness");
    expect(html).toContain("60 closed trades");
    expect(html).toContain("5,000 resamples");

    // risk-adjusted return and tails
    expect(html).toContain("Sharpe (annual)");
    expect(html).toContain("2.92");
    expect(html).toContain("Prob. Sharpe");
    expect(html).toContain("97.8%");
    expect(html).toContain("Max drawdown");
    expect(html).toContain("Break-even fee");

    // intervals, with the reading of each one
    expect(html).toContain("Mean return per trade");
    expect(html).toContain("still contains zero");
    expect(html).toContain("clears 1.0");

    // hypothesis tests
    expect(html).toContain("The average trade beats zero");
    expect(html).toContain("p &lt; 0.001");
    expect(html).toContain("Wins and losses are independent");
    expect(html).toContain("narrower than it should be");

    // simulations
    expect(html).toContain("94.6%");
    expect(html).toContain("of the runs ended in profit");
    expect(html).toContain("26.0%");

    // distribution and stability
    expect(html.match(/histogram-bar (pos|neg)/g)).toHaveLength(3);
    expect(html.match(/window-bar (pos|neg)/g)).toHaveLength(2);
  });

  it("offers an explanation for every metric it shows", () => {
    const html = renderToStaticMarkup(
      <StatisticsPanel statistics={report()} loading={false} error={null} />,
    );

    // Every tile, interval, test and chart carries its own "?" trigger.
    const triggers = html.match(/aria-label="What ([^"]+) means"/g) ?? [];
    const explained = triggers.map((trigger) =>
      trigger.replace(/aria-label="What (.+) means"/, "$1"),
    );

    for (const metric of [
      "Sharpe (annual)",
      "Sortino (annual)",
      "Calmar",
      "Sharpe (per trade)",
      "SQN",
      "Prob. Sharpe",
      "Alpha (annual)",
      "Max drawdown",
      "Value at risk 95%",
      "Expected shortfall",
      "Worst trade",
      "Break-even fee",
      "Skew",
      "Excess kurtosis",
      "Autocorrelation",
      "Expectancy",
      "Mean return per trade",
      "Profit factor",
      "The average trade beats zero",
      "Wins and losses are independent",
      "Final return",
      "Confidence intervals",
      "Hypothesis tests",
      "Monte Carlo",
      "Trade returns",
      "Stability",
      "Simulated outcomes",
      "Resamples and seed",
    ]) {
      expect(explained).toContain(metric);
    }

    // The balloon itself only exists once opened, so nothing is in the markup.
    expect(html).not.toContain("infotip-balloon");
  });

  it("explains what a backtest without a paper wallet cannot show", () => {
    const html = renderToStaticMarkup(
      <StatisticsPanel
        statistics={report({ monte_carlo: undefined, sample: { ...report().sample, bars: 0 } })}
        loading={false}
        error={null}
      />,
    );

    expect(html).toContain("needs a paper wallet");
    expect(html).toContain("Run the backtest with a paper wallet");
    expect(html).not.toContain("Sharpe (annual)");
    expect(html).toContain("Sharpe (per trade)");
  });

  it("warns instead of concluding from a handful of trades", () => {
    const small = report({
      sample: { ...report().sample, trades: 7, wins: 4, losses: 3 },
    });
    const html = renderToStaticMarkup(
      <StatisticsPanel statistics={small} loading={false} error={null} />,
    );

    expect(html).toContain("Only 7 closed trades");
  });

  it("says when the Sharpe rests on too short a record", () => {
    const html = renderToStaticMarkup(
      <StatisticsPanel
        statistics={report({ risk: { ...report().risk, minimum_track_record: 180.2 } })}
        loading={false}
        error={null}
      />,
    );

    expect(html).toContain("about 181 trades");
  });

  it("keeps its place while loading and reports a failure", () => {
    const loading = renderToStaticMarkup(
      <StatisticsPanel statistics={null} loading={true} error={null} />,
    );
    expect(loading).toContain("Running the simulations");

    const failed = renderToStaticMarkup(
      <StatisticsPanel statistics={null} loading={false} error="boom" />,
    );
    expect(failed).toContain("boom");
  });
});
