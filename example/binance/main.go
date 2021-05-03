package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/rodrigo-brito/ninjabot"
	"github.com/rodrigo-brito/ninjabot/pkg/exchange"
	"github.com/rodrigo-brito/ninjabot/pkg/model"
	"github.com/rodrigo-brito/ninjabot/pkg/notification"

	"github.com/markcheno/go-talib"
)

type Example struct{}

func (e Example) Init(settings model.Settings) {}

func (e Example) Timeframe() string {
	return "1m"
}

func (e Example) WarmupPeriod() int {
	return 14
}

func (e Example) Indicators(dataframe *model.Dataframe) {
	dataframe.Metadata["rsi"] = talib.Rsi(dataframe.Close, 14)
	dataframe.Metadata["ema"] = talib.Ema(dataframe.Close, 9)
}

func (e Example) OnCandle(dataframe *model.Dataframe, broker exchange.Broker) {
	fmt.Println("New Candle = ", dataframe.Pair, dataframe.LastUpdate, model.Last(dataframe.Close, 0))

	if model.Last(dataframe.Metadata["rsi"], 0) < 30 &&
		model.Last(dataframe.Metadata["ema"], 0) > model.Last(dataframe.Metadata["ema"], 1) {
		broker.OrderMarket(model.SideTypeBuy, dataframe.Pair, 1)
	}

	if model.Last(dataframe.Metadata["rsi"], 0) > 70 {
		broker.OrderMarket(model.SideTypeSell, dataframe.Pair, 1)
	}
}

func main() {

	var (
		ctx             = context.Background()
		apiKey          = os.Getenv("API_KEY")
		secretKey       = os.Getenv("API_SECRET")
		telegramKey     = os.Getenv("TELEGRAM_KEY")
		telegramID      = os.Getenv("TELEGRAM_ID")
		telegramChannel = os.Getenv("TELEGRAM_CHANNEL")
	)

	settings := model.Settings{
		Pairs: []string{
			"BTCUSDT",
			"ETHUSDT",
		},
	}

	// Initialize your exchange
	binance, err := exchange.NewBinance(ctx, exchange.WithBinanceCredentials(apiKey, secretKey))
	if err != nil {
		log.Fatalln(err)
	}

	// (Optional) Telegram notifier
	notifier := notification.NewTelegram(telegramID, telegramKey, telegramChannel)

	strategy := Example{}
	bot, err := ninjabot.NewBot(settings, binance, strategy, ninjabot.WithNotifier(notifier))
	if err != nil {
		log.Fatalln(err)
	}

	err = bot.Run(ctx)
	if err != nil {
		log.Fatalln(err)
	}
}
