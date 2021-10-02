package plot

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sync"
	"time"

	"github.com/rodrigo-brito/ninjabot/model"
	log "github.com/sirupsen/logrus"
)

//go:embed assets
var staticFiles embed.FS

type Chart struct {
	sync.Mutex
	port       int
	candles    map[string][]Candle
	dataframe  map[string]*model.Dataframe
	orders     map[string][]*Order
	indicators []Indicator
}

type Candle struct {
	Time   time.Time `json:"time"`
	Open   float64   `json:"open"`
	Close  float64   `json:"close"`
	High   float64   `json:"high"`
	Low    float64   `json:"low"`
	Volume float64   `json:"volume"`
	Orders []Order   `json:"orders"`
}

type Order struct {
	ID       int64     `json:"id"`
	Time     time.Time `json:"time"`
	Price    float64   `json:"price"`
	Quantity float64   `json:"quantity"`
	Type     string    `json:"type"`
	Side     string    `json:"side"`
	Profit   float64   `json:"profit"`
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

func (c *Chart) OnOrder(order model.Order) {
	c.Lock()
	defer c.Unlock()

	if order.Status == model.OrderStatusTypeFilled {
		item := &Order{
			ID:       order.ID,
			Time:     order.UpdatedAt,
			Price:    order.Price,
			Quantity: order.Quantity,
			Type:     string(order.Type),
			Side:     string(order.Side),
			Profit:   order.Profit,
		}

		if order.Type == model.OrderTypeStopLoss || order.Type == model.OrderTypeStopLossLimit {
			item.Price = *order.Stop
		}

		c.orders[order.Symbol] = append(c.orders[order.Symbol], item)
	}
}

func (c *Chart) OnCandle(candle model.Candle) {
	c.Lock()
	defer c.Unlock()

	if candle.Complete && (len(c.candles[candle.Symbol]) == 0 ||
		candle.Time.After(c.candles[candle.Symbol][len(c.candles[candle.Symbol])-1].Time)) {

		c.candles[candle.Symbol] = append(c.candles[candle.Symbol], Candle{
			Time:   candle.Time,
			Open:   candle.Open,
			Close:  candle.Close,
			High:   candle.High,
			Low:    candle.Low,
			Volume: candle.Volume,
			Orders: make([]Order, 0),
		})

		if c.dataframe[candle.Symbol] == nil {
			c.dataframe[candle.Symbol] = &model.Dataframe{
				Pair:     candle.Symbol,
				Metadata: make(map[string]model.Series),
			}
		}

		c.dataframe[candle.Symbol].Close = append(c.dataframe[candle.Symbol].Close, candle.Close)
		c.dataframe[candle.Symbol].Open = append(c.dataframe[candle.Symbol].Open, candle.Open)
		c.dataframe[candle.Symbol].High = append(c.dataframe[candle.Symbol].High, candle.High)
		c.dataframe[candle.Symbol].Low = append(c.dataframe[candle.Symbol].Low, candle.Low)
		c.dataframe[candle.Symbol].Volume = append(c.dataframe[candle.Symbol].Volume, candle.Volume)
		c.dataframe[candle.Symbol].Time = append(c.dataframe[candle.Symbol].Time, candle.Time)
		c.dataframe[candle.Symbol].LastUpdate = candle.Time
	}
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
	return indicators
}

func (c *Chart) candlesByPair(pair string) []Candle {
	for i := range c.candles[pair] {
		for j, order := range c.orders[pair] {
			if order == nil {
				continue
			}

			if i < len(c.candles[pair])-1 &&
				(order.Time.After(c.candles[pair][i].Time) &&
					order.Time.Before(c.candles[pair][i+1].Time)) ||
				order.Time.Equal(c.candles[pair][i].Time) {
				c.candles[pair][i].Orders = append(c.candles[pair][i].Orders, *order)
				c.orders[pair][j] = nil
			}
		}
	}

	for _, order := range c.orders[pair] {
		if order != nil {
			log.Warnf("orders without candle data: %v", order)
		}
	}

	return c.candles[pair]
}

func (c *Chart) Start() error {
	t, err := template.ParseFS(staticFiles, "assets/chart.html")
	if err != nil {
		return err
	}

	http.Handle(
		"/assets/",
		http.FileServer(http.FS(staticFiles)),
	)

	var pairs = make([]string, 0)
	for pair := range c.candles {
		pairs = append(pairs, pair)
	}

	http.HandleFunc("/data", func(w http.ResponseWriter, req *http.Request) {
		pair := req.URL.Query().Get("pair")
		if pair == "" {
			pair = pairs[0]
		}

		w.Header().Set("Content-type", "text/json")
		err := json.NewEncoder(w).Encode(map[string]interface{}{
			"candles":    c.candlesByPair(pair),
			"indicators": c.indicatorsByPair(pair),
		})
		if err != nil {
			log.Error(err)
		}
	})

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		err := t.Execute(w, map[string][]string{
			"pairs": pairs,
		})
		if err != nil {
			log.Error(err)
		}
	})
	fmt.Printf("Chart available at http://localhost:%d\n", c.port)
	return http.ListenAndServe(fmt.Sprintf(":%d", c.port), nil)
}

type Option func(*Chart)

func WithPort(port int) Option {
	return func(chart *Chart) {
		chart.port = port
	}
}

func WithIndicators(indicators ...Indicator) Option {
	return func(chart *Chart) {
		chart.indicators = indicators
	}
}

func NewChart(options ...Option) *Chart {
	chart := &Chart{
		port:      8080,
		candles:   make(map[string][]Candle),
		dataframe: make(map[string]*model.Dataframe),
		orders:    make(map[string][]*Order),
	}
	for _, option := range options {
		option(chart)
	}
	return chart
}
