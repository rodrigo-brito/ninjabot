package ninjabot

import (
	"context"
	"github.com/rodrigo-brito/ninjabot/strategy"
	"testing"

	"github.com/markcheno/go-talib"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/service"
	"github.com/rodrigo-brito/ninjabot/storage"
)

type fakeStrategy struct{}

func (e fakeStrategy) Timeframe() string {
	return "1d"
}

func (e fakeStrategy) WarmupPeriod() int {
	return 9
}

func (e fakeStrategy) Indicators(df *Dataframe) []strategy.ChartIndicator {
	df.Metadata["ema9"] = talib.Ema(df.Close, 9)
	return nil
}

func (e *fakeStrategy) OnCandle(df *Dataframe, broker service.Broker) {
	closePrice := df.Close.Last(0)
	assetPosition, quotePosition, err := broker.Position(df.Pair)
	if err != nil {
		log.Error(err)
	}

	if quotePosition > 0 && df.Close.Crossover(df.Metadata["ema9"]) {
		_, err := broker.CreateOrderMarket(SideTypeBuy, df.Pair, quotePosition/closePrice*0.5)
		if err != nil {
			log.Fatal(err)
		}
	}

	if assetPosition > 0 &&
		df.Close.Crossunder(df.Metadata["ema9"]) {
		_, err := broker.CreateOrderMarket(SideTypeSell, df.Pair, assetPosition)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func TestMarketOrder(t *testing.T) {
	ctx := context.Background()

	storage, err := storage.FromMemory()
	require.NoError(t, err)

	strategy := new(fakeStrategy)
	csvFeed, err := exchange.NewCSVFeed(
		strategy.Timeframe(),
		exchange.PairFeed{
			Pair:      "BTCUSDT",
			File:      "testdata/btc-1h.csv",
			Timeframe: "1h",
		},
		exchange.PairFeed{
			Pair:      "ETHUSDT",
			File:      "testdata/eth-1h.csv",
			Timeframe: "1h",
		},
	)
	require.NoError(t, err)

	paperWallet := exchange.NewPaperWallet(
		ctx,
		"USDT",
		exchange.WithPaperAsset("USDT", 10000),
		exchange.WithDataFeed(csvFeed),
	)

	bot, err := NewBot(ctx, Settings{
		Pairs: []string{
			"BTCUSDT",
			"ETHUSDT",
		},
	},
		paperWallet,
		strategy,
		WithStorage(storage),
		WithBacktest(paperWallet),
		WithLogLevel(log.ErrorLevel),
	)
	require.NoError(t, err)
	require.NoError(t, bot.Run(ctx))

	assets, quote, err := bot.paperWallet.Position("BTCUSDT")
	require.NoError(t, err)
	require.Equal(t, assets, 0.0)
	require.InDelta(t, quote, 26694.6741, 0.001)

	results := bot.orderController.Results["BTCUSDT"]
	require.InDelta(t, 7424.3705, results.Profit(), 0.001)
	require.Len(t, results.Win, 6)
	require.Len(t, results.Lose, 11)

	results = bot.orderController.Results["ETHUSDT"]
	require.InDelta(t, 9270.3036, results.Profit(), 0.001)
	require.Len(t, results.Win, 9)
	require.Len(t, results.Lose, 8)

	bot.Summary()
}
