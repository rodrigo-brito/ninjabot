package plot

import (
	"fmt"
	"time"

	"github.com/markcheno/go-talib"

	"github.com/rodrigo-brito/ninjabot/model"
)

type Metric struct {
	Name   string
	Color  string
	Style  string
	Values model.Series
	Time   []time.Time
}

type Indicator interface {
	Name() string
	Overlay() bool
	Metrics() []Metric
	Load(dataframe *model.Dataframe)
}

func EMA(period int, color string) Indicator {
	return &ema{
		Period: period,
		Color:  color,
	}
}

type ema struct {
	Period int
	Color  string
	Values model.Series
	Time   []time.Time
}

func (e ema) Name() string {
	return fmt.Sprintf("EMA(%d)", e.Period)
}

func (e ema) Overlay() bool {
	return true
}

func (e *ema) Load(dataframe *model.Dataframe) {
	e.Values = talib.Ema(dataframe.Close, e.Period)
	e.Time = dataframe.Time
}

func (e ema) Metrics() []Metric {
	return []Metric{
		{
			Name:   "value",
			Style:  "line",
			Color:  e.Color,
			Values: e.Values,
			Time:   e.Time,
		},
	}
}

func RSI(period int, color string) Indicator {
	return &ema{
		Period: period,
		Color:  color,
	}
}

type rsi struct {
	Period int
	Color  string
	Values model.Series
	Time   []time.Time
}

func (e rsi) Name() string {
	return fmt.Sprintf("RSI(%d)", e.Period)
}

func (e rsi) Overlay() bool {
	return false
}

func (e *rsi) Load(dataframe *model.Dataframe) {
	e.Values = talib.Rsi(dataframe.Close, e.Period)
	e.Time = dataframe.Time
}

func (e rsi) Metrics() []Metric {
	return []Metric{
		{
			Name:   "value",
			Color:  e.Color,
			Style:  "line",
			Values: e.Values,
			Time:   e.Time,
		},
	}
}
