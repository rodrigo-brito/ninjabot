package strategy

import (
	"github.com/rodrigo-brito/ninjabot/pkg/model"
	"github.com/rodrigo-brito/ninjabot/pkg/service"
)

type Strategy interface {
	Init(settings model.Settings)
	Timeframe() string
	WarmupPeriod() int
	Indicators(dataframe *model.Dataframe)
	OnCandle(dataframe *model.Dataframe, broker service.Broker)
}
