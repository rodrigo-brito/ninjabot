package ui

import (
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/strategy"
)

// StreamIndicator is a CustomIndicator computed incrementally: the chart
// feeds every closed candle of a pair to its own stream and keeps the
// points, instead of reloading the whole history on every candle. The
// indicators of the indicator subpackage implement it.
type StreamIndicator interface {
	CustomIndicator
	// NewStream returns the computation of one pair. It receives every
	// closed candle and returns one value per metric of Metrics(), in order.
	// The first Warmup() values are not plotted.
	NewStream() func(model.Candle) []float64
}

// streamState is a StreamIndicator running for one pair.
type streamState struct {
	update func(model.Candle) []float64
	seen   int
	points [][]Point // one series per metric
}

// plotted are the indicators of a strategy received through OnIndicators,
// accumulated for one pair.
type plotted struct {
	observations int
	groups       []Indicator
}

// updateStreams feeds a closed candle to the stream indicators of its pair.
// The caller holds the lock.
func (c *Chart) updateStreams(candle model.Candle) {
	states := c.streams[candle.Pair]
	if states == nil {
		states = make([]*streamState, len(c.indicators))
		for i, indicator := range c.indicators {
			if s, ok := indicator.(StreamIndicator); ok {
				states[i] = &streamState{update: s.NewStream(), points: make([][]Point, len(s.Metrics()))}
			}
		}
		c.streams[candle.Pair] = states
	}

	for i, state := range states {
		if state == nil {
			continue
		}
		values := state.update(candle)
		state.seen++
		if state.seen <= c.indicators[i].Warmup() {
			continue
		}
		for j := range state.points {
			if j < len(values) {
				state.points[j] = appendPoint(state.points[j], Point{Time: candle.Time.Unix(), Value: values[j]})
			}
		}
	}
}

// appendPoint appends p, or replaces the last point when it has the same
// time (a candle received twice).
func appendPoint(points []Point, p Point) []Point {
	if n := len(points); n > 0 && points[n-1].Time == p.Time {
		points[n-1] = p
		return points
	}
	return append(points, p)
}

// OnIndicators receives the indicators of the strategy after each closed
// candle (see strategy.IndicatorObserver) and keeps their last values. The
// dashboard then shows exactly what the strategy computed, at a constant
// cost per candle.
func (c *Chart) OnIndicators(candle model.Candle, indicators []strategy.ChartIndicator) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.observing = true
	state := c.plotted[candle.Pair]
	if state == nil {
		state = &plotted{}
		c.plotted[candle.Pair] = state
	}

	// the first observation is the candle that completes the warmup of
	// the strategy; plotting starts at the warmup of each indicator
	warmup := 0
	if c.strategy != nil {
		warmup = c.strategy.WarmupPeriod()
	}
	index := warmup - 1 + state.observations
	state.observations++

	for g, indicator := range indicators {
		if g >= len(state.groups) {
			group := Indicator{Name: indicator.GroupName, Overlay: indicator.Overlay}
			for _, metric := range indicator.Metrics {
				group.Metrics = append(group.Metrics, Metric{Name: metric.Name, Color: metric.Color,
					Style: string(metric.Style), Points: make([]Point, 0)})
			}
			state.groups = append(state.groups, group)
		}
		if index < max(indicator.Warmup, warmup) {
			continue
		}
		group := &state.groups[g]
		for m, metric := range indicator.Metrics {
			if m >= len(group.Metrics) || len(metric.Values) == 0 {
				continue
			}
			group.Metrics[m].Points = appendPoint(group.Metrics[m].Points,
				Point{Time: candle.Time.Unix(), Value: metric.Values.Last(0)})
		}
	}
}
