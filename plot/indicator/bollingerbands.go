package indicator

import (
	"fmt"
	"time"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/plot"

	"github.com/markcheno/go-talib"
)

func BollingerBands(period int, stdDeviation float64, upDnBandColor, midBandColor string) plot.Indicator {
	return &bollingerBands{
		Period:        period,
		StdDeviation:  stdDeviation,
		UpDnBandColor: upDnBandColor,
		MidBandColor:  midBandColor,
	}
}

type bollingerBands struct {
	Period        int
	StdDeviation  float64
	UpDnBandColor string
	MidBandColor  string
	UpperBand     model.Series
	MiddleBand    model.Series
	LowerBand     model.Series
	Time          []time.Time
}

func (bb bollingerBands) Name() string {
	return fmt.Sprintf("BB(%d, %.2f)", bb.Period, bb.StdDeviation)
}

func (bb bollingerBands) Overlay() bool {
	return true
}

func (bb *bollingerBands) Load(dataframe *model.Dataframe) {
	if len(dataframe.Time) < bb.Period {
		return
	}

	upper, mid, lower := talib.BBands(dataframe.Close, bb.Period, bb.StdDeviation, bb.StdDeviation, talib.EMA)
	bb.UpperBand, bb.MiddleBand, bb.LowerBand = upper[bb.Period:], mid[bb.Period:], lower[bb.Period:]

	bb.Time = dataframe.Time[bb.Period:]
}

func (bb bollingerBands) Metrics() []plot.IndicatorMetric {
	return []plot.IndicatorMetric{
		{
			Style:  "line",
			Color:  bb.UpDnBandColor,
			Values: bb.UpperBand,
			Time:   bb.Time,
		},
		{
			Style:  "line",
			Color:  bb.MidBandColor,
			Values: bb.MiddleBand,
			Time:   bb.Time,
		},
		{
			Style:  "line",
			Color:  bb.UpDnBandColor,
			Values: bb.LowerBand,
			Time:   bb.Time,
		},
	}
}
