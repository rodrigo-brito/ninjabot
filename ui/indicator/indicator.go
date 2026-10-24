// Package indicator holds the custom indicators of the dashboard. They are
// incremental (see ui.StreamIndicator): the chart updates each one once per
// closed candle instead of recomputing it over the whole history.
package indicator

import (
	"fmt"
	"time"

	"github.com/rodrigo-brito/ninjabot/indicator/stream"
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/ui"
)

// metric is the plotting style of one output.
type metric struct {
	name, color, style string
}

// streamIndicator adapts incremental computations to ui.StreamIndicator.
type streamIndicator struct {
	name    string
	overlay bool
	// skip is the number of leading values that are not plotted.
	skip    int
	metrics []metric
	factory func() func(model.Candle) []float64

	// filled by Load, for callers of the legacy CustomIndicator API
	values [][]float64
	times  []time.Time
}

func (s *streamIndicator) Name() string  { return s.name }
func (s *streamIndicator) Overlay() bool { return s.overlay }
func (s *streamIndicator) Warmup() int   { return s.skip }

func (s *streamIndicator) NewStream() func(model.Candle) []float64 { return s.factory() }

// Load computes the indicator over a whole dataframe.
func (s *streamIndicator) Load(df *model.Dataframe) {
	update := s.factory()
	s.values = make([][]float64, len(s.metrics))
	s.times = s.times[:0]
	for i := range df.Close {
		values := update(model.Candle{Time: df.Time[i], Open: df.Open[i], High: df.High[i],
			Low: df.Low[i], Close: df.Close[i], Volume: df.Volume[i], Complete: true})
		if i < s.skip {
			continue
		}
		s.times = append(s.times, df.Time[i])
		for j := range s.values {
			s.values[j] = append(s.values[j], values[j])
		}
	}
}

func (s *streamIndicator) Metrics() []ui.IndicatorMetric {
	metrics := make([]ui.IndicatorMetric, len(s.metrics))
	for i, m := range s.metrics {
		metrics[i] = ui.IndicatorMetric{Name: m.name, Color: m.color, Style: m.style, Time: s.times}
		if i < len(s.values) {
			metrics[i].Values = s.values[i]
		}
	}
	return metrics
}

func bar(newBar func() stream.Bar) func() func(model.Candle) []float64 {
	return func() func(model.Candle) []float64 {
		b := newBar()
		out := make([]float64, 1)
		return func(c model.Candle) []float64 {
			out[0] = b.Update(&c)
			return out
		}
	}
}

func multi(newMulti func() stream.Multi) func() func(model.Candle) []float64 {
	return func() func(model.Candle) []float64 {
		m := newMulti()
		return func(c model.Candle) []float64 { return m.Update(&c) }
	}
}

func closeLine(newLine func() stream.Line) func() func(model.Candle) []float64 {
	return bar(func() stream.Bar { return stream.FromLine(newLine(), stream.Close) })
}

// EMA is the exponential moving average of the close.
func EMA(period int, color string) ui.CustomIndicator {
	return &streamIndicator{
		name: fmt.Sprintf("EMA(%d)", period), overlay: true, skip: period,
		metrics: []metric{{color: color, style: "line"}},
		factory: closeLine(func() stream.Line { return stream.NewEMA(period) }),
	}
}

// SMA is the simple moving average of the close.
func SMA(period int, color string) ui.CustomIndicator {
	return &streamIndicator{
		name: fmt.Sprintf("SMA(%d)", period), overlay: true, skip: period,
		metrics: []metric{{color: color, style: "line"}},
		factory: closeLine(func() stream.Line { return stream.NewSMA(period) }),
	}
}

// RSI is the relative strength index of the close.
func RSI(period int, color string) ui.CustomIndicator {
	return &streamIndicator{
		name: fmt.Sprintf("RSI(%d)", period), skip: period,
		metrics: []metric{{color: color, style: "line"}},
		factory: closeLine(func() stream.Line { return stream.NewRSI(period) }),
	}
}

// CCI is the commodity channel index.
func CCI(period int, color string) ui.CustomIndicator {
	return &streamIndicator{
		name: fmt.Sprintf("CCI(%d)", period), skip: period,
		metrics: []metric{{color: color, style: "line"}},
		factory: bar(func() stream.Bar { return stream.NewCCI(period) }),
	}
}

// WillR is the Williams %R.
func WillR(period int, color string) ui.CustomIndicator {
	return &streamIndicator{
		name: fmt.Sprintf("%%R(%d)", period), skip: period,
		metrics: []metric{{color: color, style: "line"}},
		factory: bar(func() stream.Bar { return stream.NewWillR(period) }),
	}
}

// OBV is the on-balance volume.
func OBV(color string) ui.CustomIndicator {
	return &streamIndicator{
		name:    "OBV",
		metrics: []metric{{color: color, style: "line"}},
		factory: bar(func() stream.Bar { return stream.NewOBV() }),
	}
}

// Spertrend is the SuperTrend, drawn as dots over the price.
func Spertrend(period int, factor float64, color string) ui.CustomIndicator {
	return &streamIndicator{
		name: fmt.Sprintf("SuperTrend(%d,%.1f)", period, factor), overlay: true, skip: period,
		metrics: []metric{{color: color, style: "scatter"}},
		factory: bar(func() stream.Bar { return stream.NewSuperTrend(period, factor) }),
	}
}

// MACD is the moving average convergence divergence of the close.
func MACD(fast, slow, signal int, colorMACD, colorMACDSignal, colorMACDHist string) ui.CustomIndicator {
	return &streamIndicator{
		name: fmt.Sprintf("MACD(%d, %d, %d)", fast, slow, signal), skip: slow + signal,
		metrics: []metric{
			{"MACD", colorMACD, "line"}, {"MACDSignal", colorMACDSignal, "line"}, {"MACDHist", colorMACDHist, "bar"},
		},
		factory: multi(func() stream.Multi { return stream.NewMACD(fast, slow, signal) }),
	}
}

// BollingerBands are the bands around an exponential moving average of the
// close.
func BollingerBands(period int, stdDeviation float64, upDnBandColor, midBandColor string) ui.CustomIndicator {
	return &streamIndicator{
		name: fmt.Sprintf("BB(%d, %.2f)", period, stdDeviation), overlay: true, skip: period,
		metrics: []metric{{"", upDnBandColor, "line"}, {"", midBandColor, "line"}, {"", upDnBandColor, "line"}},
		factory: multi(func() stream.Multi { return stream.NewBBEMA(period, stdDeviation) }),
	}
}

// Stoch is the slow stochastic oscillator.
func Stoch(fastK, slowK, slowD int, colorK, colorD string) ui.CustomIndicator {
	return &streamIndicator{
		name:    fmt.Sprintf("STOCH(%d, %d, %d)", fastK, slowK, slowD),
		metrics: []metric{{"K", colorK, "line"}, {"D", colorD, "line"}},
		factory: multi(func() stream.Multi { return stream.NewStoch(fastK, slowK, slowD) }),
	}
}

// PPO is the percentage price oscillator: the gap between a fast and a slow
// EMA of the close as a percentage of the slow one, its signal line and the
// histogram.
func PPO(fast, slow, signal int, colorPPO, colorPPOSignal, colorPPOHist string) ui.CustomIndicator {
	return &streamIndicator{
		name: fmt.Sprintf("PPO(%d, %d, %d)", fast, slow, signal), skip: slow + signal,
		metrics: []metric{
			{"PPO", colorPPO, "line"}, {"PPOSignal", colorPPOSignal, "line"}, {"PPOHist", colorPPOHist, "bar"},
		},
		factory: func() func(model.Candle) []float64 {
			fastEMA, slowEMA, signalEMA := stream.NewEMA(fast), stream.NewEMA(slow), stream.NewEMA(signal)
			out := make([]float64, 3)
			return func(c model.Candle) []float64 {
				f, s := fastEMA.Update(c.Close), slowEMA.Update(c.Close)
				ppo := 0.0
				if s != 0 {
					ppo = (f - s) / s * 100
				}
				sig := signalEMA.Update(ppo)
				out[0], out[1], out[2] = ppo, sig, ppo-sig
				return out
			}
		},
	}
}
