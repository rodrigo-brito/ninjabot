package strategies

import (
	"github.com/rodrigo-brito/ninjabot"
	"github.com/rodrigo-brito/ninjabot/indicator"
	"github.com/rodrigo-brito/ninjabot/service"
	"github.com/rodrigo-brito/ninjabot/strategy"
	"github.com/rodrigo-brito/ninjabot/tools/log"
)

// https://www.investopedia.com/articles/trading/08/turtle-trading.asp
type Turtle struct{}

func (e Turtle) Timeframe() string {
	return "4h"
}

func (e Turtle) WarmupPeriod() int {
	return 40
}

func (e Turtle) Indicators(df *ninjabot.Dataframe) []strategy.ChartIndicator {
	df.Metadata["ema8"] = indicator.EMA(df.Close, 8)
	df.Metadata["sma21"] = indicator.SMA(df.Close, 21)

	return nil
}

func (e *Turtle) OnCandle(df *ninjabot.Dataframe, broker service.Broker) {
	position, ok := broker.Position(df.Pair)
	balances, _ := broker.Account()
	funds := balances.Equity()

	position.CurrentCandle = df.CurrentCandle()

	if !ok && df.Metadata["ema8"].Crossover(df.Metadata["sma21"]) {
		err := broker.OpenPosition(ninjabot.SideTypeBuy, df.Pair, 0.5*funds, 10)
		if err != nil {
			log.Error(err)
		}
		return
	}

	if ok && df.Metadata["ema8"].Crossunder(df.Metadata["sma21"]) {
		_ = broker.ClosePosition(position)
	}
}
