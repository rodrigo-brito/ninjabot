package indicator

import (
	"fmt"
	"time"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/plot"

	"github.com/markcheno/go-talib"
)

func Spertrend(period int, factor float64, color string) plot.Indicator {
	return &supertrend{
		Period: period,
		Factor: factor,
		Color:  color,
	}
}

type supertrend struct {
	Period         int
	Factor         float64
	Color          string
	Close          model.Series
	BasicUpperBand model.Series
	FinalUpperBand model.Series
	BasicLowerBand model.Series
	FinalLowerBand model.Series
	SuperTrend     model.Series
	Time           []time.Time
}

func (s supertrend) Name() string {
	return fmt.Sprintf("SuperTrend(%d,%.1f)", s.Period, s.Factor)
}

func (s supertrend) Overlay() bool {
	return true
}

func (s *supertrend) Load(df *model.Dataframe) {
	if len(df.Time) < s.Period {
		return
	}

	atr := talib.Atr(df.High, df.Low, df.Close, s.Period)
	s.BasicUpperBand = make([]float64, len(atr))
	s.BasicLowerBand = make([]float64, len(atr))
	s.FinalUpperBand = make([]float64, len(atr))
	s.FinalLowerBand = make([]float64, len(atr))
	s.SuperTrend = make([]float64, len(atr))

	for i := 1; i < len(s.BasicLowerBand); i++ {
		s.BasicUpperBand[i] = (df.High[i]+df.Low[i])/2.0 + atr[i]*s.Factor
		s.BasicLowerBand[i] = (df.High[i]+df.Low[i])/2.0 - atr[i]*s.Factor

		if i == 0 {
			s.FinalUpperBand[i] = s.BasicUpperBand[i]
		} else if s.BasicUpperBand[i] < s.FinalUpperBand[i-1] ||
			df.Close[i-1] > s.FinalUpperBand[i-1] {
			s.FinalUpperBand[i] = s.BasicUpperBand[i]
		} else {
			s.FinalUpperBand[i] = s.FinalUpperBand[i-1]
		}

		if i == 0 || s.BasicLowerBand[i] > s.FinalLowerBand[i-1] ||
			df.Close[i-1] < s.FinalLowerBand[i-1] {
			s.FinalLowerBand[i] = s.BasicLowerBand[i]
		} else {
			s.FinalLowerBand[i] = s.FinalLowerBand[i-1]
		}

		if i == 0 || s.FinalUpperBand[i-1] == s.SuperTrend[i-1] {
			if df.Close[i] > s.FinalUpperBand[i] {
				s.SuperTrend[i] = s.FinalLowerBand[i]
			} else {
				s.SuperTrend[i] = s.FinalUpperBand[i]
			}
		} else {
			if df.Close[i] < s.FinalLowerBand[i] {
				s.SuperTrend[i] = s.FinalUpperBand[i]
			} else {
				s.SuperTrend[i] = s.FinalLowerBand[i]
			}
		}
	}

	s.Time = df.Time[s.Period:]
	s.SuperTrend = s.SuperTrend[s.Period:]

}

func (s supertrend) Metrics() []plot.IndicatorMetric {
	return []plot.IndicatorMetric{
		{
			Style:  "scatter",
			Color:  s.Color,
			Values: s.SuperTrend,
			Time:   s.Time,
		},
	}
}
