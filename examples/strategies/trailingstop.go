package strategies

import (
	"fmt"

	"github.com/rodrigo-brito/ninjabot"
	"github.com/rodrigo-brito/ninjabot/indicator/stream"
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/service"
	"github.com/rodrigo-brito/ninjabot/strategy"
	"github.com/rodrigo-brito/ninjabot/tools"
	"github.com/rodrigo-brito/ninjabot/tools/log"
)

// TrailingParams are the parameters of the trailing stop strategy.
type TrailingParams struct {
	Timeframe string  `json:"timeframe" default:"4h" description:"candle timeframe"`
	Fast      int     `json:"fast" default:"8" min:"2" description:"period of the fast EMA"`
	Slow      int     `json:"slow" default:"21" min:"2" description:"period of the slow SMA"`
	MinQuote  float64 `json:"min_quote" default:"10" min:"0" description:"minimum quote balance to open a position"`
}

// trailing buys on an EMA/SMA crossover and exits with a trailing stop
// updated on every partial candle.
type trailing struct {
	params       TrailingParams
	trailingStop map[string]*tools.TrailingStop
}

// NewTrailing creates the trailing stop strategy with the default
// parameters.
func NewTrailing(pairs []string) strategy.HighFrequencyStrategy {
	s, _ := NewTrailingWithParams(TrailingParams{})
	for _, pair := range pairs {
		s.(*trailing).stop(pair)
	}
	return s
}

// NewTrailingWithParams validates the parameters and creates the strategy.
func NewTrailingWithParams(params TrailingParams) (strategy.HighFrequencyStrategy, error) {
	params = strategy.WithDefaults(params)
	if params.Fast >= params.Slow {
		return nil, fmt.Errorf("fast period (%d) must be below the slow period (%d)", params.Fast, params.Slow)
	}
	return &trailing{params: params, trailingStop: make(map[string]*tools.TrailingStop)}, nil
}

// stop returns the trailing stop of a pair, created on first use.
func (t *trailing) stop(pair string) *tools.TrailingStop {
	stop, ok := t.trailingStop[pair]
	if !ok {
		stop = tools.NewTrailingStop()
		t.trailingStop[pair] = stop
	}
	return stop
}

func (t *trailing) Timeframe() string { return t.params.Timeframe }

func (t *trailing) WarmupPeriod() int { return t.params.Slow }

// Indicators is not used: the strategy registers incremental indicators.
func (t *trailing) Indicators(*model.Dataframe) []strategy.ChartIndicator { return nil }

func (t *trailing) Setup(pair string, set *strategy.IndicatorSet) {
	t.stop(pair)
	set.Line("ema_fast", stream.NewEMA(t.params.Fast), stream.Close)
	set.Line("sma_slow", stream.NewSMA(t.params.Slow), stream.Close)
}

func (t *trailing) OnCandle(df *model.Dataframe, broker service.Broker) {
	asset, quote, err := broker.Position(df.Pair)
	if err != nil {
		log.Error(err)
		return
	}

	if quote > t.params.MinQuote && // enough cash?
		asset*df.Close.Last(0) < t.params.MinQuote && // without position yet
		df.Metadata["ema_fast"].Crossover(df.Metadata["sma_slow"]) {
		_, err = broker.CreateOrderMarketQuote(ninjabot.SideTypeBuy, df.Pair, quote)
		if err != nil {
			log.Error(err)
			return
		}

		t.stop(df.Pair).Start(df.Close.Last(0), df.Low.Last(0))
	}
}

func (t *trailing) OnPartialCandle(df *model.Dataframe, broker service.Broker) {
	if trailing := t.stop(df.Pair); trailing.Update(df.Close.Last(0)) {
		asset, _, err := broker.Position(df.Pair)
		if err != nil {
			log.Error(err)
			return
		}

		if asset > 0 {
			_, err = broker.CreateOrderMarket(ninjabot.SideTypeSell, df.Pair, asset)
			if err != nil {
				log.Error(err)
				return
			}
			trailing.Stop()
		}
	}
}
