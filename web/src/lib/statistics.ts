import type { HypothesisTest, Interval, Statistics } from "../api/types";
import { formatNumber, formatPercent } from "./format";

/**
 * The API reports the robustness of a backtest as numbers plus stable ids; the
 * wording lives here, next to the dashboard that shows it. Every entry answers
 * the same question in plain words: what does this number say about trusting
 * the result?
 */

export interface TestMeta {
  name: string;
  /** The null hypothesis, in the notation a report would use. */
  hypothesis: string;
  /** What the verdict means, one line each. */
  pass: string;
  warn: string;
  /** What the test does and what it assumes, shown in the "?" balloon. */
  help: string;
}

export const TEST_META: Record<string, TestMeta> = {
  mean_return_t: {
    name: "The average trade beats zero",
    hypothesis: "H₀: the average trade return is zero (one-sided Student's t)",
    pass: "The average trade is too consistent to pass for chance.",
    warn: "The average trade is inside what chance alone would produce.",
    help: "Compares the average trade with zero using Student's t. The p-value is the chance of seeing an average this good if the strategy had no edge at all. It assumes the trades are independent and roughly bell-shaped -- the two tests below check the first assumption, the bootstrap the second.",
  },
  mean_return_bootstrap: {
    name: "…and without assuming a bell curve",
    hypothesis: "H₀: the average trade return is zero (bootstrap resampling)",
    pass: "The edge survives resampling, so it does not rest on normality.",
    warn: "Resampling reproduces this average often enough to doubt the edge.",
    help: "The same question without assuming any shape: the trades are recentered on zero and drawn again thousands of times, and the p-value is how often chance alone reaches the observed average. When it agrees with the t-test, the result does not hang on the bell curve.",
  },
  independence_runs: {
    name: "Wins and losses are independent",
    hypothesis: "H₀: wins and losses follow one another at random (Wald–Wolfowitz runs)",
    pass: "The sequence looks random, so the intervals above can be trusted.",
    warn: "Wins and losses cluster: every interval above is narrower than it should be.",
    help: "Counts the runs of consecutive wins and losses and compares them with what a coin flip produces. Too few runs means wins and losses cluster; too many means they alternate. Either way the trades are not independent, and every confidence interval on this page is narrower than it should be.",
  },
  stability_halves: {
    name: "The edge holds across the period",
    hypothesis: "H₀: both halves of the history have the same average return (permutation)",
    pass: "The two halves agree, so the edge is not tied to one stretch.",
    warn: "The halves disagree beyond chance: the edge decayed or arrived late.",
    help: "Splits the history in two and shuffles the trades between the halves thousands of times. The p-value is how often a random split opens a gap as wide as the observed one. A low value means the performance changed over the period, which is what a strategy fitted to one stretch looks like.",
  },
};

export interface IntervalMeta {
  name: string;
  /** What the threshold means when the interval crosses it. */
  thresholdLabel: string;
  /** What the measure is and how to read its interval. */
  help: string;
}

export const INTERVAL_META: Record<string, IntervalMeta> = {
  mean_return: {
    name: "Mean return per trade",
    thresholdLabel: "zero",
    help: "Where the average trade return lands when the same trades are drawn again with replacement, thousands of times. The bar is the 95% interval and the tick is its center. An interval that still contains zero means a strategy with no edge could have produced this backtest.",
  },
  payoff: {
    name: "Payoff (avg win / avg loss)",
    thresholdLabel: "1.0",
    help: "Average win divided by average loss. Above 1.0 the winners are bigger than the losers, which is what lets a strategy with a low win rate still make money. An interval crossing 1.0 means that advantage is not established.",
  },
  profit_factor: {
    name: "Profit factor",
    thresholdLabel: "1.0",
    help: "Gross profit divided by gross loss. Below 1.0 the strategy loses money, so an interval that still contains 1.0 cannot rule that out. It is dominated by the largest trades, which is why the interval is usually wide.",
  },
};

/** Explanations of the sections that are charts rather than tiles. */
export const SECTION_HELP = {
  intervals:
    "Each measure is recomputed on thousands of samples drawn from the same trades, with replacement. The spread of those results is the interval: it says how much the number would move if the same strategy had drawn a different sample of the same trades.",
  tests:
    "A p-value is the probability of seeing a result at least this extreme if the null hypothesis were true. Below 0.05 the null is rejected by convention. The first two tests want a low p-value, the last two want a high one.",
  distribution:
    "Each bar counts the trades whose return falls in that range. It is the shape behind the average: a long left tail means the losses that matter are rare and large, and a single tall bar on the right means the result rests on a handful of trades.",
  monteCarlo:
    "The trades are drawn again into thousands of new sequences and replayed over the initial equity, keeping the position sizing of the backtest. It answers what a single equity curve cannot: how much of the outcome depended on the order the trades happened to arrive in.",
  finalReturn:
    "Distribution of the final return across the simulated runs. The light band is the 5th to 95th percentile, the solid one the interquartile range, the tick the median, and the diamond is what the backtest produced. A backtest sitting near the top of the band was the lucky end of it.",
  simulatedDrawdown:
    "Deepest peak-to-trough decline of each simulated run. The drawdown of the backtest is one draw from this distribution -- if it sits at the low end, plan for the higher percentiles instead.",
  outcomes:
    "How the simulated runs ended: the share that finished in profit, and the share whose drawdown went past 20%, a level most strategies are abandoned well before reaching.",
  windows:
    "The trade history split into four consecutive slices, each with its own average return. A strategy that only makes money in one slice was fitted to it, however good the total looks.",
  sample:
    "Every bootstrap, permutation and Monte Carlo on this page runs from a fixed seed, so the same backtest always reports the same numbers. Reloading the dashboard cannot move them.",
} as const;

/** Formats a p-value the way a paper would, without pretending precision. */
export function formatPValue(value: number): string {
  if (!Number.isFinite(value)) return "—";
  if (value < 0.001) return "p < 0.001";
  return `p = ${value.toFixed(3)}`;
}

export function formatRatio(value: number, digits = 2): string {
  if (!Number.isFinite(value)) return "∞";
  return value.toFixed(digits);
}

/** Formats a value the way its interval declares it. */
export function formatIntervalValue(interval: Interval, value: number): string {
  return interval.format === "percent" ? formatPercent(value) : formatRatio(value);
}

/**
 * An interval that still contains its threshold cannot rule out a strategy
 * with no edge at all, which is the single most useful reading of the chart.
 */
export function crossesThreshold(interval: Interval): boolean {
  return interval.lower <= interval.threshold && interval.upper >= interval.threshold;
}

export function verdictText(test: HypothesisTest): string {
  const meta = TEST_META[test.id];
  if (!meta) return "";
  if (test.verdict === "inconclusive") return "Not enough trades to run this test.";
  return test.verdict === "pass" ? meta.pass : meta.warn;
}

/**
 * A backtest with a handful of trades cannot support any of this, and saying so
 * is more useful than printing a confident p-value over five samples.
 */
export const MIN_TRADES_FOR_TESTS = 20;

export function sampleWarning(statistics: Statistics): string | null {
  const { trades } = statistics.sample;
  if (trades === 0) return "No closed trades yet: there is nothing to test.";
  if (trades < MIN_TRADES_FOR_TESTS) {
    return `Only ${trades} closed trades. Every number below is a wide guess; treat them as hints, not evidence.`;
  }
  const record = statistics.risk.minimum_track_record;
  if (record > trades) {
    return `The Sharpe of this backtest needs about ${Math.ceil(record)} trades to be credible at 95%, and it has ${trades}.`;
  }
  return null;
}

/** Probabilities and costs read better without the leading sign. */
export function formatProbability(fraction: number, digits = 1): string {
  if (!Number.isFinite(fraction)) return "—";
  return `${(fraction * 100).toFixed(digits)}%`;
}

/** Right-hand label of the Monte Carlo bars. */
export function formatPercentile(value: number): string {
  return formatPercent(value, 1);
}

export function formatQuote(value: number, quote: string): string {
  return `${formatNumber(value)} ${quote}`;
}
