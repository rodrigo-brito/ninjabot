package indicator

import (
	"fmt"
	"time"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/plot"

	"github.com/markcheno/go-talib"
)

func SMA(period int, color string) plot.Indicator {
	return &sma{
		Period: period,
		Color:  color,
	}
}

type sma struct {
	Period int
	Color  string
	Values model.Series
	Time   []time.Time
}

func (s sma) Name() string {
	return fmt.Sprintf("SMA(%d)", s.Period)
}

func (s sma) Overlay() bool {
	return true
}

func (s *sma) Load(dataframe *model.Dataframe) {
	if len(dataframe.Time) < s.Period {
		return
	}

	s.Values = talib.Sma(dataframe.Close, s.Period)[s.Period:]
	s.Time = dataframe.Time[s.Period:]
}

func (s sma) Metrics() []plot.IndicatorMetric {
	return []plot.IndicatorMetric{
		{
			Style:  "line",
			Color:  s.Color,
			Values: s.Values,
			Time:   s.Time,
		},
	}
}
