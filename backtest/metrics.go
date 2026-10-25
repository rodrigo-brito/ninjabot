package backtest

import (
	"fmt"
	"math"

	str2duration "github.com/xhit/go-str2duration/v2"

	"github.com/rodrigo-brito/ninjabot/tools/metrics"
)

// Metrics are the headline numbers of a run. They are cheap to compute, so
// every result carries them, and they are what sweeps rank and compare.
type Metrics struct {
	Trades       int     `json:"trades"`
	NetProfit    float64 `json:"net_profit"`
	ReturnPct    float64 `json:"return_pct"`
	BuyAndHold   float64 `json:"buy_and_hold"`
	MaxDrawdown  float64 `json:"max_drawdown"`
	WinRate      float64 `json:"win_rate"`
	ProfitFactor float64 `json:"profit_factor"`
	Payoff       float64 `json:"payoff"`
	Expectancy   float64 `json:"expectancy"`
	SQN          float64 `json:"sqn"`
	// The annualized ratios are measured on the bar returns of the equity.
	SharpeAnnual  float64 `json:"sharpe_annual"`
	SortinoAnnual float64 `json:"sortino_annual"`
	Calmar        float64 `json:"calmar"`
	Fees          float64 `json:"fees"`
}

// MetricNames lists the metrics a sweep can rank by, with whether a higher
// value is better.
var MetricNames = map[string]bool{
	"trades": true, "net_profit": true, "return_pct": true, "buy_and_hold": true, "max_drawdown": false,
	"win_rate": true, "profit_factor": true, "payoff": true, "expectancy": true, "sqn": true,
	"sharpe_annual": true, "sortino_annual": true, "calmar": true, "fees": false,
}

// Value returns a metric by its JSON name.
func (m Metrics) Value(name string) (float64, error) {
	values := map[string]float64{
		"trades": float64(m.Trades), "net_profit": m.NetProfit, "return_pct": m.ReturnPct,
		"buy_and_hold": m.BuyAndHold, "max_drawdown": m.MaxDrawdown, "win_rate": m.WinRate,
		"profit_factor": m.ProfitFactor, "payoff": m.Payoff, "expectancy": m.Expectancy, "sqn": m.SQN,
		"sharpe_annual": m.SharpeAnnual, "sortino_annual": m.SortinoAnnual, "calmar": m.Calmar, "fees": m.Fees,
	}
	value, ok := values[name]
	if !ok {
		return 0, fmt.Errorf("unknown metric %q", name)
	}
	return value, nil
}

// computeMetrics derives the headline numbers of a result.
func computeMetrics(r *Result) Metrics {
	m := Metrics{
		Trades:      len(r.Trades),
		NetProfit:   r.Wallet.NetProfit,
		ReturnPct:   r.Wallet.ReturnPct,
		BuyAndHold:  r.Wallet.BuyAndHold,
		MaxDrawdown: r.Wallet.MaxDrawdown.Value,
		Fees:        r.Wallet.Fees,
	}

	returns := make([]float64, len(r.Trades))
	pnl := make([]float64, len(r.Trades))
	wins := 0
	for i, trade := range r.Trades {
		returns[i] = trade.ProfitPercent
		pnl[i] = trade.ProfitValue
		if trade.ProfitPercent > 0 {
			wins++
		}
	}
	if len(returns) > 0 {
		m.WinRate = float64(wins) / float64(len(returns))
		m.ProfitFactor = metrics.ProfitFactor(returns)
		m.Payoff = metrics.Payoff(returns)
		m.Expectancy = metrics.Mean(pnl)
	}
	if len(pnl) > 1 {
		if deviation := metrics.StdDev(pnl); deviation > 0 {
			m.SQN = math.Sqrt(float64(len(pnl))) * metrics.Mean(pnl) / deviation
		}
	}

	barReturns := make([]float64, 0, len(r.Equity))
	for i := 1; i < len(r.Equity); i++ {
		if previous := r.Equity[i-1].Value; previous > 0 {
			barReturns = append(barReturns, (r.Equity[i].Value-previous)/previous)
		}
	}
	if bars := barsPerYear(r.Config.Timeframe); bars > 0 && len(barReturns) > 1 {
		m.SharpeAnnual = metrics.Annualize(metrics.Sharpe(barReturns), bars)
		m.SortinoAnnual = metrics.Annualize(metrics.Sortino(barReturns), bars)
		if years := float64(len(barReturns)) / bars; years > 0 && m.MaxDrawdown > 0 {
			m.Calmar = (m.ReturnPct / years) / m.MaxDrawdown
		}
	}

	for _, value := range []*float64{&m.ProfitFactor, &m.Payoff, &m.Expectancy, &m.SQN,
		&m.SharpeAnnual, &m.SortinoAnnual, &m.Calmar} {
		if math.IsNaN(*value) || math.IsInf(*value, 0) {
			*value = 0
		}
	}
	return m
}

// barsPerYear is the number of bars of a timeframe in a year, zero when the
// timeframe is unknown.
func barsPerYear(timeframe string) float64 {
	duration, err := str2duration.ParseDuration(timeframe)
	if err != nil || duration <= 0 {
		return 0
	}
	return float64(365*24*60*60) / duration.Seconds()
}
