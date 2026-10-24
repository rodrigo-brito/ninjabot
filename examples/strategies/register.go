package strategies

import "github.com/rodrigo-brito/ninjabot/strategy"

// The example strategies are available by name to the ninjabot CLI and to
// any program that imports this package.
func init() {
	strategy.RegisterParams("emacross", "EMA/SMA crossover: all in on the cross up, all out on the cross down",
		func(p CrossEMAParams) (strategy.Strategy, error) { return NewCrossEMA(p) })
	strategy.RegisterParams("turtle", "Turtle breakout: buy the highest close, sell the lowest close",
		func(p TurtleParams) (strategy.Strategy, error) { return NewTurtle(p) })
	strategy.RegisterParams("ocosell", "Stochastic crossover entries protected by an OCO take profit / stop loss",
		func(p OCOSellParams) (strategy.Strategy, error) { return NewOCOSell(p) })
	strategy.RegisterParams("trailing", "EMA/SMA crossover entries with a trailing stop exit on partial candles",
		func(p TrailingParams) (strategy.Strategy, error) { return NewTrailingWithParams(p) })
}
