package indicator

import (
	"fmt"
	"time"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/plot"

	"github.com/markcheno/go-talib"
)

func PPO(fast, slow, signal int, colorPPO, colorPPOSignal, colorPPOHist string) plot.Indicator {
	return &ppo{
		Fast:           fast,
		Slow:           slow,
		Signal:         signal,
		ColorPPO:       colorPPO,
		ColorPPOSignal: colorPPOSignal,
		ColorPPOHist:   colorPPOHist,
	}
}

type ppo struct {
	Fast            int
	Slow            int
	Signal          int
	ColorPPO        string
	ColorPPOSignal  string
	ColorPPOHist    string
	ValuesPPO       model.Series[float64]
	ValuesPPOSignal model.Series[float64]
	ValuesPPOHist   model.Series[float64]
	Time            []time.Time
}

func (e ppo) Warmup() int {
	return e.Slow + e.Signal
}

func (e ppo) Name() string {
	return fmt.Sprintf("PPO(%d, %d, %d)", e.Fast, e.Slow, e.Signal)
}

func (e ppo) Overlay() bool {
	return false
}

func (e *ppo) Load(df *model.Dataframe) {
	warmup := e.Slow + e.Signal
	if len(df.Close) < warmup {
		return
	}

	fastEMA := talib.Ema(df.Close, e.Fast)
	slowEMA := talib.Ema(df.Close, e.Slow)
	values := make(model.Series[float64], len(slowEMA))
	for i := range slowEMA {
		if slowEMA[i] == 0 {
			continue
		}
		values[i] = (fastEMA[i] - slowEMA[i]) / slowEMA[i] * 100
	}

	e.ValuesPPO = values
	e.ValuesPPOSignal = talib.Ema(values, e.Signal)
	hist := make(model.Series[float64], len(values))
	for i := range values {
		hist[i] = values[i] - e.ValuesPPOSignal[i]
	}
	e.ValuesPPOHist = hist

	e.Time = df.Time[warmup:]
	e.ValuesPPO = e.ValuesPPO[warmup:]
	e.ValuesPPOSignal = e.ValuesPPOSignal[warmup:]
	e.ValuesPPOHist = e.ValuesPPOHist[warmup:]
}

func (e ppo) Metrics() []plot.IndicatorMetric {
	return []plot.IndicatorMetric{
		{
			Color:  e.ColorPPO,
			Name:   "PPO",
			Style:  "line",
			Values: e.ValuesPPO,
			Time:   e.Time,
		},
		{
			Color:  e.ColorPPOSignal,
			Name:   "PPOSignal",
			Style:  "line",
			Values: e.ValuesPPOSignal,
			Time:   e.Time,
		},
		{
			Color:  e.ColorPPOHist,
			Name:   "PPOHist",
			Style:  "bar",
			Values: e.ValuesPPOHist,
			Time:   e.Time,
		},
	}
}
