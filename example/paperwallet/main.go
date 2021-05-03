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
}

func (e Example) OnCandle(dataframe *model.Dataframe, broker exchange.Broker) {
	fmt.Println("New Candle = ", dataframe.Pair, dataframe.LastUpdate, model.Last(dataframe.Close, 0))

	broker.OrderMarket(model.SideTypeBuy, dataframe.Pair, 100.0/model.Last(dataframe.Close, 0))
	if model.Last(dataframe.Metadata["rsi"], 0) < 30 {
		broker.OrderMarket(model.SideTypeBuy, dataframe.Pair, 1)
	}

	if model.Last(dataframe.Metadata["rsi"], 0) > 70 {
		broker.OrderMarket(model.SideTypeSell, dataframe.Pair, 1)
	}
}

func main() {
	var (
		ctx             = context.Background()
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

	// Use binance for realtime data feed
	binance, err := exchange.NewBinance(ctx)
	if err != nil {
		log.Fatal(err)
	}

	notifier := notification.NewTelegram(telegramID, telegramKey, telegramChannel)
	paperWallet := exchange.NewPaperWallet(
		ctx,
		exchange.WithPaperFee(0.001, 0.001),
		exchange.WithPaperAsset("USDT", 150),
		exchange.WithDataSource(binance),
	)

	strategy := Example{}
	bot, err := ninjabot.NewBot(
		ctx,
		settings,
		paperWallet,
		strategy,
		ninjabot.WithNotifier(notifier),
	)
	if err != nil {
		log.Fatalln(err)
	}

	bot.SubscribeDataFeed(paperWallet.OnCandle, false)

	err = bot.Run(ctx)
	if err != nil {
		log.Fatalln(err)
	}
}
