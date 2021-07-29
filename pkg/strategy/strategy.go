package strategy

import (
	"github.com/rodrigo-brito/ninjabot/pkg/exchange"
	"github.com/rodrigo-brito/ninjabot/pkg/model"
)

type Strategy interface {
	Init(settings model.Settings)
	Timeframe() string
	WarmupPeriod() int
	Indicators(dataframe *model.Dataframe)
	OnCandle(dataframe *model.Dataframe, broker exchange.Broker)
	Finish(dataframe *model.Dataframe, broker exchange.Broker)
}
