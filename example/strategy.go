package example

import (
	"github.com/rodrigo-brito/ninjabot/pkg/exchange"
	"github.com/rodrigo-brito/ninjabot/pkg/model"

	"github.com/markcheno/go-talib"
	log "github.com/sirupsen/logrus"
)

type MyStrategy struct{}

func (e MyStrategy) Init(settings model.Settings) {}

func (e MyStrategy) Timeframe() string {
	return "1h"
}

func (e MyStrategy) WarmupPeriod() int {
	return 21
}

func (e MyStrategy) Indicators(df *model.Dataframe) {
	df.Metadata["ema9"] = talib.Ema(df.Close, 9)
	df.Metadata["ema21"] = talib.Ema(df.Close, 21)
}

func (e *MyStrategy) OnCandle(df *model.Dataframe, broker exchange.Broker) {
	closePrice := df.Close.Last(0)
	log.Info("New Candle = ", df.Pair, df.LastUpdate, closePrice)

	assetPosition, quotePosition, err := broker.Position(df.Pair)
	if err != nil {
		log.Error(err)
	}

	buyAmount := 4000.0
	if quotePosition > buyAmount && df.Metadata["ema9"].Crossover(df.Metadata["ema21"]) {
		size := buyAmount / closePrice
		_, err := broker.OrderMarket(model.SideTypeBuy, df.Pair, size)
		if err != nil {
			log.WithFields(map[string]interface{}{
				"pair":  df.Pair,
				"side":  model.SideTypeBuy,
				"close": closePrice,
				"asset": assetPosition,
				"quote": quotePosition,
				"size":  size,
			}).Error(err)
		}
	}

	if assetPosition*closePrice > 10 && // minimum tradable size
		df.Metadata["ema9"].Crossunder(df.Metadata["ema21"]) {
		_, err := broker.OrderMarket(model.SideTypeSell, df.Pair, assetPosition)
		if err != nil {
			log.WithFields(map[string]interface{}{
				"pair":  df.Pair,
				"side":  model.SideTypeSell,
				"close": closePrice,
				"asset": assetPosition,
				"quote": quotePosition,
				"size":  assetPosition,
			}).Error(err)
		}
	}
}
