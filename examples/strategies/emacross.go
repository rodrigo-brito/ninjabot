package strategies

import (
	"github.com/rodrigo-brito/ninjabot"
	"github.com/rodrigo-brito/ninjabot/indicator"
	"github.com/rodrigo-brito/ninjabot/service"
	"github.com/rodrigo-brito/ninjabot/strategy"
	"github.com/rodrigo-brito/ninjabot/tools/log"
)

type CrossEMA struct{}

func (e CrossEMA) Timeframe() string {
	return "4h"
}

func (e CrossEMA) WarmupPeriod() int {
	return 21
}

func (e CrossEMA) Indicators(df *ninjabot.Dataframe) []strategy.ChartIndicator {
	df.Metadata["ema8"] = indicator.EMA(df.Close, 8)
	df.Metadata["sma21"] = indicator.SMA(df.Close, 21)

	return []strategy.ChartIndicator{
		{
			Overlay:   true,
			GroupName: "MA's",
			Time:      df.Time,
			Metrics: []strategy.IndicatorMetric{
				{
					Values: df.Metadata["ema8"],
					Name:   "EMA 8",
					Color:  "red",
					Style:  strategy.StyleLine,
				},
				{
					Values: df.Metadata["sma21"],
					Name:   "SMA 21",
					Color:  "blue",
					Style:  strategy.StyleLine,
				},
			},
		},
	}
}

func (e *CrossEMA) OnCandle(df *ninjabot.Dataframe, broker service.Broker) {
	closePrice := df.Close.Last(0)

	assetPosition, quotePosition, err := broker.Position(df.Pair)
	if err != nil {
		log.Error(err)
		return
	}

	if (quotePosition > 10 || assetPosition*closePrice < -10) && df.Metadata["ema8"].Crossover(df.Metadata["sma21"]) {
		if assetPosition < 0 {
			// close previous short position
			_, err := broker.CreateOrderMarket(ninjabot.SideTypeBuy, df.Pair, -assetPosition)
			if err != nil {
				log.Error(err)
			}

			assetPosition, quotePosition, err = broker.Position(df.Pair)
			if err != nil {
				log.Error(err)
				return
			}
		}

		_, err = broker.CreateOrderMarket(ninjabot.SideTypeBuy, df.Pair, quotePosition/closePrice*0.99)
		if err != nil {
			log.Error(err)
		}
		return
	}

	if (assetPosition*closePrice > 10 || quotePosition > 10) &&
		df.Metadata["ema8"].Crossunder(df.Metadata["sma21"]) {
		if assetPosition > 0 {
			// close previous long position
			_, err := broker.CreateOrderMarket(ninjabot.SideTypeSell, df.Pair, assetPosition)
			if err != nil {
				log.Error(err)
			}

			assetPosition, quotePosition, err = broker.Position(df.Pair)
			if err != nil {
				log.Error(err)
				return
			}
		}

		_, err = broker.CreateOrderMarket(ninjabot.SideTypeSell, df.Pair, quotePosition/closePrice*0.99)
		if err != nil {
			log.Error(err)
		}
	}
}
