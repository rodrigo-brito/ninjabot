package indicator

import (
	"fmt"
	"time"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/plot"

	"github.com/markcheno/go-talib"
)

func Stoch(periodK, peridodD int, colorK, colorD string) plot.Indicator {
	return &stoch{
		PeriodK: periodK,
		PeriodD: peridodD,
		ColorK:  colorK,
		ColorD:  colorD,
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

	e.ValuesK, e.ValuesD = talib.Stoch(
		dataframe.High, dataframe.Low, dataframe.Close, e.PeriodK, e.PeriodD, talib.SMA, e.PeriodD, talib.SMA,
	)
	e.ValuesK = e.ValuesK[e.PeriodK+e.PeriodD:]
	e.ValuesD = e.ValuesD[e.PeriodK+e.PeriodD:]
	e.Time = dataframe.Time[e.PeriodK+e.PeriodD:]
}

func (e stoch) Metrics() []plot.IndicatorMetric {
	return []plot.IndicatorMetric{
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
