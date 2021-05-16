package plot

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"

	"github.com/rodrigo-brito/ninjabot/pkg/model"

	log "github.com/sirupsen/logrus"
)

//go:embed assets
var staticFiles embed.FS

type Chart struct {
	port    int
	candles []model.Candle
	orders  []model.Order
}

func (c *Chart) OnOrder(order model.Order) {
	c.orders = append(c.orders, order)
}

func (c *Chart) OnCandle(candle model.Candle) {
	if candle.Complete {
		c.candles = append(c.candles, candle)
	}
}

func (c *Chart) Start() error {
	t, err := template.ParseFS(staticFiles, "assets/chart.html.tmpl")
	if err != nil {
		return err
	}

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Add("Content-Type", "text/html")
		err := t.Execute(w, struct {
			Candles []model.Candle
			Orders  []model.Order
		}{
			Candles: c.candles,
			Orders:  c.orders,
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
		port: 8080,
	}
	for _, option := range options {
		option(chart)
	}
	return chart
}
