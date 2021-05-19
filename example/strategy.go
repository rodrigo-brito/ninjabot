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
	return "1d"
}

func (e MyStrategy) WarmupPeriod() int {
	return 9
}

func (e MyStrategy) Indicators(dataframe *model.Dataframe) {
	dataframe.Metadata["ema"] = talib.Ema(dataframe.Close, 9)
}

func (e *MyStrategy) OnCandle(dataframe *model.Dataframe, broker exchange.Broker) {
	closePrice := model.Last(dataframe.Close, 0)
	log.Info("New Candle = ", dataframe.Pair, dataframe.LastUpdate, closePrice)

	assetPosition, quotePosition, err := broker.Position(dataframe.Pair)
	if err != nil {
		log.Error(err)
	}

	if quotePosition > 1000 && // minimum size
		assetPosition*closePrice < 10 && // no position
		model.Last(dataframe.Metadata["ema"], 0) > model.Last(dataframe.Metadata["ema"], 1) {
		size := 1000 / closePrice
		_, err := broker.OrderMarket(model.SideTypeBuy, dataframe.Pair, size)
		if err != nil {
			log.WithFields(map[string]interface{}{
				"pair":  dataframe.Pair,
				"side":  model.SideTypeBuy,
				"close": closePrice,
				"asset": assetPosition,
				"quote": quotePosition,
				"size":  size,
			}).Error(err)
		}
	}

	if assetPosition*closePrice > 10 && // minimum size
		model.Last(dataframe.Metadata["ema"], 0) < model.Last(dataframe.Metadata["ema"], 1) {
		_, err := broker.OrderMarket(model.SideTypeSell, dataframe.Pair, assetPosition)
		if err != nil {
			log.WithFields(map[string]interface{}{
				"pair":  dataframe.Pair,
				"side":  model.SideTypeSell,
				"close": closePrice,
				"asset": assetPosition,
				"quote": quotePosition,
				"size":  assetPosition,
			}).Error(err)
		}
	}
}
