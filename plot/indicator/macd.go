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

func (e *macd) Load(df *model.Dataframe) {
	warmup := e.Slow + e.Signal
	e.ValuesMACD, e.ValuesMACDSignal, e.ValuesMACDHist = talib.Macd(df.Close, e.Fast, e.Slow, e.Signal)
	e.Time = df.Time[warmup:]
	e.ValuesMACD = e.ValuesMACD[warmup:]
	e.ValuesMACDSignal = e.ValuesMACDSignal[warmup:]
	e.ValuesMACDHist = e.ValuesMACDHist[warmup:]
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
			Style:  "bar",
			Values: e.ValuesMACDHist,
			Time:   e.Time,
		},
	}
}
