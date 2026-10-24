import { describe, expect, it } from "vitest";
import { averageCorrelation, correlationColor, STRATEGY_COLORS, strategyColor } from "./portfolio";

describe("portfolio helpers", () => {
  it("cycles the strategy colors", () => {
    expect(strategyColor(0)).toBe(STRATEGY_COLORS[0]);
    expect(strategyColor(STRATEGY_COLORS.length)).toBe(STRATEGY_COLORS[0]);
  });

  it("colors correlations by sign and strength", () => {
    expect(correlationColor(0)).toBe("rgba(45, 212, 191, 0.000)");
    expect(correlationColor(1)).toBe("rgba(45, 212, 191, 0.550)");
    expect(correlationColor(-0.5)).toBe("rgba(251, 113, 133, 0.275)");
    expect(correlationColor(3)).toBe("rgba(45, 212, 191, 0.550)");
    expect(correlationColor(Number.NaN)).toBe("transparent");
  });

  it("averages the correlations between distinct strategies", () => {
    expect(
      averageCorrelation([
        [1, 0.2, 0.4],
        [0.2, 1, 0],
        [0.4, 0, 1],
      ]),
    ).toBeCloseTo(0.2);
    expect(averageCorrelation([[1]])).toBeNull();
    expect(averageCorrelation([])).toBeNull();
  });
});
