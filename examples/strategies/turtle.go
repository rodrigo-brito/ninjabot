package strategies

import (
	"github.com/rodrigo-brito/ninjabot"
	"github.com/rodrigo-brito/ninjabot/service"

	"github.com/markcheno/go-talib"
	log "github.com/sirupsen/logrus"
)

// https://www.investopedia.com/articles/trading/08/turtle-trading.asp
type Turtle struct{}

func (e Turtle) Timeframe() string {
	return "4h"
}

func (e Turtle) WarmupPeriod() int {
	return 40
}

func (e Turtle) Indicators(df *ninjabot.Dataframe) {
	df.Metadata["turtleHighest"] = talib.Max(df.Close, 40)
	df.Metadata["turtleLowest"] = talib.Min(df.Close, 20)
}

func (e *Turtle) OnCandle(df *ninjabot.Dataframe, broker service.Broker) {
	closePrice := df.Close.Last(0)
	maxS := df.Metadata["turtleHighest"]
	highest := maxS[len(maxS)-1]
	minS := df.Metadata["turtleLowest"]
	lowest := minS[len(minS)-1]
	assetPosition, quotePosition, err := broker.Position(df.Pair)
	if err != nil {
		log.Error(err)
	}

	// If position already open wait till it will be closed
	if assetPosition == 0 && closePrice >= highest {
		_, err := broker.CreateOrderMarketQuote(ninjabot.SideTypeBuy, df.Pair, quotePosition/2)
		if err != nil {
			log.WithFields(map[string]interface{}{
				"pair":  df.Pair,
				"side":  ninjabot.SideTypeBuy,
				"close": closePrice,
				"asset": assetPosition,
				"quote": quotePosition,
			}).Error(err)
		}
		return
	}

	if assetPosition > 0 && closePrice <= lowest {
		_, err := broker.CreateOrderMarket(ninjabot.SideTypeSell, df.Pair, assetPosition)
		if err != nil {
			log.WithFields(map[string]interface{}{
				"pair":  df.Pair,
				"side":  ninjabot.SideTypeSell,
				"close": closePrice,
				"asset": assetPosition,
				"quote": quotePosition,
				"size":  assetPosition,
			}).Error(err)
		}
	}
}
