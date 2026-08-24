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

// ocoStrategy opens a long with a market order every few candles and brackets
// it with an OCO sell: a target 3% above and a stop 2% below the entry.
type ocoStrategy struct {
	candles int
}

func (ocoStrategy) Timeframe() string                               { return "1h" }
func (ocoStrategy) WarmupPeriod() int                               { return 1 }
func (ocoStrategy) Indicators(*Dataframe) []strategy.ChartIndicator { return nil }

func (s *ocoStrategy) OnCandle(df *Dataframe, broker service.Broker) {
	s.candles++
	asset, quote, err := broker.Position(df.Pair)
	if err != nil {
		log.Fatal(err)
	}

	if asset > 0 || s.candles%50 != 0 {
		return
	}

	closePrice := df.Close.Last(0)
	size := quote * 0.3 / closePrice
	if _, err := broker.CreateOrderMarket(SideTypeBuy, df.Pair, size); err != nil {
		log.Fatal(err)
	}

	_, err = broker.CreateOrderOCO(SideTypeSell, df.Pair, size, closePrice*1.03, closePrice*0.98, closePrice*0.979)
	if err != nil {
		log.Fatal(err)
	}
}

// TestOCOBacktest checks that the fills of pending orders are accounted as the
// candles go by: every trade result must match the bracket that closed it.
func TestOCOBacktest(t *testing.T) {
	ctx := context.Background()

	storage, err := storage.FromMemory()
	require.NoError(t, err)

	strategy := new(ocoStrategy)
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
		exchange.WithPaperFee(0.001, 0.001),
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

	// every filled sell closes one bracket and produces exactly one result
	orders, err := storage.Orders()
	require.NoError(t, err)
	filledSells := 0
	for _, order := range orders {
		require.NotEqual(t, OrderStatusTypeNew, order.Status, order.String()) // none is left pending
		if order.Side == SideTypeSell && order.Status == OrderStatusTypeFilled {
			filledSells++
		}
	}
	require.Greater(t, filledSells, 10)

	results := bot.orderController.Results["BTCUSDT"]
	require.NotNil(t, results)
	require.NotEmpty(t, results.Win())
	require.NotEmpty(t, results.Lose())
	require.Equal(t, filledSells, len(results.Win())+len(results.Lose()))

	// a win is the 3% target minus two 0.1% fees, a loss the 2% stop plus them
	for _, profit := range results.WinPercent() {
		require.InDelta(t, 0.028, profit, 0.0005)
	}
	for _, profit := range results.LosePercent() {
		require.InDelta(t, -0.022, profit, 0.0005)
	}

	// the trade results add up to what the wallet made
	asset, quote, err := paperWallet.Position("BTCUSDT")
	require.NoError(t, err)
	require.Equal(t, 0.0, asset)
	require.InDelta(t, 10000+results.Profit(), quote, 0.001)
}
