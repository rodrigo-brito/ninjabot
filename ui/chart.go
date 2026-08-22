// Package ui serves the ninjabot web dashboard: an HTTP API with the bot
// state (candles, indicators, orders, equity) plus a single-page React
// application that renders it.
//
// The dashboard bundle is not embedded in the Go binary. It is published as
// an asset of every GitHub release (ninjabot-ui.tar.gz) and downloaded on
// first use to the user cache directory. See bundle.go for the resolution
// rules and the NINJABOT_UI_DIR / NINJABOT_UI_VERSION overrides.
package ui

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/StudioSol/set"
	log "github.com/sirupsen/logrus"

	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/strategy"
)

// CustomIndicator is an indicator rendered by the dashboard in addition to
// the ones exposed by the strategy. See the indicator subpackage.
type CustomIndicator interface {
	Name() string
	Overlay() bool
	Warmup() int
	Metrics() []IndicatorMetric
	Load(dataframe *model.Dataframe)
}

// IndicatorMetric is one series produced by a CustomIndicator.
type IndicatorMetric struct {
	Name   string
	Color  string
	Style  string
	Values model.Series[float64]
	Time   []time.Time
}

// Chart collects candles and orders from the bot and serves the dashboard.
// It implements ninjabot.CandleSubscriber and ninjabot.OrderSubscriber.
type Chart struct {
	mu sync.Mutex

	port        int
	candles     map[string][]Candle
	dataframe   map[string]*model.Dataframe
	orderIDs    map[string]*set.LinkedHashSetINT64
	orderByID   map[int64]model.Order
	indicators  []CustomIndicator
	paperWallet *exchange.PaperWallet
	strategy    strategy.Strategy
	controller  OrderController
	lastUpdate  time.Time

	events *broker
	bundle bundleConfig
}

// Option configures a Chart.
type Option func(*Chart)

// WithPort sets the HTTP port (default 8080).
func WithPort(port int) Option {
	return func(c *Chart) { c.port = port }
}

// WithStrategyIndicators renders the indicators returned by
// strategy.Indicators on the dashboard.
func WithStrategyIndicators(s strategy.Strategy) Option {
	return func(c *Chart) { c.strategy = s }
}

// WithPaperWallet enables the equity / asset value charts and drawdown.
func WithPaperWallet(wallet *exchange.PaperWallet) Option {
	return func(c *Chart) { c.paperWallet = wallet }
}

// WithCustomIndicators adds indicators that are not part of the strategy.
func WithCustomIndicators(indicators ...CustomIndicator) Option {
	return func(c *Chart) { c.indicators = indicators }
}

// WithUIDir serves the dashboard from a local directory (the output of
// `bun run build` in web/) instead of downloading the release bundle.
// The NINJABOT_UI_DIR environment variable has the same effect.
func WithUIDir(dir string) Option {
	return func(c *Chart) { c.bundle.dir = dir }
}

// WithUIVersion forces the release tag of the bundle to download
// (e.g. "v1.4.0" or "latest"). The NINJABOT_UI_VERSION environment variable
// has the same effect. By default the version of the ninjabot module compiled
// into the binary is used.
func WithUIVersion(version string) Option {
	return func(c *Chart) { c.bundle.version = version }
}

// WithCacheDir overrides where downloaded bundles are stored
// (default: os.UserCacheDir()/ninjabot/ui).
func WithCacheDir(dir string) Option {
	return func(c *Chart) { c.bundle.cacheDir = dir }
}

// WithHTTPClient sets the client used to download the bundle.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Chart) { c.bundle.client = client }
}

// withReleaseURL points the downloader to another host (tests).
func withReleaseURL(url string) Option {
	return func(c *Chart) { c.bundle.releaseURL = url }
}

// New creates a dashboard. Call Start to serve it.
func New(options ...Option) (*Chart, error) {
	c := &Chart{
		port:      8080,
		candles:   make(map[string][]Candle),
		dataframe: make(map[string]*model.Dataframe),
		orderIDs:  make(map[string]*set.LinkedHashSetINT64),
		orderByID: make(map[int64]model.Order),
		events:    newBroker(),
		bundle:    defaultBundleConfig(),
	}

	for _, option := range options {
		option(c)
	}

	return c, nil
}

// OnOrder stores an order and notifies connected dashboards.
func (c *Chart) OnOrder(order model.Order) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.orderIDs[order.Pair] == nil {
		c.orderIDs[order.Pair] = set.NewLinkedHashSetINT64()
	}
	c.orderIDs[order.Pair].Add(order.ID)
	c.orderByID[order.ID] = order

	c.events.publish("order", OrderEvent{
		Pair:  order.Pair,
		Order: c.toOrder(order, c.candles[order.Pair]),
	})
}

// OnCandle stores a closed candle and notifies connected dashboards.
func (c *Chart) OnCandle(candle model.Candle) {
	if !candle.Complete {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	candles := c.candles[candle.Pair]
	if len(candles) > 0 && candle.Time.Unix() <= candles[len(candles)-1].Time {
		return
	}

	dto := Candle{
		Time:   candle.Time.Unix(),
		Open:   candle.Open,
		High:   candle.High,
		Low:    candle.Low,
		Close:  candle.Close,
		Volume: candle.Volume,
	}
	c.candles[candle.Pair] = append(candles, dto)

	df := c.dataframe[candle.Pair]
	if df == nil {
		df = &model.Dataframe{
			Pair:     candle.Pair,
			Metadata: make(map[string]model.Series[float64]),
		}
		c.dataframe[candle.Pair] = df
	}
	if c.orderIDs[candle.Pair] == nil {
		c.orderIDs[candle.Pair] = set.NewLinkedHashSetINT64()
	}

	df.Close = append(df.Close, candle.Close)
	df.Open = append(df.Open, candle.Open)
	df.High = append(df.High, candle.High)
	df.Low = append(df.Low, candle.Low)
	df.Volume = append(df.Volume, candle.Volume)
	df.Time = append(df.Time, candle.Time)
	df.LastUpdate = candle.Time
	for k, v := range candle.Metadata {
		df.Metadata[k] = append(df.Metadata[k], v)
	}
	c.lastUpdate = time.Now()

	if c.events.hasSubscribers() {
		c.events.publish("candle", c.candleEvent(candle.Pair, dto))
	}
}

func (c *Chart) candleEvent(pair string, candle Candle) CandleEvent {
	event := CandleEvent{
		Pair:       pair,
		Candle:     candle,
		Indicators: make([]IndicatorUpdate, 0),
	}

	for _, indicator := range c.indicatorsByPair(pair) {
		update := IndicatorUpdate{Name: indicator.Name, Metrics: make([]MetricUpdate, 0, len(indicator.Metrics))}
		for _, metric := range indicator.Metrics {
			if len(metric.Points) == 0 {
				continue
			}
			update.Metrics = append(update.Metrics, MetricUpdate{
				Name:  metric.Name,
				Point: metric.Points[len(metric.Points)-1],
			})
		}
		event.Indicators = append(event.Indicators, update)
	}

	assetValues, equityValues := c.equityValuesByPair(pair)
	if len(equityValues) > 0 {
		last := equityValues[len(equityValues)-1]
		event.Equity = &last
	}
	if len(assetValues) > 0 {
		last := assetValues[len(assetValues)-1]
		event.Asset = &last
	}

	return event
}

func (c *Chart) pairs() []string {
	pairs := make([]string, 0, len(c.candles))
	for pair := range c.candles {
		pairs = append(pairs, pair)
	}
	sort.Strings(pairs)
	return pairs
}

func (c *Chart) snapshot(pair string) Snapshot {
	asset, quote := exchange.SplitAssetQuote(pair)
	assetValues, equityValues := c.equityValuesByPair(pair)

	candles := make([]Candle, len(c.candles[pair]))
	copy(candles, c.candles[pair])

	return Snapshot{
		Pair:         pair,
		Asset:        asset,
		Quote:        quote,
		Candles:      candles,
		Indicators:   c.indicatorsByPair(pair),
		Orders:       c.ordersByPair(pair),
		EquityValues: equityValues,
		AssetValues:  assetValues,
		MaxDrawdown:  c.maxDrawdown(),
	}
}

func (c *Chart) maxDrawdown() *Drawdown {
	if c.paperWallet == nil {
		return nil
	}
	value, start, end := c.paperWallet.MaxDrawdown()
	if start.IsZero() {
		return nil
	}
	return &Drawdown{Value: value, Start: start.Unix(), End: end.Unix()}
}

func (c *Chart) equityValuesByPair(pair string) (asset []Point, equity []Point) {
	asset = make([]Point, 0)
	equity = make([]Point, 0)

	if c.paperWallet == nil {
		return asset, equity
	}

	assetName, _ := exchange.SplitAssetQuote(pair)
	for _, value := range c.paperWallet.AssetValues(assetName) {
		asset = append(asset, Point{Time: value.Time.Unix(), Value: value.Value})
	}
	for _, value := range c.paperWallet.EquityValues() {
		equity = append(equity, Point{Time: value.Time.Unix(), Value: value.Value})
	}

	return asset, equity
}

func toPoints(times []time.Time, values []float64) []Point {
	n := len(values)
	if len(times) < n {
		n = len(times)
	}
	points := make([]Point, 0, n)
	for i := 0; i < n; i++ {
		points = append(points, Point{Time: times[i].Unix(), Value: values[i]})
	}
	return points
}

func (c *Chart) indicatorsByPair(pair string) []Indicator {
	indicators := make([]Indicator, 0)
	df := c.dataframe[pair]
	if df == nil {
		return indicators
	}

	for _, i := range c.indicators {
		i.Load(df)
		indicator := Indicator{
			Name:    i.Name(),
			Overlay: i.Overlay(),
			Metrics: make([]Metric, 0),
		}
		for _, metric := range i.Metrics() {
			indicator.Metrics = append(indicator.Metrics, Metric{
				Name:   metric.Name,
				Color:  metric.Color,
				Style:  metric.Style,
				Points: toPoints(metric.Time, metric.Values),
			})
		}
		indicators = append(indicators, indicator)
	}

	if c.strategy != nil {
		warmup := c.strategy.WarmupPeriod()
		for _, i := range c.strategy.Indicators(df) {
			indicator := Indicator{
				Name:    i.GroupName,
				Overlay: i.Overlay,
				Metrics: make([]Metric, 0),
			}
			// Values before the warmup period are not meaningful (usually zero)
			// and would distort the price scale, so they are dropped.
			start := i.Warmup
			if warmup > start {
				start = warmup
			}
			for _, metric := range i.Metrics {
				if len(metric.Values) < start || len(i.Time) < start {
					continue
				}
				indicator.Metrics = append(indicator.Metrics, Metric{
					Name:   metric.Name,
					Color:  metric.Color,
					Style:  string(metric.Style),
					Points: toPoints(i.Time[start:], metric.Values[start:]),
				})
			}
			indicators = append(indicators, indicator)
		}
	}

	return indicators
}

// candleTimeFor returns the open time of the candle that contains t, or the
// last candle when t is after the last known candle. Zero when no candles.
func candleTimeFor(candles []Candle, t time.Time) int64 {
	if len(candles) == 0 {
		return 0
	}
	ts := t.Unix()
	idx := sort.Search(len(candles), func(i int) bool { return candles[i].Time > ts })
	if idx == 0 {
		return candles[0].Time
	}
	return candles[idx-1].Time
}

func (c *Chart) toOrder(o model.Order, candles []Candle) Order {
	return Order{
		ID:          o.ID,
		ExchangeID:  o.ExchangeID,
		Pair:        o.Pair,
		Side:        string(o.Side),
		Type:        string(o.Type),
		Status:      string(o.Status),
		Price:       o.Price,
		Quantity:    o.Quantity,
		CreatedAt:   o.CreatedAt.Unix(),
		UpdatedAt:   o.UpdatedAt.Unix(),
		CandleTime:  candleTimeFor(candles, o.UpdatedAt),
		Stop:        o.Stop,
		GroupID:     o.GroupID,
		RefPrice:    o.RefPrice,
		Profit:      o.Profit,
		ProfitValue: o.ProfitValue,
	}
}

func (c *Chart) ordersByPair(pair string) []Order {
	orders := make([]Order, 0)
	ids := c.orderIDs[pair]
	if ids == nil {
		return orders
	}
	candles := c.candles[pair]
	for id := range ids.Iter() {
		orders = append(orders, c.toOrder(c.orderByID[id], candles))
	}
	return orders
}

func (c *Chart) orderRowsByPair(pair string) [][]string {
	rows := make([][]string, 0)
	ids := c.orderIDs[pair]
	if ids == nil {
		return rows
	}
	for id := range ids.Iter() {
		o := c.orderByID[id]
		var profit string
		if o.Profit != 0 {
			profit = fmt.Sprintf("%.2f", o.Profit)
		}
		rows = append(rows, []string{
			o.CreatedAt.Format(time.RFC3339),
			string(o.Status),
			string(o.Side),
			fmt.Sprintf("%d", o.ID),
			string(o.Type),
			fmt.Sprintf("%f", o.Quantity),
			fmt.Sprintf("%f", o.Price),
			fmt.Sprintf("%.2f", o.Quantity*o.Price),
			profit,
		})
	}
	return rows
}

// Start resolves the dashboard bundle and serves it with the API on the
// configured port. It blocks until the server stops.
func (c *Chart) Start() error {
	handler := c.Handler(context.Background())
	fmt.Printf("Dashboard available at http://localhost:%d\n", c.port)
	return http.ListenAndServe(fmt.Sprintf(":%d", c.port), handler)
}

// Handler resolves the dashboard bundle and returns the HTTP handler (API +
// static files), for users who want to mount the dashboard in their own
// server. When the bundle cannot be resolved the API still works and the
// index page explains the failure.
func (c *Chart) Handler(ctx context.Context) http.Handler {
	bundle, err := resolveBundle(ctx, c.bundle)
	if err != nil {
		log.Warnf("ui: dashboard bundle unavailable, serving API only: %v", err)
	}
	return c.router(bundle, err)
}
