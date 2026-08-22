package ninjabot

import (
	"context"
	"testing"

	"github.com/rodrigo-brito/ninjabot/strategy"

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
	return 10
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
	require.InDelta(t, quote, 22930.9622, 0.001)

	results := bot.orderController.Results["BTCUSDT"]
	require.InDelta(t, 5340.224, results.Profit(), 0.001)
	require.Len(t, results.Win(), 5)
	require.Len(t, results.Lose(), 3)

	results = bot.orderController.Results["ETHUSDT"]
	require.InDelta(t, 7590.7381, results.Profit(), 0.001)
	require.Len(t, results.Win(), 7)
	require.Len(t, results.Lose(), 9)

	bot.Summary()
}

// runBTCBacktest replays the BTCUSDT sample with the given paper wallet fees
// and returns the realized profit and the final quote balance.
func runBTCBacktest(t *testing.T, maker, taker float64) (profit, quote float64, trades int) {
	t.Helper()
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
	)
	require.NoError(t, err)

	paperWallet := exchange.NewPaperWallet(
		ctx,
		"USDT",
		exchange.WithPaperAsset("USDT", 10000),
		exchange.WithPaperFee(maker, taker),
		exchange.WithDataFeed(csvFeed),
	)

	bot, err := NewBot(ctx, Settings{Pairs: []string{"BTCUSDT"}},
		paperWallet,
		strategy,
		WithStorage(storage),
		WithBacktest(paperWallet),
		WithLogLevel(log.ErrorLevel),
	)
	require.NoError(t, err)
	require.NoError(t, bot.Run(ctx))

	_, quote, err = bot.paperWallet.Position("BTCUSDT")
	require.NoError(t, err)

	results := bot.orderController.Results["BTCUSDT"]
	return results.Profit(), quote, len(results.Win()) + len(results.Lose())
}

func TestMarketOrderWithFee(t *testing.T) {
	profit, quote, trades := runBTCBacktest(t, 0, 0)
	feeProfit, feeQuote, feeTrades := runBTCBacktest(t, 0.001, 0.001)

	// the same trades are taken, they just yield less
	require.Equal(t, trades, feeTrades)
	require.Less(t, feeProfit, profit)
	require.Less(t, feeQuote, quote)

	// the profit reported by the order controller is net of fees, so it has to
	// match what the wallet actually holds at the end of the simulation
	require.InDelta(t, 10000+profit, quote, 1e-6)
	require.InDelta(t, 10000+feeProfit, feeQuote, 1e-6)
}
