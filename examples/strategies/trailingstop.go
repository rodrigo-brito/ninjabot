package strategies

import (
	"github.com/rodrigo-brito/ninjabot"
	"github.com/rodrigo-brito/ninjabot/indicator"
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/service"
	"github.com/rodrigo-brito/ninjabot/strategy"
	"github.com/rodrigo-brito/ninjabot/tools"

	log "github.com/sirupsen/logrus"
)

type trailing struct {
	trailingStop map[string]*tools.TrailingStop
	scheduler    map[string]*tools.Scheduler
}

func NewTrailing(pairs []string) strategy.HighFrequencyStrategy {
	strategy := &trailing{
		trailingStop: make(map[string]*tools.TrailingStop),
		scheduler:    make(map[string]*tools.Scheduler),
	}

	for _, pair := range pairs {
		strategy.trailingStop[pair] = tools.NewTrailingStop()
		strategy.scheduler[pair] = tools.NewScheduler(pair)
	}

	return strategy
}

func (t trailing) Timeframe() string {
	return "4h"
}

func (t trailing) WarmupPeriod() int {
	return 21
}

func (t trailing) Indicators(df *model.Dataframe) []strategy.ChartIndicator {
	df.Metadata["ema_fast"] = indicator.EMA(df.Close, 8)
	df.Metadata["sma_slow"] = indicator.SMA(df.Close, 21)

	return nil
}

func (t trailing) OnCandle(df *model.Dataframe, broker service.Broker) {
	asset, quote, err := broker.Position(df.Pair)
	if err != nil {
		log.Error(err)
		return
	}

	if quote > 10.0 && // enough cash?
		asset*df.Close.Last(0) < 10 && // without position yet
		df.Metadata["ema_fast"].Crossover(df.Metadata["sma_slow"]) {
		_, err = broker.CreateOrderMarketQuote(ninjabot.SideTypeBuy, df.Pair, quote)
		if err != nil {
			log.Error(err)
			return
		}

		t.trailingStop[df.Pair].Start(df.Close.Last(0), df.Low.Last(0))

		return
	}
}

func (t trailing) OnPartialCandle(df *model.Dataframe, broker service.Broker) {
	if trailing := t.trailingStop[df.Pair]; trailing != nil && trailing.Update(df.Close.Last(0)) {
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
