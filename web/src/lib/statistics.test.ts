import { describe, expect, it } from "vitest";
import type { HypothesisTest, Interval, Statistics } from "../api/types";
import {
  crossesThreshold,
  formatIntervalValue,
  formatProbability,
  formatPValue,
  formatRatio,
  INTERVAL_META,
  sampleWarning,
  TEST_META,
  verdictText,
} from "./statistics";

function interval(partial: Partial<Interval> = {}): Interval {
  return {
    id: "mean_return",
    mean: 0.01,
    lower: 0.005,
    upper: 0.015,
    std_dev: 0.002,
    threshold: 0,
    format: "percent",
    ...partial,
  };
}

function test(partial: Partial<HypothesisTest> = {}): HypothesisTest {
  return {
    id: "mean_return_t",
    statistic: 3.2,
    statistic_label: "t",
    p_value: 0.001,
    reject_is_good: true,
    verdict: "pass",
    ...partial,
  };
}

function statistics(partial: Partial<Statistics> = {}): Statistics {
  return {
    pair: "BTCUSDT",
    quote: "USDT",
    sample: {
      trades: 60,
      wins: 40,
      losses: 20,
      bars: 500,
      bars_per_year: 8760,
      iterations: 5000,
      seed: 1,
    },
    performance: {
      net_profit: 0,
      return_pct: 0,
      buy_and_hold: 0,
      win_rate: 0,
      mean_return: 0,
      median_return: 0,
      expectancy: 0,
      payoff: 0,
      profit_factor: 0,
      sqn: 0,
      sharpe_per_trade: 0,
      sharpe_annual: 0,
      sortino_annual: 0,
      calmar: 0,
      alpha: 0,
      beta: 0,
    },
    risk: {
      max_drawdown: 0,
      value_at_risk: 0,
      expected_shortfall: 0,
      worst_trade: 0,
      best_trade: 0,
      longest_losing_streak: 0,
      skew: 0,
      excess_kurtosis: 0,
      autocorrelation: 0,
      break_even_fee: 0,
      probabilistic_sharpe: 0,
      minimum_track_record: 10,
    },
    intervals: [],
    tests: [],
    distribution: [],
    windows: [],
    ...partial,
  };
}

describe("formatPValue", () => {
  it("does not pretend precision it does not have", () => {
    expect(formatPValue(0.0004)).toBe("p < 0.001");
    expect(formatPValue(0.0321)).toBe("p = 0.032");
    expect(formatPValue(Number.NaN)).toBe("—");
  });
});

describe("formatRatio", () => {
  it("keeps an unbounded ratio readable", () => {
    expect(formatRatio(1.234)).toBe("1.23");
    expect(formatRatio(Number.POSITIVE_INFINITY)).toBe("∞");
  });
});

describe("formatProbability", () => {
  it("drops the sign a probability never needs", () => {
    expect(formatProbability(0.973)).toBe("97.3%");
    expect(formatProbability(0.0005, 3)).toBe("0.050%");
  });
});

describe("formatIntervalValue", () => {
  it("follows the format the API declares", () => {
    expect(formatIntervalValue(interval(), 0.0123)).toBe("+1.23%");
    expect(formatIntervalValue(interval({ format: "ratio" }), 1.5)).toBe("1.50");
  });
});

describe("crossesThreshold", () => {
  it("is the reading that matters: can a no-edge strategy be ruled out?", () => {
    expect(crossesThreshold(interval({ lower: 0.005, upper: 0.02, threshold: 0 }))).toBe(false);
    expect(crossesThreshold(interval({ lower: -0.004, upper: 0.02, threshold: 0 }))).toBe(true);
    expect(crossesThreshold(interval({ lower: 0.9, upper: 1.4, threshold: 1 }))).toBe(true);
  });
});

describe("verdictText", () => {
  it("explains the verdict of every test the API can return", () => {
    for (const id of Object.keys(TEST_META)) {
      expect(verdictText(test({ id, verdict: "pass" }))).toBe(TEST_META[id]?.pass);
      expect(verdictText(test({ id, verdict: "warn" }))).toBe(TEST_META[id]?.warn);
      expect(verdictText(test({ id, verdict: "inconclusive" }))).toContain("Not enough trades");
    }
  });

  it("stays quiet about a test it does not know", () => {
    expect(verdictText(test({ id: "future_test" }))).toBe("");
  });
});

describe("metadata", () => {
  // The API sends ids, the dashboard owns the words: a new test on the Go side
  // has to land here too, and this is what says so.
  it("covers the ids of the Go report", () => {
    expect(Object.keys(TEST_META).sort()).toEqual([
      "independence_runs",
      "mean_return_bootstrap",
      "mean_return_t",
      "stability_halves",
    ]);
    expect(Object.keys(INTERVAL_META).sort()).toEqual(["mean_return", "payoff", "profit_factor"]);
  });
});

describe("sampleWarning", () => {
  it("says when there is nothing to test", () => {
    expect(sampleWarning(statistics({ sample: { ...statistics().sample, trades: 0 } }))).toContain(
      "nothing to test",
    );
  });

  it("warns about a sample too small to conclude anything", () => {
    const small = statistics({ sample: { ...statistics().sample, trades: 7 } });
    expect(sampleWarning(small)).toContain("Only 7 closed trades");
  });

  it("warns when the Sharpe needs more trades than the backtest has", () => {
    const short = statistics({
      risk: { ...statistics().risk, minimum_track_record: 180.2 },
    });
    expect(sampleWarning(short)).toContain("181 trades");
  });

  it("stays quiet when the sample supports the report", () => {
    expect(sampleWarning(statistics())).toBeNull();
  });
});
