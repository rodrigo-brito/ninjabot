package plot

import (
	"bytes"
	"embed"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/strategy"

	"github.com/StudioSol/set"
	"github.com/evanw/esbuild/pkg/api"
	log "github.com/sirupsen/logrus"
)

var (
	//go:embed assets
	staticFiles embed.FS
)

type Chart struct {
	sync.Mutex
	port          int
	debug         bool
	candles       map[string][]Candle
	dataframe     map[string]*model.Dataframe
	ordersByPair  map[string]*set.LinkedHashSetINT64
	orderByID     map[int64]model.Order
	indicators    []Indicator
	paperWallet   *exchange.PaperWallet
	scriptContent string
	indexHTML     *template.Template
	strategy      strategy.Strategy
}

type Candle struct {
	Time   time.Time     `json:"time"`
	Open   float64       `json:"open"`
	Close  float64       `json:"close"`
	High   float64       `json:"high"`
	Low    float64       `json:"low"`
	Volume float64       `json:"volume"`
	Orders []model.Order `json:"orders"`
}

type Shape struct {
	StartX time.Time `json:"x0"`
	EndX   time.Time `json:"x1"`
	StartY float64   `json:"y0"`
	EndY   float64   `json:"y1"`
	Color  string    `json:"color"`
}

type assetValue struct {
	Time  time.Time `json:"time"`
	Value float64   `json:"value"`
}

type indicatorMetric struct {
	Name   string      `json:"name"`
	Time   []time.Time `json:"time"`
	Values []float64   `json:"value"`
	Color  string      `json:"color"`
	Style  string      `json:"style"`
}

type plotIndicator struct {
	Name    string            `json:"name"`
	Overlay bool              `json:"overlay"`
	Metrics []indicatorMetric `json:"metrics"`
}

type drawdown struct {
	Value string    `json:"value"`
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type Indicator interface {
	Name() string
	Overlay() bool
	Metrics() []IndicatorMetric
	Load(dataframe *model.Dataframe)
}

type IndicatorMetric struct {
	Name   string
	Color  string
	Style  string
	Values model.Series
	Time   []time.Time
}

func (c *Chart) OnOrder(order model.Order) {
	c.Lock()
	defer c.Unlock()

	c.ordersByPair[order.Pair].Add(order.ID)
	c.orderByID[order.ID] = order

}

func (c *Chart) OnCandle(candle model.Candle) {
	c.Lock()
	defer c.Unlock()

	if candle.Complete && (len(c.candles[candle.Pair]) == 0 ||
		candle.Time.After(c.candles[candle.Pair][len(c.candles[candle.Pair])-1].Time)) {

		c.candles[candle.Pair] = append(c.candles[candle.Pair], Candle{
			Time:   candle.Time,
			Open:   candle.Open,
			Close:  candle.Close,
			High:   candle.High,
			Low:    candle.Low,
			Volume: candle.Volume,
			Orders: make([]model.Order, 0),
		})

		if c.dataframe[candle.Pair] == nil {
			c.dataframe[candle.Pair] = &model.Dataframe{
				Pair:     candle.Pair,
				Metadata: make(map[string]model.Series),
			}
			c.ordersByPair[candle.Pair] = set.NewLinkedHashSetINT64()
		}

		c.dataframe[candle.Pair].Close = append(c.dataframe[candle.Pair].Close, candle.Close)
		c.dataframe[candle.Pair].Open = append(c.dataframe[candle.Pair].Open, candle.Open)
		c.dataframe[candle.Pair].High = append(c.dataframe[candle.Pair].High, candle.High)
		c.dataframe[candle.Pair].Low = append(c.dataframe[candle.Pair].Low, candle.Low)
		c.dataframe[candle.Pair].Volume = append(c.dataframe[candle.Pair].Volume, candle.Volume)
		c.dataframe[candle.Pair].Time = append(c.dataframe[candle.Pair].Time, candle.Time)
		c.dataframe[candle.Pair].LastUpdate = candle.Time
		for k, v := range candle.Metadata {
			c.dataframe[candle.Pair].Metadata[k] = append(c.dataframe[candle.Pair].Metadata[k], v)
		}
	}
}

func (c *Chart) equityValuesByPair(pair string) (asset []assetValue, quote []assetValue) {
	assetValues := make([]assetValue, 0)
	equityValues := make([]assetValue, 0)

	if c.paperWallet != nil {
		asset, _ := exchange.SplitAssetQuote(pair)
		for _, value := range c.paperWallet.AssetValues(asset) {
			assetValues = append(assetValues, assetValue{
				Time:  value.Time,
				Value: value.Value,
			})
		}

		for _, value := range c.paperWallet.EquityValues() {
			equityValues = append(equityValues, assetValue{
				Time:  value.Time,
				Value: value.Value,
			})
		}
	}

	return assetValues, equityValues
}

func (c *Chart) indicatorsByPair(pair string) []plotIndicator {
	indicators := make([]plotIndicator, 0)
	for _, i := range c.indicators {
		i.Load(c.dataframe[pair])
		indicator := plotIndicator{
			Name:    i.Name(),
			Overlay: i.Overlay(),
			Metrics: make([]indicatorMetric, 0),
		}

		for _, metric := range i.Metrics() {
			indicator.Metrics = append(indicator.Metrics, indicatorMetric{
				Name:   metric.Name,
				Values: metric.Values,
				Time:   metric.Time,
				Color:  metric.Color,
				Style:  metric.Style,
			})
		}

		indicators = append(indicators, indicator)
	}

	if c.strategy != nil {
		warmup := c.strategy.WarmupPeriod()
		strategyIndicators := c.strategy.Indicators(c.dataframe[pair])
		for _, i := range strategyIndicators {
			indicator := plotIndicator{
				Name:    i.GroupName,
				Overlay: i.Overlay,
				Metrics: make([]indicatorMetric, 0),
			}

			for _, metric := range i.Metrics {
				if len(metric.Values) < warmup {
					continue
				}

				indicator.Metrics = append(indicator.Metrics, indicatorMetric{
					Time:   i.Time[warmup:],
					Values: metric.Values[warmup:],
					Name:   metric.Name,
					Color:  metric.Color,
					Style:  string(metric.Style),
				})
			}
			indicators = append(indicators, indicator)
		}
	}

	return indicators
}

func (c *Chart) candlesByPair(pair string) []Candle {
	candles := make([]Candle, len(c.candles[pair]))
	for i := range c.candles[pair] {
		candles[i] = c.candles[pair][i]
		for id := range c.ordersByPair[pair].Iter() {
			order := c.orderByID[id]

			if i < len(c.candles[pair])-1 &&
				(order.UpdatedAt.After(c.candles[pair][i].Time) &&
					order.UpdatedAt.Before(c.candles[pair][i+1].Time)) ||
				order.UpdatedAt.Equal(c.candles[pair][i].Time) {
				candles[i].Orders = append(candles[i].Orders, order)
			}
		}
	}

	return candles
}

func (c *Chart) shapesByPair(pair string) []Shape {
	shapes := make([]Shape, 0)
	for id := range c.ordersByPair[pair].Iter() {
		order := c.orderByID[id]

		if order.Type != model.OrderTypeStopLoss &&
			order.Type != model.OrderTypeLimitMaker {
			continue
		}

		shape := Shape{
			StartX: order.CreatedAt,
			EndX:   order.UpdatedAt,
			StartY: order.RefPrice,
			EndY:   order.Price,
			Color:  "rgba(0, 255, 0, 0.3)",
		}

		if order.Type == model.OrderTypeStopLoss {
			shape.Color = "rgba(255, 0, 0, 0.3)"
		}

		shapes = append(shapes, shape)
	}

	return shapes
}

func (c *Chart) orderStringByPair(pair string) [][]string {
	orders := make([][]string, 0)
	for id := range c.ordersByPair[pair].Iter() {
		o := c.orderByID[id]
		orderString := fmt.Sprintf("%s,%s,%d,%s,%f,%f,%.2f,%s",
			o.Status, o.Side, o.ID, o.Type, o.Quantity, o.Price, o.Quantity*o.Price, o.CreatedAt)
		order := strings.Split(orderString, ",")
		orders = append(orders, order)
	}
	return orders
}

func (c *Chart) handleIndex(w http.ResponseWriter, r *http.Request) {
	var pairs = make([]string, 0, len(c.candles))
	for pair := range c.candles {
		pairs = append(pairs, pair)
	}

	sort.Strings(pairs)
	pair := r.URL.Query().Get("pair")
	if pair == "" && len(pairs) > 0 {
		http.Redirect(w, r, fmt.Sprintf("/?pair=%s", pairs[0]), http.StatusFound)
		return
	}

	w.Header().Add("Content-Type", "text/html")
	err := c.indexHTML.Execute(w, map[string]interface{}{
		"pair":  pair,
		"pairs": pairs,
	})
	if err != nil {
		log.Error(err)
	}
}

func (c *Chart) handleData(w http.ResponseWriter, r *http.Request) {
	pair := r.URL.Query().Get("pair")
	if pair == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-type", "text/json")

	var maxDrawdown *drawdown
	if c.paperWallet != nil {
		value, start, end := c.paperWallet.MaxDrawdown()
		maxDrawdown = &drawdown{
			Start: start,
			End:   end,
			Value: fmt.Sprintf("%.1f", value*100),
		}
	}

	asset, quote := exchange.SplitAssetQuote(pair)
	assetValues, equityValues := c.equityValuesByPair(pair)
	err := json.NewEncoder(w).Encode(map[string]interface{}{
		"candles":       c.candlesByPair(pair),
		"indicators":    c.indicatorsByPair(pair),
		"shapes":        c.shapesByPair(pair),
		"asset_values":  assetValues,
		"equity_values": equityValues,
		"quote":         quote,
		"asset":         asset,
		"max_drawdown":  maxDrawdown,
	})
	if err != nil {
		log.Error(err)
	}
}

func (c *Chart) handleTradingHistoryData(w http.ResponseWriter, r *http.Request) {
	pair := r.URL.Query().Get("pair")
	if pair == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment;filename=history_"+pair+".csv")
	w.Header().Set("Transfer-Encoding", "chunked")

	orders := c.orderStringByPair(pair)

	buffer := bytes.NewBuffer(nil)
	csvWriter := csv.NewWriter(buffer)
	err := csvWriter.Write([]string{"status", "side", "id", "type", "quantity", "price", "total", "created_at"})
	if err != nil {
		log.Errorf("failed writing header file: %s", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	err = csvWriter.WriteAll(orders)
	if err != nil {
		log.Errorf("failed writing data: %s", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	csvWriter.Flush()

	w.WriteHeader(http.StatusOK)
	_, err = w.Write(buffer.Bytes())
	if err != nil {
		log.Errorf("failed writing response: %s", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}
}

func (c *Chart) Start() error {
	http.Handle(
		"/assets/",
		http.FileServer(http.FS(staticFiles)),
	)

	http.HandleFunc("/assets/chart.js", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-type", "application/javascript")
		fmt.Fprint(w, c.scriptContent)
	})

	http.HandleFunc("/history", c.handleTradingHistoryData)
	http.HandleFunc("/data", c.handleData)
	http.HandleFunc("/", c.handleIndex)

	fmt.Printf("Chart available at http://localhost:%d\n", c.port)
	return http.ListenAndServe(fmt.Sprintf(":%d", c.port), nil)
}

type Option func(*Chart)

func WithPort(port int) Option {
	return func(chart *Chart) {
		chart.port = port
	}
}

func WithStrategyIndicators(strategy strategy.Strategy) Option {
	return func(chart *Chart) {
		chart.strategy = strategy
	}
}

func WithPaperWallet(paperWallet *exchange.PaperWallet) Option {
	return func(chart *Chart) {
		chart.paperWallet = paperWallet
	}
}

// WithDebug starts chart without compress
func WithDebug() Option {
	return func(chart *Chart) {
		chart.debug = true
	}
}

func WithCustomIndicators(indicators ...Indicator) Option {
	return func(chart *Chart) {
		chart.indicators = indicators
	}
}

func NewChart(options ...Option) (*Chart, error) {
	chart := &Chart{
		port:         8080,
		candles:      make(map[string][]Candle),
		dataframe:    make(map[string]*model.Dataframe),
		ordersByPair: make(map[string]*set.LinkedHashSetINT64),
		orderByID:    make(map[int64]model.Order),
	}

	for _, option := range options {
		option(chart)
	}

	chartJS, err := staticFiles.ReadFile("assets/chart.js")
	if err != nil {
		return nil, err
	}

	chart.indexHTML, err = template.ParseFS(staticFiles, "assets/chart.html")
	if err != nil {
		return nil, err
	}

	transpileChartJS := api.Transform(string(chartJS), api.TransformOptions{
		Loader:            api.LoaderJS,
		Target:            api.ES2015,
		MinifySyntax:      !chart.debug,
		MinifyIdentifiers: !chart.debug,
		MinifyWhitespace:  !chart.debug,
	})

	if len(transpileChartJS.Errors) > 0 {
		return nil, fmt.Errorf("chart script faild with: %v", transpileChartJS.Errors)
	}

	chart.scriptContent = string(transpileChartJS.Code)

	return chart, nil
}
