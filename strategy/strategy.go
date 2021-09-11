package strategy

import (
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/service"
)

type Strategy interface {
	Timeframe() string
	WarmupPeriod() int
	Indicators(dataframe *model.Dataframe)
	OnCandle(dataframe *model.Dataframe, broker service.Broker)
}
