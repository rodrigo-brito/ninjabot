package strategy

import "github.com/rodrigo-brito/ninjabot/model"

// IndicatorObserver receives the plotted indicators of a strategy after each
// closed candle, as the strategy computed them. The series are shared with
// the controller and must not be modified or retained beyond the call.
type IndicatorObserver interface {
	OnIndicators(candle model.Candle, indicators []ChartIndicator)
}

// Subscribe registers an observer of the plotted indicators.
func (s *Controller) Subscribe(observer IndicatorObserver) {
	s.observers = append(s.observers, observer)
}

func (s *Controller) notify(candle model.Candle, charts []ChartIndicator) {
	if len(charts) == 0 {
		return
	}
	for _, observer := range s.observers {
		observer.OnIndicators(candle, charts)
	}
}
