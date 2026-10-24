package strategies

import (
	"fmt"

	"github.com/rodrigo-brito/ninjabot"
	"github.com/rodrigo-brito/ninjabot/indicator/stream"
	"github.com/rodrigo-brito/ninjabot/service"
	"github.com/rodrigo-brito/ninjabot/strategy"
	"github.com/rodrigo-brito/ninjabot/tools/log"
)

// TurtleParams are the parameters of Turtle.
type TurtleParams struct {
	Timeframe  string  `json:"timeframe" default:"4h" description:"candle timeframe"`
	Entry      int     `json:"entry" default:"40" min:"2" description:"breakout period: buy at the highest close"`
	Exit       int     `json:"exit" default:"20" min:"2" description:"exit period: sell at the lowest close"`
	Allocation float64 `json:"allocation" default:"0.5" min:"0" max:"1" description:"share of the quote balance per entry"`
}

// Turtle is a breakout strategy: it buys when the close reaches the highest
// close of the entry period and sells when it reaches the lowest close of the
// exit period. The zero value uses the default parameters.
// https://www.investopedia.com/articles/trading/08/turtle-trading.asp
type Turtle struct {
	Params TurtleParams
}

// NewTurtle validates the parameters and creates the strategy.
func NewTurtle(params TurtleParams) (*Turtle, error) {
	params = strategy.WithDefaults(params)
	if params.Allocation <= 0 {
		return nil, fmt.Errorf("allocation must be positive")
	}
	return &Turtle{Params: params}, nil
}

func (e *Turtle) params() TurtleParams { return strategy.WithDefaults(e.Params) }

func (e *Turtle) Timeframe() string { return e.params().Timeframe }

func (e *Turtle) WarmupPeriod() int {
	p := e.params()
	return max(p.Entry, p.Exit)
}

// Indicators is not used: the strategy registers incremental indicators.
func (e *Turtle) Indicators(*ninjabot.Dataframe) []strategy.ChartIndicator { return nil }

func (e *Turtle) Setup(_ string, set *strategy.IndicatorSet) {
	e.Params = e.params()
	set.Line("highest", stream.NewMax(e.Params.Entry), stream.Close)
	set.Line("lowest", stream.NewMin(e.Params.Exit), stream.Close)
	set.Plot(strategy.ChartSpec{
		GroupName: "Channel",
		Overlay:   true,
		Metrics: []strategy.MetricSpec{
			{Key: "highest", Name: fmt.Sprintf("Max %d", e.Params.Entry), Color: "green"},
			{Key: "lowest", Name: fmt.Sprintf("Min %d", e.Params.Exit), Color: "red"},
		},
	})
}

func (e *Turtle) OnCandle(df *ninjabot.Dataframe, broker service.Broker) {
	closePrice := df.Close.Last(0)
	highest := df.Metadata["highest"].Last(0)
	lowest := df.Metadata["lowest"].Last(0)

	assetPosition, quotePosition, err := broker.Position(df.Pair)
	if err != nil {
		log.Error(err)
		return
	}

	// If position already open wait till it will be closed
	if assetPosition == 0 && closePrice >= highest {
		_, err := broker.CreateOrderMarketQuote(ninjabot.SideTypeBuy, df.Pair, quotePosition*e.Params.Allocation)
		if err != nil {
			log.Error(err)
		}
		return
	}

	if assetPosition > 0 && closePrice <= lowest {
		_, err := broker.CreateOrderMarket(ninjabot.SideTypeSell, df.Pair, assetPosition)
		if err != nil {
			log.Error(err)
		}
	}
}
