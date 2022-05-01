package utils

import (
	"github.com/rodrigo-brito/ninjabot/model"
	"sort"
)

type HeikinAshiCandle struct {
	Open      float64
	Close     float64
	High      float64
	Low       float64
	IsBullish bool
}

type HeikinAshi struct {
	PreviousHACandle HeikinAshiCandle
}

func NewHeikinAshi() *HeikinAshi {
	return &HeikinAshi{}
}

func (ha *HeikinAshi) CalculateHeikinAshi(c model.Candle) HeikinAshiCandle {
	var hkCandle HeikinAshiCandle

	highValues := []float64{c.High, c.Open, c.Close}
	sort.Float64s(highValues)

	lowValues := []float64{c.Low, c.Open, c.Close}
	sort.Float64s(lowValues)

	openValue := ha.PreviousHACandle.Open
	closeValue := ha.PreviousHACandle.Close

	// First HA candle is calculated using current candle
	if (ha.PreviousHACandle == HeikinAshiCandle{}) {
		openValue = c.Open
		closeValue = c.Close
	}

	hkCandle.Open = (openValue + closeValue) / 2
	hkCandle.High = highValues[2]
	hkCandle.Close = (c.Open + c.High + c.Low + c.Close) / 4
	hkCandle.Low = lowValues[0]

	if hkCandle.Open < hkCandle.Close {
		hkCandle.IsBullish = true
	}

	ha.PreviousHACandle = hkCandle

	return hkCandle
}
