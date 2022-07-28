package strategy

import (
	"github.com/rodrigo-brito/ninjabot/model"
	"time"
)

type MetricStyle string

const (
	StyleBar       = "bar"
	StyleScatter   = "scatter"
	StyleLine      = "line"
	StyleHistogram = "histogram"
	StyleWaterfall = "waterfall"
)

type IndicatorMetric struct {
	Name   string
	Color  string
	Style  MetricStyle // default: line
	Values model.Series
}

type ChartIndicator struct {
	Time      []time.Time
	Metrics   []IndicatorMetric
	Overlay   bool
	GroupName string
}
