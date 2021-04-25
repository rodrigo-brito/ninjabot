package main

import (
	"context"
	"fmt"
	"log"
	"os"
)

type Example struct{}

func (e Example) Init(settings Settings) {}

func (e Example) Timeframe() string {
	return "1m"
}

func (e Example) WarmupPeriod() int {
	return 10
}

func (e Example) Indicators(dataframe *Dataframe) {
	dataframe.Metadata["rsi"] = dataframe.Close
}

func (e Example) OnCandle(dataframe *Dataframe, broker Broker) {
	fmt.Println(dataframe.Time, dataframe.Close[0])
}

func main() {
	var (
		apiKey    = os.Getenv("TEST_KEY")
		secretKey = os.Getenv("TEST_SECRET")
		ctx       = context.Background()
	)

	binance := NewBinance(apiKey, secretKey)
	strategy := Example{}
	bot := NewBot(Settings{
		Pairs: []string{"BTCUSDT"},
	}, binance, strategy)

	err := bot.Run(ctx)
	if err != nil {
		log.Fatalln(err)
	}
}
