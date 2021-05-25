package plot

import (
	"embed"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"sync"

	"github.com/rodrigo-brito/ninjabot/pkg/model"

	log "github.com/sirupsen/logrus"
)

//go:embed assets
var staticFiles embed.FS

type Chart struct {
	sync.Mutex
	port    int
	candles map[string][]model.Candle
	orders  map[string][]model.Order
}

func (c *Chart) OnOrder(order model.Order) {
	c.Lock()
	defer c.Unlock()

	candles := c.candles[order.Symbol]
	order.Candle = candles[len(candles)-1]

	// find the closes candle to the order
	for i := len(candles) - 1; i >= 0; i-- {
		// check if the the candle is closest
		newPosition := float64(order.UpdatedAt.Sub(candles[i].Time))
		lastPosition := float64(order.UpdatedAt.Sub(order.Candle.Time))
		if math.Abs(newPosition) <= math.Abs(lastPosition) {
			order.Candle = candles[i]
		} else {
			// if the distance increase, stop
			break
		}
	}
	c.orders[order.Symbol] = append(c.orders[order.Symbol], order)
}

func (c *Chart) OnCandle(candle model.Candle) {
	c.Lock()
	defer c.Unlock()

	if candle.Complete {
		c.candles[candle.Symbol] = append(c.candles[candle.Symbol], candle)
	}
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

		var filledOrders []model.Order
		for _, order := range c.orders[pair] {
			if order.Status != model.OrderStatusTypeFilled {
				continue
			}
			filledOrders = append(filledOrders, order)
		}

		w.Header().Add("Content-Type", "text/html")
		err := t.Execute(w, struct {
			Pairs   []string
			Candles []model.Candle
			Orders  []model.Order
		}{
			Pairs:   pairs,
			Candles: c.candles[pair],
			Orders:  filledOrders,
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
		candles: make(map[string][]model.Candle),
		orders:  make(map[string][]model.Order),
	}
	for _, option := range options {
		option(chart)
	}
	return chart
}
