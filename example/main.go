package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/markcheno/go-talib"
	"github.com/rodrigo-brito/ninjabot"
)

type Example struct{}

func (e Example) Init(settings ninjabot.Settings) {}

func (e Example) Timeframe() string {
	return "1m"
}

func (e Example) WarmupPeriod() int {
	return 14
}

func (e Example) Indicators(dataframe *ninjabot.Dataframe) {
	dataframe.Metadata["rsi"] = talib.Rsi(dataframe.Close, 14)
	dataframe.Metadata["ema"] = talib.Ema(dataframe.Close, 9)
}

func (e Example) OnCandle(dataframe *ninjabot.Dataframe, broker ninjabot.Broker) {
	fmt.Println("New Candle = ", dataframe.LastUpdate, ninjabot.Last(dataframe.Close, 0))

	if ninjabot.Last(dataframe.Metadata["rsi"], 0) < 30 {
		broker.OrderMarket(ninjabot.BuyOrder, dataframe.Pair, 1)
	}

	if ninjabot.Last(dataframe.Metadata["rsi"], 0) > 70 {
		broker.OrderMarket(ninjabot.SellOrder, dataframe.Pair, 1)
	}
}

func main() {
	var (
		apiKey    = os.Getenv("API_KEY")
		secretKey = os.Getenv("API_SECRET")
		ctx       = context.Background()
	)

	settings := ninjabot.Settings{
		Pairs: []string{
			"BTCUSDT",
		},
	}
	binance := ninjabot.NewBinance(apiKey, secretKey)
	strategy := Example{}
	bot := ninjabot.NewBot(settings, binance, strategy)

	err := bot.Run(ctx)
	if err != nil {
		log.Fatalln(err)
	}
}
