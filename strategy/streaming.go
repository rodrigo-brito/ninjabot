package strategy

import (
	"fmt"

	"github.com/rodrigo-brito/ninjabot/indicator/stream"
	"github.com/rodrigo-brito/ninjabot/model"
)

// StreamingStrategy is a strategy whose indicators are incremental. Instead
// of recomputing every indicator over a window on each candle (Indicators),
// it registers them once per pair in Setup; the controller then updates each
// one in constant time and exposes its values in df.Metadata under the
// registered name, computed over the whole history.
//
// The controller never calls Indicators on a streaming strategy, which may
// return nil.
type StreamingStrategy interface {
	Strategy
	// Setup registers the indicators of a pair. It is called once per pair,
	// before the first candle.
	Setup(pair string, set *IndicatorSet)
}

// MetricSpec describes how a registered indicator value is plotted.
type MetricSpec struct {
	// Key is the name the value was registered under (for Multi indicators,
	// prefix_output).
	Key   string
	Name  string
	Color string
	Style MetricStyle // default: line
}

// ChartSpec groups plotted values, like a ChartIndicator does for legacy
// strategies.
type ChartSpec struct {
	GroupName string
	Overlay   bool
	Warmup    int
	Metrics   []MetricSpec
}

type indicatorEntry struct {
	names []string
	line  stream.Line
	src   stream.Source
	bar   stream.Bar
	multi stream.Multi
	// series are the values of the outputs, kept here so the controller
	// appends to them without looking them up in df.Metadata
	series []model.Series[float64]
}

// IndicatorSet holds the incremental indicators of a pair, in registration
// order.
type IndicatorSet struct {
	entries []indicatorEntry
	names   map[string]bool
	plots   []ChartSpec
}

func newIndicatorSet() *IndicatorSet {
	return &IndicatorSet{names: make(map[string]bool)}
}

func (s *IndicatorSet) register(entry indicatorEntry) {
	for _, name := range entry.names {
		if s.names[name] {
			panic(fmt.Sprintf("strategy: indicator %q registered twice", name))
		}
		s.names[name] = true
	}
	s.entries = append(s.entries, entry)
}

// Line registers an indicator over one value per candle, read with src.
func (s *IndicatorSet) Line(name string, l stream.Line, src stream.Source) {
	s.register(indicatorEntry{names: []string{name}, line: l, src: src})
}

// Bar registers an indicator over whole candles.
func (s *IndicatorSet) Bar(name string, b stream.Bar) {
	s.register(indicatorEntry{names: []string{name}, bar: b})
}

// Multi registers an indicator with several outputs. Each output is exposed
// as prefix_output, e.g. macd_signal.
func (s *IndicatorSet) Multi(prefix string, m stream.Multi) {
	outputs := m.Outputs()
	names := make([]string, len(outputs))
	for i, output := range outputs {
		names[i] = prefix + "_" + output
	}
	s.register(indicatorEntry{names: names, multi: m})
}

// Plot declares how registered values are drawn on the dashboard.
func (s *IndicatorSet) Plot(spec ChartSpec) {
	s.plots = append(s.plots, spec)
}

// Names returns the registered value names, in order.
func (s *IndicatorSet) Names() []string {
	names := make([]string, 0, len(s.names))
	for _, entry := range s.entries {
		names = append(names, entry.names...)
	}
	return names
}

// update feeds a candle to every indicator, committing it (Update) or
// previewing it, and writes the values at the last row of df: appended when
// the row is new, overwritten otherwise.
func (s *IndicatorSet) update(df *model.Dataframe, candle *model.Candle, preview bool) {
	rows := len(df.Close)
	for e := range s.entries {
		entry := &s.entries[e]
		if entry.series == nil {
			entry.series = make([]model.Series[float64], len(entry.names))
			for i, name := range entry.names {
				entry.series[i] = df.Metadata[name]
			}
		}

		switch {
		case entry.line != nil:
			v := entry.src(candle)
			if preview {
				v = entry.line.Preview(v)
			} else {
				v = entry.line.Update(v)
			}
			s.write(df, entry, 0, v, rows)
		case entry.bar != nil:
			var v float64
			if preview {
				v = entry.bar.Preview(candle)
			} else {
				v = entry.bar.Update(candle)
			}
			s.write(df, entry, 0, v, rows)
		default:
			var values []float64
			if preview {
				values = entry.multi.Preview(candle)
			} else {
				values = entry.multi.Update(candle)
			}
			for i := range entry.names {
				s.write(df, entry, i, values[i], rows)
			}
		}
	}
}

// write sets the value of output i at the last row of df.
func (s *IndicatorSet) write(df *model.Dataframe, entry *indicatorEntry, i int, value float64, rows int) {
	series := entry.series[i]
	if len(series) < rows {
		series = append(series, value)
		entry.series[i] = series
		df.Metadata[entry.names[i]] = series
		return
	}
	series[len(series)-1] = value
}

// resync reloads the series from df after the controller compacted it.
func (s *IndicatorSet) resync(df *model.Dataframe) {
	for e := range s.entries {
		for i, name := range s.entries[e].names {
			if s.entries[e].series != nil {
				s.entries[e].series[i] = df.Metadata[name]
			}
		}
	}
}

// chart builds the plotted indicators from the values in df.
func (s *IndicatorSet) chart(df *model.Dataframe) []ChartIndicator {
	indicators := make([]ChartIndicator, 0, len(s.plots))
	for _, plot := range s.plots {
		indicator := ChartIndicator{
			Time:      df.Time,
			Overlay:   plot.Overlay,
			GroupName: plot.GroupName,
			Warmup:    plot.Warmup,
			Metrics:   make([]IndicatorMetric, 0, len(plot.Metrics)),
		}
		for _, metric := range plot.Metrics {
			style := metric.Style
			if style == "" {
				style = StyleLine
			}
			indicator.Metrics = append(indicator.Metrics, IndicatorMetric{
				Name:   metric.Name,
				Color:  metric.Color,
				Style:  style,
				Values: df.Metadata[metric.Key],
			})
		}
		indicators = append(indicators, indicator)
	}
	return indicators
}
