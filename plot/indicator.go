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
	if len(dataframe.Time) < e.Period {
		return
	}

	e.Values = talib.Ema(dataframe.Close, e.Period)[e.Period:]
	e.Time = dataframe.Time[e.Period:]
}

func (e ema) Metrics() []Metric {
	return []Metric{
		{
			Style:  "line",
			Color:  e.Color,
			Values: e.Values,
			Time:   e.Time,
		},
	}
}

func RSI(period int, color string) Indicator {
	return &rsi{
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
	if len(dataframe.Time) < e.Period {
		return
	}

	e.Values = talib.Rsi(dataframe.Close, e.Period)[e.Period:]
	e.Time = dataframe.Time[e.Period:]
}

func (e rsi) Metrics() []Metric {
	return []Metric{
		{
			Color:  e.Color,
			Style:  "line",
			Values: e.Values,
			Time:   e.Time,
		},
	}
}

func Stoch(k, d int, colork, colord string) Indicator {
	return &stoch{
		PeriodK: k,
		PeriodD: d,
		ColorK:  colork,
		ColorD:  colord,
	}
}

type stoch struct {
	PeriodK int
	PeriodD int
	ColorK  string
	ColorD  string
	ValuesK model.Series
	ValuesD model.Series
	Time    []time.Time
}

func (e stoch) Name() string {
	return fmt.Sprintf("STOCH(%d, %d)", e.PeriodK, e.PeriodD)
}

func (e stoch) Overlay() bool {
	return false
}

func (e *stoch) Load(dataframe *model.Dataframe) {
	if len(dataframe.Time) < e.PeriodK+e.PeriodD {
		return
	}

	e.ValuesK, e.ValuesD = talib.Stoch(dataframe.High, dataframe.Low, dataframe.Close, e.PeriodK, e.PeriodD, talib.SMA, e.PeriodD, talib.SMA)
	e.ValuesK = e.ValuesK[e.PeriodK+e.PeriodD:]
	e.ValuesD = e.ValuesD[e.PeriodK+e.PeriodD:]
	e.Time = dataframe.Time[e.PeriodK+e.PeriodD:]
}

func (e stoch) Metrics() []Metric {
	return []Metric{
		{
			Color:  e.ColorK,
			Name:   "K",
			Style:  "line",
			Values: e.ValuesK,
			Time:   e.Time,
		},
		{
			Color:  e.ColorD,
			Name:   "D",
			Style:  "line",
			Values: e.ValuesD,
			Time:   e.Time,
		},
	}
}
