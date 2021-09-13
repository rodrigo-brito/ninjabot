package strategies

import (
	"github.com/rodrigo-brito/ninjabot"
	"github.com/rodrigo-brito/ninjabot/service"

	"github.com/markcheno/go-talib"
	log "github.com/sirupsen/logrus"
)

type CrossEMA struct{}

func (e CrossEMA) Timeframe() string {
	return "1d"
}

func (e CrossEMA) WarmupPeriod() int {
	return 9
}

func (e CrossEMA) Indicators(df *ninjabot.Dataframe) {
	df.Metadata["ema9"] = talib.Ema(df.Close, 9)
}

func (e *CrossEMA) OnCandle(df *ninjabot.Dataframe, broker service.Broker) {
	closePrice := df.Close.Last(0)
	log.Info("New Candle = ", df.Pair, df.LastUpdate, closePrice)

	assetPosition, quotePosition, err := broker.Position(df.Pair)
	if err != nil {
		log.Error(err)
	}

	buyAmount := 4000.0
	if quotePosition > buyAmount && df.Close.Crossover(df.Metadata["ema9"]) {
		size := buyAmount / closePrice
		_, err := broker.CreateOrderMarket(ninjabot.SideTypeBuy, df.Pair, size)
		if err != nil {
			log.WithFields(map[string]interface{}{
				"pair":  df.Pair,
				"side":  ninjabot.SideTypeBuy,
				"close": closePrice,
				"asset": assetPosition,
				"quote": quotePosition,
				"size":  size,
			}).Error(err)
		}
	}

	if assetPosition*closePrice > 10 && // minimum tradable size
		df.Close.Crossunder(df.Metadata["ema9"]) {
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
