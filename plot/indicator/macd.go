package indicator

import (
	"fmt"
	"time"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/plot"

	"github.com/markcheno/go-talib"
)

func MACD(fast, slow, signal int, colorMACD, colorMACDSignal, colorMACDHist string) plot.Indicator {
	return &macd{
		Fast:            fast,
		Slow:            slow,
		Signal:          signal,
		ColorMACD:       colorMACD,
		ColorMACDSignal: colorMACDSignal,
		ColorMACDHist:   colorMACDHist,
	}
}

type macd struct {
	Fast             int
	Slow             int
	Signal           int
	ColorMACD        string
	ColorMACDSignal  string
	ColorMACDHist    string
	ValuesMACD       model.Series
	ValuesMACDSignal model.Series
	ValuesMACDHist   model.Series
	Time             []time.Time
}

func (e macd) Name() string {
	return fmt.Sprintf("MACD(%d, %d, %d)", e.Fast, e.Slow, e.Signal)
}

func (e macd) Overlay() bool {
	return false
}

func (e *macd) Load(dataframe *model.Dataframe) {
	e.ValuesMACD, e.ValuesMACDSignal, e.ValuesMACDHist = talib.Macd(dataframe.Close, e.Fast, e.Slow, e.Signal)
	e.Time = dataframe.Time
}

func (e macd) Metrics() []plot.IndicatorMetric {
	return []plot.IndicatorMetric{
		{
			Color:  e.ColorMACD,
			Name:   "MACD",
			Style:  "line",
			Values: e.ValuesMACD,
			Time:   e.Time,
		},
		{
			Color:  e.ColorMACDSignal,
			Name:   "MACDSignal",
			Style:  "line",
			Values: e.ValuesMACDSignal,
			Time:   e.Time,
		},
		{
			Color:  e.ColorMACDHist,
			Name:   "MACDHist",
			Style:  "line",
			Values: e.ValuesMACDHist,
			Time:   e.Time,
		},
	}
}
