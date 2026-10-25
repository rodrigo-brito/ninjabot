import type { ReactNode } from "react";
import type { Statistics } from "../api/types";
import { formatNumber, formatPercent, signClass } from "../lib/format";
import {
  formatProbability,
  formatPValue,
  formatRatio,
  INTERVAL_META,
  SECTION_HELP,
  sampleWarning,
  TEST_META,
  verdictText,
} from "../lib/statistics";
import { InfoTip } from "./InfoTip";
import { ConfidenceBar, PercentileRange, ReturnHistogram, WindowBars } from "./StatisticsCharts";

interface Props {
  statistics: Statistics | null;
  loading: boolean;
  error: string | null;
}

interface Metric {
  label: string;
  value: string;
  /** Required: a metric nobody can explain has no place on the page. */
  help: ReactNode;
  hint?: string;
  tone?: string;
}

/**
 * The robustness panel: everything the terminal summary prints, plus the
 * resampling tests that say how much of the result is edge and how much is
 * luck. The equity curve above answers "how much did it make"; this answers
 * "would it make it again".
 *
 * Every number carries a "?" with what it measures and how it misleads: these
 * are statistics, and a statistic read without its assumptions is worse than
 * no statistic at all.
 */
export function StatisticsPanel({ statistics, loading, error }: Props) {
  if (error) {
    return (
      <section className="card">
        <div className="card-title">
          <span>Robustness</span>
        </div>
        <p className="empty">{error}</p>
      </section>
    );
  }

  if (!statistics) {
    return (
      <section className="card">
        <div className="card-title">
          <span>Robustness</span>
        </div>
        <p className="empty">{loading ? "Running the simulations…" : "No report yet."}</p>
      </section>
    );
  }

  const { sample, performance, risk } = statistics;
  const hasEquity = sample.bars > 0;
  const warning = sampleWarning(statistics);

  const quality: Metric[] = [
    ...(hasEquity
      ? [
          {
            label: "Sharpe (annual)",
            value: formatRatio(performance.sharpe_annual),
            hint: "return per unit of volatility",
            tone: signClass(performance.sharpe_annual),
            help: "Average return of the equity curve divided by its volatility, annualized from the spacing of the candles. Above 1 is good and above 2 is rare outside a fitted backtest. It counts gains and losses as risk alike, and it understates the danger when returns have fat tails.",
          },
          {
            label: "Sortino (annual)",
            value: formatRatio(performance.sortino_annual),
            hint: "downside volatility only",
            tone: signClass(performance.sortino_annual),
            help: "Sharpe with only the downside deviation in the denominator, so upside swings are not punished. Read it next to Sharpe: a Sortino much higher than the Sharpe means the volatility of the strategy is mostly in its favor.",
          },
          {
            label: "Calmar",
            value: formatRatio(performance.calmar),
            hint: "yearly return / max drawdown",
            tone: signClass(performance.calmar),
            help: "Yearly return divided by the deepest drawdown: how much the strategy earns per unit of the worst loss it made you sit through. Short backtests inflate it, because the worst drawdown simply has not happened yet.",
          },
        ]
      : []),
    {
      label: "Sharpe (per trade)",
      value: formatRatio(performance.sharpe_per_trade),
      hint: "edge per unit of trade noise",
      tone: signClass(performance.sharpe_per_trade),
      help: "Average trade return divided by the spread of the trades. Unlike the annual Sharpe it ignores how long each trade took, so it measures the edge per trade and does not reward trading more often.",
    },
    {
      label: "SQN",
      value: formatRatio(performance.sqn, 1),
      hint: "system quality number",
      tone: signClass(performance.sqn),
      help: "System Quality Number of Van Tharp: the t-statistic of the profit and loss, sqrt(trades) × mean / standard deviation. Around 2 is tradable and above 3 is good. It grows with the square root of the trade count, so a long backtest flatters it.",
    },
    {
      label: "Prob. Sharpe",
      value: formatProbability(risk.probabilistic_sharpe),
      hint: "chance the true Sharpe is positive",
      tone: risk.probabilistic_sharpe >= 0.95 ? "pos" : "warn",
      help: "Probability that the true Sharpe ratio is above zero, once the number of trades, the skew and the fat tails of the sample are accounted for (Bailey and Lopez de Prado). Below 95% the observed Sharpe is not distinguishable from noise, however high it looks.",
    },
    ...(hasEquity
      ? [
          {
            label: "Alpha (annual)",
            value: formatPercent(performance.alpha),
            hint: `beta ${formatRatio(performance.beta)} vs holding the asset`,
            tone: signClass(performance.alpha),
            help: "The yearly return that holding the asset does not explain. Beta next to it is the exposure: 1 moves with the market, 0 is uncorrelated. Alpha near zero with a beta near 1 means the strategy is an expensive way to hold the asset.",
          },
        ]
      : []),
  ];

  const tails: Metric[] = [
    ...(hasEquity
      ? [
          {
            label: "Max drawdown",
            value: formatProbability(risk.max_drawdown),
            hint: "deepest peak-to-trough drop, in percent",
            tone: "neg",
            help: "Largest decline from a peak of the equity curve to the trough that follows it, in percent. This is the loss you would have had to live through to collect the return above. The tile at the top of the page reports the largest decline in quote currency instead, so the two rarely point at the same week.",
          },
        ]
      : []),
    {
      label: "Value at risk 95%",
      value: formatPercent(risk.value_at_risk),
      hint: "worst 5% of the trades start here",
      tone: "neg",
      help: "The trade return that the worst 5% of the trades fall below. It marks where the tail begins, not how deep it goes -- for that, read the expected shortfall next to it.",
    },
    {
      label: "Expected shortfall",
      value: formatPercent(risk.expected_shortfall),
      hint: "average of that tail",
      tone: "neg",
      help: "Average of the trades beyond the value at risk: when it goes wrong, this is how bad it goes on average. On a small sample it collapses onto the worst trade, since the tail holds a single trade.",
    },
    {
      label: "Worst trade",
      value: formatPercent(risk.worst_trade),
      hint: `longest losing streak: ${risk.longest_losing_streak}`,
      tone: "neg",
      help: "Largest single loss of the backtest, with the longest run of consecutive losers next to it. Together they are what a stop, a position size and your patience have to survive.",
    },
    {
      label: "Break-even fee",
      value: formatProbability(risk.break_even_fee, risk.break_even_fee < 0.01 ? 3 : 2),
      hint: "round-trip cost that cancels the edge",
      tone: risk.break_even_fee > 0.001 ? "pos" : "warn",
      help: "The fee per fill that would wipe out the average trade -- half the average return, since a round trip pays it twice. Compare it with the fee of your exchange: if it is not comfortably above, the strategy only works on paper.",
    },
    {
      label: "Skew",
      value: formatRatio(risk.skew),
      hint: "negative means rare big losses",
      tone: risk.skew < 0 ? "warn" : "",
      help: "Asymmetry of the trade returns. Negative means many small wins and rare large losses, the shape that looks safe on an equity curve until the day it is not. Positive means the opposite: frequent small losses paid for by rare large winners.",
    },
    {
      label: "Excess kurtosis",
      value: formatRatio(risk.excess_kurtosis),
      hint: "fatter tails than a bell curve",
      tone: risk.excess_kurtosis > 3 ? "warn" : "",
      help: "How much fatter the tails are than a normal distribution, which sits at zero. High values mean the extremes drive the result and that any test assuming normality understates the risk -- which is why the bootstrap tests below exist.",
    },
    {
      label: "Autocorrelation",
      value: formatRatio(risk.autocorrelation),
      hint: "lag-1, trade to trade",
      tone: Math.abs(risk.autocorrelation) > 0.2 ? "warn" : "",
      help: "Correlation between each trade and the next one. Far from zero means the trades are not independent, so the confidence intervals below are narrower than they should be and the runs test will say so.",
    },
    {
      label: "Expectancy",
      value: formatNumber(performance.expectancy),
      hint: `${statistics.quote} per trade`,
      tone: signClass(performance.expectancy),
      help: `Average profit of a closed trade in ${statistics.quote}. Multiply it by the trades you expect in a month and you have what the strategy claims it can produce, before it is discounted by everything else on this page.`,
    },
  ];

  return (
    <section className="card statistics">
      <div className="card-title">
        <span>
          Robustness <small>{sample.trades} closed trades</small>
        </span>
        <span className="statistics-meta">
          {formatNumber(sample.iterations)} resamples · seed {sample.seed}
          <InfoTip label="Resamples and seed">{SECTION_HELP.sample}</InfoTip>
        </span>
      </div>

      {warning && <p className="banner warn">{warning}</p>}

      <MetricGrid title="Risk-adjusted return" metrics={quality} />
      <MetricGrid title="Tails and costs" metrics={tails} />

      <div className="panel-grid">
        <section className="panel">
          <h3>
            Confidence intervals <small>95%, bootstrap</small>
            <InfoTip label="Confidence intervals">{SECTION_HELP.intervals}</InfoTip>
          </h3>
          <p className="panel-note">
            Where the measure lands when the same trades are drawn again. An interval that still
            covers its threshold cannot rule out a strategy with no edge.
          </p>
          {statistics.intervals.length === 0 ? (
            <p className="empty">Not enough trades.</p>
          ) : (
            statistics.intervals.map((interval) => {
              const meta = INTERVAL_META[interval.id];
              return (
                <ConfidenceBar
                  key={interval.id}
                  interval={interval}
                  name={meta?.name ?? interval.id}
                  thresholdLabel={meta?.thresholdLabel ?? String(interval.threshold)}
                  help={meta?.help ?? SECTION_HELP.intervals}
                />
              );
            })
          )}
        </section>

        <section className="panel">
          <h3>
            Hypothesis tests
            <InfoTip label="Hypothesis tests">{SECTION_HELP.tests}</InfoTip>
          </h3>
          <p className="panel-note">
            The first two ask whether the edge is real. The last two check the assumption they rest
            on: independent trades, and a period that does not split in two.
          </p>
          <ul className="tests">
            {statistics.tests.map((test) => {
              const meta = TEST_META[test.id];
              const name = meta?.name ?? test.id;
              return (
                <li className={`test ${test.verdict}`} key={test.id}>
                  <div className="test-head">
                    <span className="test-name">
                      {name}
                      {meta && <InfoTip label={name}>{meta.help}</InfoTip>}
                    </span>
                    <span className={`badge ${test.verdict}`}>{formatPValue(test.p_value)}</span>
                  </div>
                  <p className="test-hypothesis">{meta?.hypothesis}</p>
                  <p className="test-verdict">{verdictText(test)}</p>
                </li>
              );
            })}
          </ul>
        </section>
      </div>

      <div className="panel-grid">
        <section className="panel">
          <h3>
            Trade returns{" "}
            <small>
              {sample.wins} won · {sample.losses} lost
            </small>
            <InfoTip label="Trade returns">{SECTION_HELP.distribution}</InfoTip>
          </h3>
          <p className="panel-note">
            The shape behind the average: a long left tail means the losses that matter are the rare
            ones.
          </p>
          <ReturnHistogram bins={statistics.distribution} />
        </section>

        <section className="panel">
          <h3>
            Monte Carlo{" "}
            <small>
              {statistics.monte_carlo
                ? `${formatNumber(statistics.monte_carlo.iterations)} resampled sequences`
                : "needs a paper wallet"}
            </small>
            <InfoTip label="Monte Carlo">{SECTION_HELP.monteCarlo}</InfoTip>
          </h3>
          <p className="panel-note">
            The same trades, drawn in a different order and a different mix. It separates the edge
            from the luck of the sequence that happened to be simulated.
          </p>
          {statistics.monte_carlo ? (
            <>
              <PercentileRange
                name="Final return"
                percentiles={statistics.monte_carlo.final_return}
                observed={statistics.monte_carlo.observed}
                tone={statistics.monte_carlo.final_return.median < 0 ? "neg" : "pos"}
                help={SECTION_HELP.finalReturn}
              />
              <PercentileRange
                name="Max drawdown"
                percentiles={statistics.monte_carlo.max_drawdown}
                observed={risk.max_drawdown}
                tone="neg"
                signed={false}
                help={SECTION_HELP.simulatedDrawdown}
              />
              <div className="simulation-summary">
                <span>
                  <strong className={statistics.monte_carlo.profit_probability >= 0.9 ? "pos" : ""}>
                    {formatProbability(statistics.monte_carlo.profit_probability)}
                  </strong>{" "}
                  of the runs ended in profit
                </span>
                <span>
                  <strong className={statistics.monte_carlo.ruin_probability > 0.05 ? "neg" : ""}>
                    {formatProbability(statistics.monte_carlo.ruin_probability)}
                  </strong>{" "}
                  went past a {formatProbability(statistics.monte_carlo.ruin_threshold, 0)} drawdown
                  <InfoTip label="Simulated outcomes">{SECTION_HELP.outcomes}</InfoTip>
                </span>
              </div>
            </>
          ) : (
            <p className="empty">Run the backtest with a paper wallet to simulate the equity.</p>
          )}
        </section>
      </div>

      {statistics.windows.length > 0 && (
        <section className="panel">
          <h3>
            Stability <small>average return per slice of the history</small>
            <InfoTip label="Stability">{SECTION_HELP.windows}</InfoTip>
          </h3>
          <p className="panel-note">
            An edge that lives in a single window is a fit to that window, however good the total
            looks.
          </p>
          <WindowBars windows={statistics.windows} />
        </section>
      )}
    </section>
  );
}

function MetricGrid({ title, metrics }: { title: string; metrics: Metric[] }) {
  if (metrics.length === 0) return null;
  return (
    <div className="metric-block">
      <h3>{title}</h3>
      <div className="metric-grid">
        {metrics.map((metric) => (
          <div className="metric" key={metric.label}>
            <span className="stat-label">
              {metric.label}
              <InfoTip label={metric.label}>{metric.help}</InfoTip>
            </span>
            <span className={`metric-value ${metric.tone ?? ""}`}>{metric.value}</span>
            {metric.hint && <span className="stat-hint">{metric.hint}</span>}
          </div>
        ))}
      </div>
    </div>
  );
}
