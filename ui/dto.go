package ui

// This file holds every type exposed by the HTTP API. It is the single source
// of truth for the TypeScript types consumed by the web dashboard:
// `make generate` runs tygo on this file and writes web/src/api/types.ts.
//
// All timestamps are Unix seconds (UTC), which is the native time format of
// lightweight-charts.

//go:generate go run github.com/gzuidhof/tygo@v0.2.21 generate

// Candle is an OHLCV bar.
type Candle struct {
	Time   int64   `json:"time"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
}

// Point is a single time/value sample of a series.
type Point struct {
	Time  int64   `json:"time"`
	Value float64 `json:"value"`
}

// Metric is one plotted series of an indicator (e.g. the signal line of MACD).
// Style is one of: line, scatter, bar, histogram, waterfall.
type Metric struct {
	Name   string  `json:"name"`
	Color  string  `json:"color"`
	Style  string  `json:"style"`
	Points []Point `json:"points"`
}

// Indicator groups the metrics of one indicator. Overlay indicators are drawn
// on top of the price chart; the others get their own pane.
type Indicator struct {
	Name    string   `json:"name"`
	Overlay bool     `json:"overlay"`
	Metrics []Metric `json:"metrics"`
}

// Order is the API projection of model.Order. CandleTime is the open time of
// the candle that contains the order, so the UI can anchor markers to a bar.
type Order struct {
	ID          int64    `json:"id"`
	ExchangeID  int64    `json:"exchange_id"`
	Pair        string   `json:"pair"`
	Side        string   `json:"side"`
	Type        string   `json:"type"`
	Status      string   `json:"status"`
	Price       float64  `json:"price"`
	Quantity    float64  `json:"quantity"`
	Fee         float64  `json:"fee"`
	CreatedAt   int64    `json:"created_at"`
	UpdatedAt   int64    `json:"updated_at"`
	CandleTime  int64    `json:"candle_time"`
	Stop        *float64 `json:"stop"`
	GroupID     *int64   `json:"group_id"`
	RefPrice    float64  `json:"ref_price"`
	Profit      float64  `json:"profit"`
	ProfitValue float64  `json:"profit_value"`
}

// Drawdown is the maximum drawdown window of the paper wallet equity.
type Drawdown struct {
	Value float64 `json:"value"`
	Start int64   `json:"start"`
	End   int64   `json:"end"`
}

// Snapshot is everything the dashboard needs to render one pair.
type Snapshot struct {
	Pair         string      `json:"pair"`
	Asset        string      `json:"asset"`
	Quote        string      `json:"quote"`
	Candles      []Candle    `json:"candles"`
	Indicators   []Indicator `json:"indicators"`
	Orders       []Order     `json:"orders"`
	EquityValues []Point     `json:"equity_values"`
	AssetValues  []Point     `json:"asset_values"`
	MaxDrawdown  *Drawdown   `json:"max_drawdown"`
}

// PairsResponse lists the pairs known by the dashboard.
type PairsResponse struct {
	Pairs []string `json:"pairs"`
}

// MetricUpdate carries the latest point of one indicator metric.
type MetricUpdate struct {
	Name  string `json:"name"`
	Point Point  `json:"point"`
}

// IndicatorUpdate carries the latest point of each metric of an indicator.
type IndicatorUpdate struct {
	Name    string         `json:"name"`
	Metrics []MetricUpdate `json:"metrics"`
}

// CandleEvent is sent on the SSE stream (event: candle) when a candle closes.
type CandleEvent struct {
	Pair       string            `json:"pair"`
	Candle     Candle            `json:"candle"`
	Indicators []IndicatorUpdate `json:"indicators"`
	Equity     *Point            `json:"equity"`
	Asset      *Point            `json:"asset"`
}

// OrderEvent is sent on the SSE stream (event: order) when an order is
// created or updated.
type OrderEvent struct {
	Pair  string `json:"pair"`
	Order Order  `json:"order"`
}

// ControlsResponse reports whether the bot control panel is enabled and the
// controller status ("running" or "stopped"). It is also the payload of the
// SSE "controls" event, sent when the bot is started or stopped from the
// dashboard.
type ControlsResponse struct {
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"`
}

// ControlOrderRequest is the payload of POST /api/controls/order. Amount is
// in quote currency; with Percent it is a percentage of the available balance
// (quote balance for buys, asset balance for sells), mirroring the Telegram
// /buy and /sell commands.
type ControlOrderRequest struct {
	Pair    string  `json:"pair"`
	Side    string  `json:"side"`
	Amount  float64 `json:"amount"`
	Percent bool    `json:"percent"`
}

// ErrorResponse is the JSON body of API error replies.
type ErrorResponse struct {
	Error string `json:"error"`
}
