package strategy

import (
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/service"
)

type Strategy interface {
	// Timeframe is the time interval in which the strategy will be executed. eg: 1h, 1d, 1w
	Timeframe() string
	// WarmupPeriod is the necessary time to wait before executing the strategy, to load data for indicators.
	// This time is measured in the period specified in the `Timeframe` function.
	WarmupPeriod() int
	// Indicators will be executed for each new candle, in order to fill indicators before `OnCandle` function is called.
	Indicators(df *model.Dataframe) []ChartIndicator
	// OnCandle will be executed for each new candle, after indicators are filled, here you can do your trading logic.
	// OnCandle is executed after the candle close.
	OnCandle(df *model.Dataframe, broker service.Broker)
}

type HighFrequencyStrategy interface {
	Strategy

	// OnPartialCandle will be executed for each new partial candle, after indicators are filled.
	OnPartialCandle(df *model.Dataframe, broker service.Broker)
}
