package ui

import (
	"sort"

	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/report"
)

// This file turns a finished backtest into the robustness report served at
// GET /api/{pair}/statistics. The dashboard used to show only what the equity
// curve says; the report adds what it cannot say on its own -- whether the
// result survives resampling, whether it holds over time, and how much room it
// has before trading costs eat it.

// statisticsInput is everything the report is computed from. The handler
// copies it under the chart lock and runs the resampling outside of it, which
// keeps a few thousand simulations from holding back the candle stream.
type statisticsInput struct {
	Pair    string
	Quote   string
	Orders  []Order
	Equity  []Point
	Candles []Candle
}

// statisticsInput collects the sample of a pair. The caller holds the lock.
func (c *Chart) statisticsInput(pair string) statisticsInput {
	_, quote := exchange.SplitAssetQuote(pair)
	_, equity := c.equityValuesByPair(pair)

	candles := make([]Candle, len(c.candles[pair]))
	copy(candles, c.candles[pair])

	return statisticsInput{
		Pair:    pair,
		Quote:   quote,
		Orders:  c.ordersByPair(pair),
		Equity:  equity,
		Candles: candles,
	}
}

// buildStatistics turns the dashboard view of a pair into the report input.
func buildStatistics(in statisticsInput) Statistics {
	orders := closedTrades(in.Orders)
	trades := make([]report.Trade, len(orders))
	for i, order := range orders {
		trades[i] = report.Trade{ID: order.ID, Time: order.UpdatedAt, Profit: order.Profit, ProfitValue: order.ProfitValue}
	}

	equity := make([]report.Point, len(in.Equity))
	for i, point := range in.Equity {
		equity[i] = report.Point(point)
	}
	candles := make([]report.Candle, len(in.Candles))
	for i, candle := range in.Candles {
		candles[i] = report.Candle(candle)
	}

	return report.Build(report.Input{
		Pair:    in.Pair,
		Quote:   in.Quote,
		Trades:  trades,
		Equity:  equity,
		Candles: candles,
	})
}

// closedTrades keeps the orders that carry the result of a round trip, in the
// order they were closed: the sequence is what the runs and stability tests
// read, so it has to be chronological.
func closedTrades(orders []Order) []Order {
	trades := make([]Order, 0, len(orders))
	for _, order := range orders {
		if order.Status == string(model.OrderStatusTypeFilled) &&
			(order.Profit != 0 || order.ProfitValue != 0) {
			trades = append(trades, order)
		}
	}

	sort.SliceStable(trades, func(i, j int) bool {
		if trades[i].UpdatedAt != trades[j].UpdatedAt {
			return trades[i].UpdatedAt < trades[j].UpdatedAt
		}
		return trades[i].ID < trades[j].ID
	})

	return trades
}
