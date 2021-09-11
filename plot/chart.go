package plot

import (
	"embed"
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
	port    int
	candles map[string][]Candle
	orders  map[string][]*Order
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

	if candle.Complete {
		c.candles[candle.Symbol] = append(c.candles[candle.Symbol], Candle{
			Time:   candle.Time,
			Open:   candle.Open,
			Close:  candle.Close,
			High:   candle.High,
			Low:    candle.Low,
			Volume: candle.Volume,
			Orders: make([]Order, 0),
		})
	}
}

func (c *Chart) CandlesByPair(pair string) []Candle {
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

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		pair := req.URL.Query().Get("pair")
		if pair == "" {
			pair = pairs[0]
		}

		candles := c.CandlesByPair(pair)

		w.Header().Add("Content-Type", "text/html")
		err := t.Execute(w, struct {
			Pairs   []string
			Candles []Candle
		}{
			Pairs:   pairs,
			Candles: candles,
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

func NewChart(options ...Option) *Chart {
	chart := &Chart{
		port:    8080,
		candles: make(map[string][]Candle),
		orders:  make(map[string][]*Order),
	}
	for _, option := range options {
		option(chart)
	}
	return chart
}
