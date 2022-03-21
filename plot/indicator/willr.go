package indicator

import (
	"fmt"
	"time"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/plot"

	"github.com/markcheno/go-talib"
)

func WillR(period int, color string) plot.Indicator {
	return &willR{
		Period: period,
		Color:  color,
	}
}

type willR struct {
	Period int
	Color  string
	Values model.Series
	Time   []time.Time
}

func (w willR) Name() string {
	return fmt.Sprintf("%%R(%d)", w.Period)
}

func (w willR) Overlay() bool {
	return false
}

func (w *willR) Load(dataframe *model.Dataframe) {
	if len(dataframe.Time) < w.Period {
		return
	}

	w.Values = talib.WillR(dataframe.High, dataframe.Low, dataframe.Close, w.Period)[w.Period:]
	w.Time = dataframe.Time[w.Period:]
}

func (w willR) Metrics() []plot.IndicatorMetric {
	return []plot.IndicatorMetric{
		{
			Style:  "line",
			Color:  w.Color,
			Values: w.Values,
			Time:   w.Time,
		},
	}
}
