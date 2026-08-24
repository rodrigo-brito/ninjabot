package exchange

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/testdata/mocks"
)

// captureStdout runs fn with os.Stdout redirected and returns what it printed.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	previous := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = previous }()

	fn()
	require.NoError(t, writer.Close())

	var buffer bytes.Buffer
	_, err = io.Copy(&buffer, reader)
	require.NoError(t, err)

	return buffer.String()
}

func TestPaperWalletPairs(t *testing.T) {
	wallet := NewPaperWallet(context.Background(), "USDT",
		WithPaperAsset("USDT", 1000),
		WithPaperAsset("BTC", 1),
	)

	require.ElementsMatch(t, []string{"USDT", "BTC"}, wallet.Pairs())
}

func TestPaperWalletAccount(t *testing.T) {
	wallet := NewPaperWallet(context.Background(), "USDT", WithPaperAsset("USDT", 1000))
	wallet.assets["BTC"] = &assetInfo{Free: 1.5, Lock: 0.5}

	t.Run("Account lists every balance", func(t *testing.T) {
		account, err := wallet.Account()

		require.NoError(t, err)
		require.Len(t, account.Balances, 2)

		btc, usdt := account.Balance("BTC", "USDT")
		require.Equal(t, 1.5, btc.Free)
		require.Equal(t, 0.5, btc.Lock)
		require.Equal(t, 1000.0, usdt.Free)
	})

	t.Run("Position sums the free and locked amounts", func(t *testing.T) {
		asset, quote, err := wallet.Position("BTCUSDT")

		require.NoError(t, err)
		require.Equal(t, 2.0, asset)
		require.Equal(t, 1000.0, quote)
	})
}

func TestPaperWalletEquityTracking(t *testing.T) {
	wallet := NewPaperWallet(context.Background(), "USDT", WithPaperAsset("USDT", 1000))

	candle := model.Candle{
		Pair:     "BTCUSDT",
		Time:     time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		Close:    100,
		High:     110,
		Low:      90,
		Complete: true,
	}
	wallet.OnCandle(candle)

	require.Len(t, wallet.EquityValues(), 1)
	require.Equal(t, 1000.0, wallet.EquityValues()[0].Value)
	require.Len(t, wallet.AssetValues("USDT"), 1)
}

func TestPaperWalletSummary(t *testing.T) {
	newWalletWithHistory := func(t *testing.T) *PaperWallet {
		t.Helper()

		wallet := NewPaperWallet(context.Background(), "USDT", WithPaperAsset("USDT", 1000))
		start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

		for i, close := range []float64{100.0, 110.0} {
			wallet.OnCandle(model.Candle{
				Pair:     "BTCUSDT",
				Time:     start.Add(time.Duration(i) * time.Hour),
				Close:    close,
				High:     close,
				Low:      close,
				Complete: true,
			})
		}

		return wallet
	}

	t.Run("reports the result of a long position", func(t *testing.T) {
		wallet := newWalletWithHistory(t)
		_, err := wallet.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 1)
		require.NoError(t, err)

		output := captureStdout(t, wallet.Summary)

		require.Contains(t, output, "----- FINAL WALLET -----")
		require.Contains(t, output, "START PORTFOLIO     = 1000.00 USDT")
		require.Contains(t, output, "MARKET CHANGE (B&H)")
		require.Contains(t, output, "MAX DRAWDOWN")
		require.Contains(t, output, "BTCUSDT")
	})

	t.Run("values an open short position", func(t *testing.T) {
		wallet := newWalletWithHistory(t)
		_, err := wallet.CreateOrderMarket(model.SideTypeSell, "BTCUSDT", 1)
		require.NoError(t, err)

		output := captureStdout(t, wallet.Summary)

		require.Contains(t, output, "-1.0000 BTC")
	})

	t.Run("skips pairs without a balance", func(t *testing.T) {
		wallet := newWalletWithHistory(t)
		wallet.lastCandle["ETHUSDT"] = model.Candle{Pair: "ETHUSDT", Close: 10}
		wallet.fistCandle["ETHUSDT"] = model.Candle{Pair: "ETHUSDT", Close: 10}

		output := captureStdout(t, wallet.Summary)

		require.NotContains(t, output, "ETH =")
	})
}

func TestPaperWalletCreateOrderMarketQuote(t *testing.T) {
	newWallet := func(t *testing.T) *PaperWallet {
		t.Helper()

		wallet := NewPaperWallet(context.Background(), "USDT", WithPaperAsset("USDT", 1000))
		wallet.lastCandle["BTCUSDT"] = model.Candle{Pair: "BTCUSDT", Close: 100}
		return wallet
	}

	t.Run("converts the quote amount into a quantity", func(t *testing.T) {
		wallet := newWallet(t)

		order, err := wallet.CreateOrderMarketQuote(model.SideTypeBuy, "BTCUSDT", 500)

		require.NoError(t, err)
		require.Equal(t, 5.0, order.Quantity)
		require.Equal(t, 100.0, order.Price)
		require.Equal(t, model.OrderStatusTypeFilled, order.Status)
	})

	t.Run("rejects an amount the wallet cannot afford", func(t *testing.T) {
		wallet := newWallet(t)

		_, err := wallet.CreateOrderMarketQuote(model.SideTypeBuy, "BTCUSDT", 5000)

		require.ErrorIs(t, err, ErrInsufficientFunds)
	})
}

func TestPaperWalletDataFeed(t *testing.T) {
	ctx := context.Background()
	start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := []model.Candle{{Pair: "BTCUSDT", Time: start, Close: 100, Complete: true}}

	feeder := &mocks.Feeder{}
	feeder.On("LastQuote", mock.Anything, "BTCUSDT").Return(100.0, nil)
	feeder.On("CandlesByPeriod", mock.Anything, "BTCUSDT", "1h", mock.Anything, mock.Anything).Return(candles, nil)
	feeder.On("CandlesByLimit", mock.Anything, "BTCUSDT", "1h", 10).Return(candles, nil)

	stream := make(chan model.Candle)
	errs := make(chan error)
	feeder.On("CandlesSubscription", mock.Anything, "BTCUSDT", "1h").Return(stream, errs)

	wallet := NewPaperWallet(ctx, "USDT", WithPaperAsset("USDT", 1000), WithDataFeed(feeder))

	t.Run("LastQuote delegates to the feed", func(t *testing.T) {
		quote, err := wallet.LastQuote(ctx, "BTCUSDT")

		require.NoError(t, err)
		require.Equal(t, 100.0, quote)
	})

	t.Run("CandlesByPeriod delegates to the feed", func(t *testing.T) {
		result, err := wallet.CandlesByPeriod(ctx, "BTCUSDT", "1h", start, start.Add(time.Hour))

		require.NoError(t, err)
		require.Equal(t, candles, result)
	})

	t.Run("CandlesByLimit delegates to the feed", func(t *testing.T) {
		result, err := wallet.CandlesByLimit(ctx, "BTCUSDT", "1h", 10)

		require.NoError(t, err)
		require.Equal(t, candles, result)
	})

	t.Run("CandlesSubscription delegates to the feed", func(t *testing.T) {
		data, failures := wallet.CandlesSubscription(ctx, "BTCUSDT", "1h")

		require.NotNil(t, data)
		require.NotNil(t, failures)
	})

	feeder.AssertExpectations(t)
}
