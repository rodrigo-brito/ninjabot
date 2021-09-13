package strategies

import (
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/service"

	"github.com/markcheno/go-talib"
	log "github.com/sirupsen/logrus"
)

type OCOSell struct{}

func (e OCOSell) Timeframe() string {
	return "1d"
}

func (e OCOSell) WarmupPeriod() int {
	return 9
}

func (e OCOSell) Indicators(df *model.Dataframe) {
	df.Metadata["stoch"], df.Metadata["stoch_signal"] = talib.Stoch(
		df.High,
		df.Low,
		df.Close,
		8,
		3,
		talib.SMA,
		3,
		talib.SMA,
	)
}

func (e *OCOSell) OnCandle(df *model.Dataframe, broker service.Broker) {
	closePrice := df.Close.Last(0)
	log.Info("New Candle = ", df.Pair, df.LastUpdate, closePrice)

	assetPosition, quotePosition, err := broker.Position(df.Pair)
	if err != nil {
		log.Error(err)
	}

	buyAmount := 4000.0
	if quotePosition > buyAmount && df.Metadata["stoch"].Crossover(df.Metadata["stoch_signal"]) {
		size := buyAmount / closePrice
		_, err := broker.CreateOrderMarket(model.SideTypeBuy, df.Pair, size)
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

		_, err = broker.CreateOrderOCO(model.SideTypeSell, df.Pair, size, closePrice*1.05, closePrice*0.95, closePrice*0.95)
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
}
