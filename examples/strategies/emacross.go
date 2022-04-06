package strategies

import (
	"github.com/rodrigo-brito/ninjabot"
	"github.com/rodrigo-brito/ninjabot/service"

	"github.com/markcheno/go-talib"
	log "github.com/sirupsen/logrus"
)

type CrossEMA struct{}

func (e CrossEMA) Timeframe() string {
	return "4h"
}

func (e CrossEMA) WarmupPeriod() int {
	return 21
}

func (e CrossEMA) Indicators(df *ninjabot.Dataframe) {
	df.Metadata["ema8"] = talib.Ema(df.Close, 8)
	df.Metadata["sma21"] = talib.Sma(df.Close, 21)
}

func (e *CrossEMA) OnCandle(df *ninjabot.Dataframe, broker service.Broker) {
	closePrice := df.Close.Last(0)

	assetPosition, quotePosition, err := broker.Position(df.Pair)
	if err != nil {
		log.Error(err)
		return
	}

	if quotePosition > 10 && df.Metadata["ema8"].Crossover(df.Metadata["sma21"]) {
		_, err := broker.CreateOrderMarketQuote(ninjabot.SideTypeBuy, df.Pair, quotePosition)
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

	if assetPosition > 0 &&
		df.Metadata["ema8"].Crossunder(df.Metadata["sma21"]) {
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
