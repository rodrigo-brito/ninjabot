package backtest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"time"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/order"
	"github.com/rodrigo-brito/ninjabot/report"
)

// SchemaVersion is the version of the Result JSON layout. It changes when a
// field is renamed or removed; adding fields keeps it.
const SchemaVersion = 1

// minTradesWarning is the trade count below which a result warns that the
// statistics are noise.
const minTradesWarning = 20

// Point is a value at a Unix time, in seconds.
type Point = report.Point

// Period is the span of candles a run processed.
type Period struct {
	Start   time.Time `json:"start"`
	End     time.Time `json:"end"`
	Candles int       `json:"candles"`
}

// ConfigEcho records the settings of the run, so a result file describes how
// it was produced.
type ConfigEcho struct {
	Strategy  string             `json:"strategy"`
	Params    map[string]any     `json:"params,omitempty"`
	Pairs     []string           `json:"pairs"`
	Timeframe string             `json:"timeframe"`
	Quote     string             `json:"quote"`
	Balances  map[string]float64 `json:"balances"`
	MakerFee  float64            `json:"maker_fee"`
	TakerFee  float64            `json:"taker_fee"`
}

// Drawdown is the largest peak-to-trough decline of the equity, as a positive
// fraction of the peak, with the times of the peak and of the trough.
type Drawdown struct {
	Value float64   `json:"value"`
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// WalletSummary is the final state of the paper wallet, in quote currency.
type WalletSummary struct {
	Initial     float64            `json:"initial"`
	Final       float64            `json:"final"`
	NetProfit   float64            `json:"net_profit"`
	GrossProfit float64            `json:"gross_profit"`
	Fees        float64            `json:"fees"`
	ReturnPct   float64            `json:"return_pct"`
	BuyAndHold  float64            `json:"buy_and_hold"` // average of the pairs, as a fraction
	MaxDrawdown Drawdown           `json:"max_drawdown"`
	Volume      map[string]float64 `json:"volume"`
}

// Result is the outcome of a run. Its JSON form is the contract of the CLI
// and of everything built on top of it.
type Result struct {
	SchemaVersion int            `json:"schema_version"`
	Config        ConfigEcho     `json:"config"`
	Period        Period         `json:"period"`
	Wallet        WalletSummary  `json:"wallet"`
	Metrics       Metrics        `json:"metrics"`
	Trades        []order.Result `json:"trades"` // in the order they closed
	Orders        []model.Order  `json:"orders"` // by ID
	Equity        []Point        `json:"equity"` // one point per timestamp
	// Pairs holds the robustness report of each pair, on its own trades
	// and the wallet equity.
	Pairs map[string]report.Statistics `json:"pairs,omitempty"`
	// Portfolio is the report over every trade and the wallet equity.
	Portfolio *report.Statistics `json:"portfolio,omitempty"`
	Warnings  []string           `json:"warnings"`
}

func (e *Engine) result(first, last time.Time, candles int) (*Result, error) {
	orders, err := e.c.Orders.Orders()
	if err != nil {
		return nil, err
	}

	wallet := e.c.Wallet
	equity := wallet.EquityValues()
	points := make([]Point, len(equity))
	for i, value := range equity {
		points[i] = Point{Time: value.Time.Unix(), Value: value.Value}
	}
	drawdown, ddStart, ddEnd := wallet.MaxDrawdown()

	return assemble(resultInput{
		Config:       e.c.Config,
		Period:       Period{Start: first, End: last, Candles: candles},
		Trades:       e.c.Orders.Trades(),
		Orders:       orders,
		Equity:       points,
		Initial:      wallet.InitialValue(),
		Final:        wallet.Equity(),
		Fees:         wallet.FeesPaid(),
		Drawdown:     &Drawdown{Value: -drawdown, Start: ddStart, End: ddEnd},
		Volume:       wallet.Volume(),
		Streams:      e.c.Streams,
		NoStatistics: e.c.NoStatistics,
		Seed:         e.c.Seed,
	}), nil
}

// resultInput is what a result is assembled from.
type resultInput struct {
	Config       ConfigEcho
	Period       Period
	Trades       []order.Result
	Orders       []model.Order
	Equity       []Point
	Initial      float64
	Final        float64
	Fees         float64
	Drawdown     *Drawdown // computed from Equity when nil
	Volume       map[string]float64
	Streams      [][]model.Candle // candles of the traded pairs
	NoStatistics bool
	Seed         int64
}

func assemble(in resultInput) *Result {
	var drawdown Drawdown
	if in.Drawdown != nil {
		drawdown = *in.Drawdown
	} else {
		drawdown = drawdownOf(in.Equity)
	}
	if in.Trades == nil {
		in.Trades = []order.Result{}
	}
	if in.Orders == nil {
		in.Orders = []model.Order{}
	}
	if in.Volume == nil {
		in.Volume = map[string]float64{}
	}

	result := &Result{
		SchemaVersion: SchemaVersion,
		Config:        in.Config,
		Period:        in.Period,
		Wallet: WalletSummary{
			Initial:     in.Initial,
			Final:       in.Final,
			NetProfit:   in.Final - in.Initial,
			GrossProfit: in.Final - in.Initial + in.Fees,
			Fees:        in.Fees,
			BuyAndHold:  buyAndHold(in.Streams),
			MaxDrawdown: drawdown,
			Volume:      in.Volume,
		},
		Trades:   in.Trades,
		Orders:   in.Orders,
		Equity:   in.Equity,
		Warnings: []string{},
	}
	if in.Initial > 0 {
		result.Wallet.ReturnPct = (in.Final - in.Initial) / in.Initial
	}

	switch n := len(result.Trades); {
	case n == 0:
		result.Warnings = append(result.Warnings, "no trades: the strategy never closed a position")
	case n < minTradesWarning:
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("only %d trades: statistics are not significant below %d", n, minTradesWarning))
	}

	result.Metrics = computeMetrics(result)
	if !in.NoStatistics {
		statistics(result, in.Streams, in.Seed)
	}
	return result
}

// drawdownOf is the largest peak-to-trough decline of a curve, as a positive
// fraction, with the times of the peak and of the trough.
func drawdownOf(points []Point) Drawdown {
	var worst Drawdown
	peak := 0
	for i, point := range points {
		if point.Value > points[peak].Value {
			peak = i
		}
		if top := points[peak].Value; top > 0 {
			if value := (top - point.Value) / top; value > worst.Value {
				worst = Drawdown{Value: value, Start: time.Unix(points[peak].Time, 0).UTC(),
					End: time.Unix(point.Time, 0).UTC()}
			}
		}
	}
	return worst
}

// buyAndHold is the average return of holding every pair over the run.
func buyAndHold(streams [][]model.Candle) float64 {
	total, count := 0.0, 0
	for _, stream := range streams {
		closes := completeCloses(stream)
		if len(closes) < 2 || closes[0] <= 0 {
			continue
		}
		total += (closes[len(closes)-1] - closes[0]) / closes[0]
		count++
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}

func completeCloses(stream []model.Candle) []float64 {
	closes := make([]float64, 0, len(stream))
	for _, candle := range stream {
		if candle.Complete {
			closes = append(closes, candle.Close)
		}
	}
	return closes
}

// statistics fills the per pair and portfolio reports.
func statistics(result *Result, streams [][]model.Candle, seed int64) {
	quote := result.Config.Quote
	byPair := make(map[string][]report.Trade)
	all := make([]report.Trade, 0, len(result.Trades))
	for _, trade := range result.Trades {
		t := report.Trade{ID: trade.OrderID, Time: trade.ClosedAt.Unix(),
			Profit: trade.ProfitPercent, ProfitValue: trade.ProfitValue}
		byPair[trade.Pair] = append(byPair[trade.Pair], t)
		all = append(all, t)
	}

	result.Pairs = make(map[string]report.Statistics, len(streams))
	for _, stream := range streams {
		if len(stream) == 0 {
			continue
		}
		pair := stream[0].Pair
		candles := make([]report.Candle, 0, len(stream))
		for _, candle := range stream {
			if candle.Complete {
				candles = append(candles, report.Candle{Time: candle.Time.Unix(), Open: candle.Open,
					High: candle.High, Low: candle.Low, Close: candle.Close, Volume: candle.Volume})
			}
		}
		trades := byPair[pair]
		if trades == nil {
			trades = []report.Trade{}
		}
		result.Pairs[pair] = report.Build(report.Input{
			Pair: pair, Quote: quote, Trades: trades, Equity: result.Equity, Candles: candles, Seed: seed,
		})
	}

	portfolio := report.Build(report.Input{Pair: "portfolio", Quote: quote, Trades: all, Equity: result.Equity,
		Seed: seed})
	result.Portfolio = &portfolio
}

// WriteJSON writes the result as indented JSON.
func (r *Result) WriteJSON(w io.Writer) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(r)
}

// ReadResult decodes a result written by WriteJSON.
func ReadResult(r io.Reader) (*Result, error) {
	var result Result
	if err := json.NewDecoder(r).Decode(&result); err != nil {
		return nil, err
	}
	if result.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("backtest: unsupported result schema version %d", result.SchemaVersion)
	}
	return &result, nil
}

// WriteTradesCSV writes the closed trades, in the order they closed.
func (r *Result) WriteTradesCSV(w io.Writer) error {
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"closed_at", "pair", "side", "profit_percent", "profit_value",
		"duration_seconds", "created_at", "order_id"})
	for _, trade := range r.Trades {
		_ = writer.Write([]string{
			trade.ClosedAt.UTC().Format(time.RFC3339),
			trade.Pair,
			string(trade.Side),
			formatFloat(trade.ProfitPercent),
			formatFloat(trade.ProfitValue),
			strconv.FormatInt(int64(trade.Duration.Seconds()), 10),
			trade.CreatedAt.UTC().Format(time.RFC3339),
			strconv.FormatInt(trade.OrderID, 10),
		})
	}
	writer.Flush()
	return writer.Error()
}

// WriteEquityCSV writes the equity curve.
func (r *Result) WriteEquityCSV(w io.Writer) error {
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"time", "equity"})
	for _, point := range r.Equity {
		_ = writer.Write([]string{time.Unix(point.Time, 0).UTC().Format(time.RFC3339), formatFloat(point.Value)})
	}
	writer.Flush()
	return writer.Error()
}

// WriteOrdersCSV writes every order, by ID.
func (r *Result) WriteOrdersCSV(w io.Writer) error {
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"id", "created_at", "updated_at", "pair", "side", "type", "status",
		"quantity", "price", "fee", "profit_percent", "profit_value"})
	for _, o := range r.Orders {
		_ = writer.Write([]string{
			strconv.FormatInt(o.ID, 10),
			o.CreatedAt.UTC().Format(time.RFC3339),
			o.UpdatedAt.UTC().Format(time.RFC3339),
			o.Pair, string(o.Side), string(o.Type), string(o.Status),
			formatFloat(o.Quantity), formatFloat(o.Price), formatFloat(o.Fee),
			formatFloat(o.Profit), formatFloat(o.ProfitValue),
		})
	}
	writer.Flush()
	return writer.Error()
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// strategyName is the type name of a strategy, without the package path.
func strategyName(s any) string {
	t := reflect.TypeOf(s)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil {
		return ""
	}
	return t.Name()
}

func sortedPairs(pairs []string) []string {
	sorted := append([]string(nil), pairs...)
	sort.Strings(sorted)
	return sorted
}
