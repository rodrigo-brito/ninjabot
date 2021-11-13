package indicator

import (
	"fmt"
	"time"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/plot"

	"github.com/markcheno/go-talib"
)

func Stoch(fastK, slowK, slowD int, colorK, colorD string) plot.Indicator {
	return &stoch{
		FastK:  fastK,
		SlowK:  slowK,
		SlowD:  slowD,
		ColorK: colorK,
		ColorD: colorD,
	}
}

type stoch struct {
	FastK   int
	SlowK   int
	SlowD   int
	ColorK  string
	ColorD  string
	ValuesK model.Series
	ValuesD model.Series
	Time    []time.Time
}

func (e stoch) Name() string {
	return fmt.Sprintf("STOCH(%d, %d, %d)", e.FastK, e.SlowK, e.SlowD)
}

func (e stoch) Overlay() bool {
	return false
}

func (e *stoch) Load(dataframe *model.Dataframe) {
	e.ValuesK, e.ValuesD = talib.Stoch(
		dataframe.High, dataframe.Low, dataframe.Close, e.FastK, e.SlowK, talib.SMA, e.SlowD, talib.SMA,
	)
	e.Time = dataframe.Time
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
