package strategies

import (
	"fmt"

	"github.com/rodrigo-brito/ninjabot/indicator/stream"
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/service"
	"github.com/rodrigo-brito/ninjabot/strategy"
	"github.com/rodrigo-brito/ninjabot/tools/log"
)

// OCOSellParams are the parameters of OCOSell.
type OCOSellParams struct {
	Timeframe  string  `json:"timeframe" default:"1d" description:"candle timeframe"`
	FastK      int     `json:"fast_k" default:"8" min:"1" description:"period of the fast %K"`
	SlowK      int     `json:"slow_k" default:"3" min:"1" description:"smoothing of %K"`
	SlowD      int     `json:"slow_d" default:"3" min:"1" description:"smoothing of %D"`
	BuyAmount  float64 `json:"buy_amount" default:"4000" min:"0" description:"quote spent per entry"`
	TakeProfit float64 `json:"take_profit" default:"0.1" min:"0" description:"take profit above the entry, as a fraction"`
	StopLoss   float64 `json:"stop_loss" default:"0.05" min:"0" max:"1" description:"stop loss below the entry, as a fraction"`
}

// OCOSell buys a fixed amount when the stochastic %K crosses above %D and
// protects every entry with an OCO order: a take profit and a stop loss. The
// zero value uses the default parameters.
type OCOSell struct {
	Params OCOSellParams
}

// NewOCOSell validates the parameters and creates the strategy.
func NewOCOSell(params OCOSellParams) (*OCOSell, error) {
	params = strategy.WithDefaults(params)
	if params.BuyAmount <= 0 {
		return nil, fmt.Errorf("buy amount must be positive")
	}
	return &OCOSell{Params: params}, nil
}

func (e *OCOSell) params() OCOSellParams { return strategy.WithDefaults(e.Params) }

func (e *OCOSell) Timeframe() string { return e.params().Timeframe }

func (e *OCOSell) WarmupPeriod() int {
	p := e.params()
	return p.FastK + p.SlowK + p.SlowD
}

// Indicators is not used: the strategy registers incremental indicators.
func (e *OCOSell) Indicators(*model.Dataframe) []strategy.ChartIndicator { return nil }

func (e *OCOSell) Setup(_ string, set *strategy.IndicatorSet) {
	e.Params = e.params()
	set.Multi("stoch", stream.NewStoch(e.Params.FastK, e.Params.SlowK, e.Params.SlowD))
	set.Plot(strategy.ChartSpec{
		GroupName: "Stochastic",
		Metrics: []strategy.MetricSpec{
			{Key: "stoch_k", Name: "K", Color: "red"},
			{Key: "stoch_d", Name: "D", Color: "blue"},
		},
	})
}

func (e *OCOSell) OnCandle(df *model.Dataframe, broker service.Broker) {
	closePrice := df.Close.Last(0)
	log.Debug("New Candle = ", df.Pair, df.LastUpdate, closePrice)

	assetPosition, quotePosition, err := broker.Position(df.Pair)
	if err != nil {
		log.Error(err)
		return
	}

	buyAmount := e.Params.BuyAmount
	if quotePosition > buyAmount && df.Metadata["stoch_k"].Crossover(df.Metadata["stoch_d"]) {
		size := buyAmount / closePrice
		fields := map[string]interface{}{
			"pair":  df.Pair,
			"side":  model.SideTypeBuy,
			"close": closePrice,
			"asset": assetPosition,
			"quote": quotePosition,
			"size":  size,
		}
		_, err := broker.CreateOrderMarket(model.SideTypeBuy, df.Pair, size)
		if err != nil {
			log.WithFields(fields).Error(err)
		}

		stop := closePrice * (1 - e.Params.StopLoss)
		_, err = broker.CreateOrderOCO(model.SideTypeSell, df.Pair, size, closePrice*(1+e.Params.TakeProfit), stop, stop)
		if err != nil {
			log.WithFields(fields).Error(err)
		}
	}
}
