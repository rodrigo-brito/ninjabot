package strategies

import (
	"fmt"

	"github.com/rodrigo-brito/ninjabot"
	"github.com/rodrigo-brito/ninjabot/indicator/stream"
	"github.com/rodrigo-brito/ninjabot/service"
	"github.com/rodrigo-brito/ninjabot/strategy"
	"github.com/rodrigo-brito/ninjabot/tools/log"
)

// CrossEMAParams are the parameters of CrossEMA.
type CrossEMAParams struct {
	Timeframe string  `json:"timeframe" default:"4h" description:"candle timeframe"`
	Fast      int     `json:"fast" default:"8" min:"2" description:"period of the fast EMA"`
	Slow      int     `json:"slow" default:"21" min:"2" description:"period of the slow SMA"`
	MinQuote  float64 `json:"min_quote" default:"10" min:"0" description:"minimum quote balance to open a position"`
}

// CrossEMA buys with the whole quote balance when the fast EMA crosses above
// the slow SMA, and sells the position when it crosses below. The zero value
// uses the default parameters.
type CrossEMA struct {
	Params CrossEMAParams
}

// NewCrossEMA validates the parameters and creates the strategy.
func NewCrossEMA(params CrossEMAParams) (*CrossEMA, error) {
	params = strategy.WithDefaults(params)
	if params.Fast >= params.Slow {
		return nil, fmt.Errorf("fast period (%d) must be below the slow period (%d)", params.Fast, params.Slow)
	}
	return &CrossEMA{Params: params}, nil
}

func (e *CrossEMA) params() CrossEMAParams { return strategy.WithDefaults(e.Params) }

func (e *CrossEMA) Timeframe() string { return e.params().Timeframe }

// WarmupPeriod covers the slow average plus the previous bar of a crossover.
func (e *CrossEMA) WarmupPeriod() int { return e.params().Slow + 1 }

// Indicators is not used: the strategy registers incremental indicators.
func (e *CrossEMA) Indicators(*ninjabot.Dataframe) []strategy.ChartIndicator { return nil }

func (e *CrossEMA) Setup(_ string, set *strategy.IndicatorSet) {
	e.Params = e.params()
	set.Line("fast", stream.NewEMA(e.Params.Fast), stream.Close)
	set.Line("slow", stream.NewSMA(e.Params.Slow), stream.Close)
	set.Plot(strategy.ChartSpec{
		GroupName: "MA's",
		Overlay:   true,
		Metrics: []strategy.MetricSpec{
			{Key: "fast", Name: fmt.Sprintf("EMA %d", e.Params.Fast), Color: "red"},
			{Key: "slow", Name: fmt.Sprintf("SMA %d", e.Params.Slow), Color: "blue"},
		},
	})
}

func (e *CrossEMA) OnCandle(df *ninjabot.Dataframe, broker service.Broker) {
	closePrice := df.Close.Last(0)

	assetPosition, quotePosition, err := broker.Position(df.Pair)
	if err != nil {
		log.Error(err)
		return
	}

	if quotePosition >= e.Params.MinQuote && // minimum quote position to trade
		df.Metadata["fast"].Crossover(df.Metadata["slow"]) { // trade signal (EMA > SMA)

		amount := quotePosition / closePrice // calculate amount of asset to buy
		_, err := broker.CreateOrderMarket(ninjabot.SideTypeBuy, df.Pair, amount)
		if err != nil {
			log.Error(err)
		}

		return
	}

	if assetPosition > 0 &&
		df.Metadata["fast"].Crossunder(df.Metadata["slow"]) { // trade signal (EMA < SMA)

		_, err = broker.CreateOrderMarket(ninjabot.SideTypeSell, df.Pair, assetPosition)
		if err != nil {
			log.Error(err)
		}
	}
}
