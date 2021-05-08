package exchange

import (
	"context"
	"testing"

	"github.com/rodrigo-brito/ninjabot/pkg/model"
	"github.com/stretchr/testify/require"
)

func TestPaperWallet_OrderLimit(t *testing.T) {
	t.Run("normal order", func(t *testing.T) {
		wallet := NewPaperWallet(context.Background(), WithPaperAsset("USDT", 100))
		order, err := wallet.OrderLimit(model.SideTypeBuy, "BTCUSDT", 1, 100)
		require.NoError(t, err)

		// create order and lock values
		require.Len(t, wallet.orders, 1)
		require.Equal(t, 1.0, order.Quantity)
		require.Equal(t, 100.0, order.Price)
		require.Equal(t, 0.0, wallet.assets["USDT"].Free)
		require.Equal(t, 100.0, wallet.assets["USDT"].Lock)

		// a new candle should execute order and unlock values
		wallet.OnCandle(model.Candle{Symbol: "BTCUSDT", Close: 100})
		require.Equal(t, model.OrderStatusTypeFilled, wallet.orders[0].Status)
		require.Equal(t, 0.0, wallet.assets["USDT"].Free)
		require.Equal(t, 0.0, wallet.assets["USDT"].Lock)
		require.Equal(t, 1.0, wallet.assets["BTC"].Free)
		require.Equal(t, 0.0, wallet.assets["BTC"].Lock)
		require.Equal(t, 100.0, wallet.avgPrice["BTCUSDT"])

		// try to buy again without funds
		order, err = wallet.OrderLimit(model.SideTypeBuy, "BTCUSDT", 1, 100)
		require.Empty(t, order)
		require.Equal(t, ErrInsufficientFunds, err)

		// try to sell and profit 100 USDT
		order, err = wallet.OrderLimit(model.SideTypeSell, "BTCUSDT", 1, 200)
		require.Len(t, wallet.orders, 2)
		require.Equal(t, 1.0, order.Quantity)
		require.Equal(t, 200.0, order.Price)
		require.Equal(t, 0.0, wallet.assets["USDT"].Free)
		require.Equal(t, 0.0, wallet.assets["USDT"].Lock)
		require.Equal(t, 0.0, wallet.assets["BTC"].Free)
		require.Equal(t, 1.0, wallet.assets["BTC"].Lock)

		// a new candle should execute order and unlock values
		wallet.OnCandle(model.Candle{Symbol: "BTCUSDT", Close: 200})
		require.Equal(t, model.OrderStatusTypeFilled, wallet.orders[1].Status)
		require.Equal(t, 200.0, wallet.assets["USDT"].Free)
		require.Equal(t, 0.0, wallet.assets["USDT"].Lock)
		require.Equal(t, 0.0, wallet.assets["BTC"].Free)
		require.Equal(t, 0.0, wallet.assets["BTC"].Lock)
	})

	t.Run("multiple pending orders", func(t *testing.T) {
		wallet := NewPaperWallet(context.Background(), WithPaperAsset("USDT", 100))
		wallet.lastCandle["BTCUSDT"] = 10

		order, err := wallet.OrderLimit(model.SideTypeBuy, "BTCUSDT", 1, 10)
		require.NoError(t, err)
		require.NotEmpty(t, order)

		require.Equal(t, 90.0, wallet.assets["USDT"].Free)
		require.Equal(t, 10.0, wallet.assets["USDT"].Lock)

		order, err = wallet.OrderLimit(model.SideTypeBuy, "BTCUSDT", 1, 20)
		require.NoError(t, err)
		require.NotEmpty(t, order)

		require.Equal(t, 70.0, wallet.assets["USDT"].Free)
		require.Equal(t, 30.0, wallet.assets["USDT"].Lock)

		order, err = wallet.OrderLimit(model.SideTypeBuy, "BTCUSDT", 1, 50)
		require.NoError(t, err)
		require.NotEmpty(t, order)

		require.Equal(t, 20.0, wallet.assets["USDT"].Free)
		require.Equal(t, 80.0, wallet.assets["USDT"].Lock)

		// should execute two orders and keep one pending
		wallet.OnCandle(model.Candle{Symbol: "BTCUSDT", Close: 40})
		require.Equal(t, 20.0, wallet.assets["USDT"].Free)
		require.Equal(t, 50.0, wallet.assets["USDT"].Lock)
		require.Equal(t, 0.0, wallet.assets["BTC"].Lock)
		require.Equal(t, 2.0, wallet.assets["BTC"].Free)
		require.Equal(t, 15.0, wallet.avgPrice["BTCUSDT"])
		require.Equal(t, model.OrderStatusTypeFilled, wallet.orders[0].Status)
		require.Equal(t, model.OrderStatusTypeFilled, wallet.orders[1].Status)
		require.Equal(t, model.OrderStatusTypeNew, wallet.orders[2].Status)

		// sell all bitcoin position
		order, err = wallet.OrderLimit(model.SideTypeSell, "BTCUSDT", 2, 40)
		require.NoError(t, err)
		require.NotEmpty(t, order)

		require.Equal(t, 20.0, wallet.assets["USDT"].Free)
		require.Equal(t, 50.0, wallet.assets["USDT"].Lock)
		require.Equal(t, 0.0, wallet.assets["BTC"].Free)
		require.Equal(t, 2.0, wallet.assets["BTC"].Lock)

		wallet.OnCandle(model.Candle{Symbol: "BTCUSDT", Close: 40})
		require.Equal(t, 0.0, wallet.assets["BTC"].Free)
		require.Equal(t, 0.0, wallet.assets["BTC"].Lock)
		require.Equal(t, 100.0, wallet.assets["USDT"].Free)
		require.Equal(t, 50.0, wallet.assets["USDT"].Lock)

		// execute old buy position
		wallet.OnCandle(model.Candle{Symbol: "BTCUSDT", Close: 50})
		require.Equal(t, 1.0, wallet.assets["BTC"].Free)
		require.Equal(t, 0.0, wallet.assets["BTC"].Lock)
		require.Equal(t, 100.0, wallet.assets["USDT"].Free)
		require.Equal(t, 0.0, wallet.assets["USDT"].Lock)
		require.Equal(t, 50.0, wallet.avgPrice["BTCUSDT"])
	})
}

func TestPaperWallet_OrderMarket(t *testing.T) {
	wallet := NewPaperWallet(context.Background(), WithPaperAsset("USDT", 100))
	wallet.OnCandle(model.Candle{Symbol: "BTCUSDT", Close: 50})
	order, err := wallet.OrderMarket(model.SideTypeBuy, "BTCUSDT", 1)
	require.NoError(t, err)

	// create buy order
	require.Len(t, wallet.orders, 1)
	require.Equal(t, model.OrderStatusTypeFilled, order.Status)
	require.Equal(t, 1.0, order.Quantity)
	require.Equal(t, 50.0, order.Price)
	require.Equal(t, 50.0, wallet.assets["USDT"].Free)
	require.Equal(t, 0.0, wallet.assets["USDT"].Lock)
	require.Equal(t, 1.0, wallet.assets["BTC"].Free)
	require.Equal(t, 0.0, wallet.assets["BTC"].Lock)
	require.Equal(t, 50.0, wallet.avgPrice["BTCUSDT"])

	// insufficient funds
	order, err = wallet.OrderMarket(model.SideTypeBuy, "BTCUSDT", 100)
	require.Equal(t, ErrInsufficientFunds, err)
	require.Empty(t, order)

	// sell
	wallet.OnCandle(model.Candle{Symbol: "BTCUSDT", Close: 100})
	order, err = wallet.OrderMarket(model.SideTypeSell, "BTCUSDT", 1)
	require.Equal(t, 1.0, order.Quantity)
	require.Equal(t, 100.0, order.Price)
	require.Equal(t, 150.0, wallet.assets["USDT"].Free)
	require.Equal(t, 0.0, wallet.assets["USDT"].Lock)
	require.Equal(t, 0.0, wallet.assets["BTC"].Free)
	require.Equal(t, 0.0, wallet.assets["BTC"].Lock)
	require.Equal(t, 50.0, wallet.avgPrice["BTCUSDT"])
}

func TestPaperWallet_Order(t *testing.T) {
	wallet := NewPaperWallet(context.Background(), WithPaperAsset("USDT", 100))
	expectOrder, err := wallet.OrderMarket(model.SideTypeBuy, "BTCUSDT", 1)
	require.NoError(t, err)
	require.Equal(t, int64(1), expectOrder.ExchangeID)

	order, err := wallet.Order("BTCUSDT", expectOrder.ExchangeID)
	require.NoError(t, err)
	require.Equal(t, expectOrder, order)
}
