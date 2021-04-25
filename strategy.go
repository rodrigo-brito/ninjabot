package ninjabot

type Strategy interface {
	Init(settings Settings)
	Timeframe() string
	WarmupPeriod() int
	Indicators(dataframe *Dataframe)
	OnCandle(dataframe *Dataframe, broker Broker)
}

type strategyController struct {
	strategy  Strategy
	dataframe *Dataframe
	broker    Broker
	started   bool
}

func NewStrategyController(settings Settings, strategy Strategy, broker Broker) *strategyController {
	strategy.Init(settings)
	dataframe := &Dataframe{
		Metadata: make(map[string][]float64),
	}

	return &strategyController{
		dataframe: dataframe,
		strategy:  strategy,
		broker:    broker,
	}
}

func (s *strategyController) Start() {
	s.started = true
}

func (s *strategyController) OnCandle(candle Candle) {
	s.dataframe.Close = append(s.dataframe.Close, candle.Close)
	s.dataframe.Open = append(s.dataframe.Open, candle.Open)
	s.dataframe.High = append(s.dataframe.High, candle.High)
	s.dataframe.Low = append(s.dataframe.Low, candle.Low)
	s.dataframe.Volume = append(s.dataframe.Volume, candle.Volume)
	s.dataframe.Time = append(s.dataframe.Time, candle.Time)
	s.dataframe.LastUpdate = candle.Time

	if len(s.dataframe.Close) > s.strategy.WarmupPeriod() {
		s.strategy.Indicators(s.dataframe)
		if s.started {
			s.strategy.OnCandle(s.dataframe, s.broker)
		}
	}
}
