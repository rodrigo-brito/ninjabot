package ui

import (
	"net/http"

	"github.com/rodrigo-brito/ninjabot/portfolio"
)

// WithPortfolio shows a portfolio backtest on the dashboard.
func WithPortfolio(result *portfolio.Result) Option {
	return func(c *Chart) { c.portfolio = portfolioView(result) }
}

func portfolioView(result *portfolio.Result) *PortfolioView {
	combined := result.Combined
	points := func(values []float64) []Point {
		out := make([]Point, len(values))
		for i, value := range values {
			out[i] = Point{Time: combined.Times[i], Value: value}
		}
		return out
	}

	view := &PortfolioView{
		Name:         result.Spec.Name,
		Quote:        result.Spec.Quote,
		Initial:      combined.Initial,
		Final:        combined.Final,
		NetProfit:    combined.NetProfit,
		ReturnPct:    combined.ReturnPct,
		MaxDrawdown:  combined.MaxDrawdown,
		SharpeAnnual: combined.SharpeAnnual,
		Unallocated:  combined.Unallocated,
		Equity:       points(combined.Equity),
		Correlation:  combined.Correlation.Values,
		Strategies:   make([]PortfolioStrategy, 0, len(result.Strategies)),
	}
	for i, run := range result.Strategies {
		metrics := run.Result.Metrics
		strategy := PortfolioStrategy{
			ID:           run.ID,
			Strategy:     run.Result.Config.Strategy,
			Weight:       run.Weight,
			Capital:      run.Capital,
			Trades:       metrics.Trades,
			NetProfit:    metrics.NetProfit,
			ReturnPct:    metrics.ReturnPct,
			MaxDrawdown:  metrics.MaxDrawdown,
			SharpeAnnual: metrics.SharpeAnnual,
			Equity:       points(combined.ByStrategy[run.ID]),
		}
		if i < len(combined.Contribution) {
			strategy.PnLShare = combined.Contribution[i].PnLShare
			strategy.MarginalSharpe = combined.Contribution[i].MarginalSharpe
		}
		view.Strategies = append(view.Strategies, strategy)
	}
	return view
}

func (c *Chart) handlePortfolio(w http.ResponseWriter, _ *http.Request) {
	if c.portfolio == nil {
		http.Error(w, "no portfolio", http.StatusNotFound)
		return
	}
	writeJSON(w, c.portfolio)
}
