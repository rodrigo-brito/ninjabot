package exchange

import (
	"context"
	"testing"
	"time"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaperWallet_OrderLimit(t *testing.T) {
	t.Run("normal order", func(t *testing.T) {
		wallet := NewPaperWallet(context.Background(), "USDT", WithPaperAsset("USDT", 100))
		order, err := wallet.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 1, 100)
		require.NoError(t, err)

		// create order and lock values
		require.Len(t, wallet.orders, 1)
		require.Equal(t, 1.0, order.Quantity)
		require.Equal(t, 100.0, order.Price)
		require.Equal(t, 0.0, wallet.assets["USDT"].Free)
		require.Equal(t, 100.0, wallet.assets["USDT"].Lock)

		// a new candle should execute order and unlock values
		wallet.OnCandle(model.Candle{Pair: "BTCUSDT", Close: 100})
		require.Equal(t, model.OrderStatusTypeFilled, wallet.orders[0].Status)
		require.Equal(t, 0.0, wallet.assets["USDT"].Free)
		require.Equal(t, 0.0, wallet.assets["USDT"].Lock)
		require.Equal(t, 1.0, wallet.assets["BTC"].Free)
		require.Equal(t, 0.0, wallet.assets["BTC"].Lock)
		require.Equal(t, 100.0, wallet.avgPrice["BTCUSDT"])

		// try to buy again without funds
		order, err = wallet.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 1, 100)
		require.Empty(t, order)
		require.Equal(t, &OrderError{
			Err:      ErrInsufficientFunds,
			Pair:     "USDT",
			Quantity: 100}, err)

		// try to sell and profit 100 USDT
		order, err = wallet.CreateOrderLimit(model.SideTypeSell, "BTCUSDT", 1, 200)
		require.NoError(t, err)
		require.Len(t, wallet.orders, 2)
		require.Equal(t, 1.0, order.Quantity)
		require.Equal(t, 200.0, order.Price)
		require.Equal(t, 0.0, wallet.assets["USDT"].Free)
		require.Equal(t, 0.0, wallet.assets["USDT"].Lock)
		require.Equal(t, 0.0, wallet.assets["BTC"].Free)
		require.Equal(t, 1.0, wallet.assets["BTC"].Lock)

		// a new candle should execute order and unlock values
		wallet.OnCandle(model.Candle{Pair: "BTCUSDT", Close: 200, High: 200})
		require.Equal(t, model.OrderStatusTypeFilled, wallet.orders[1].Status)
		require.Equal(t, 200.0, wallet.assets["USDT"].Free)
		require.Equal(t, 0.0, wallet.assets["USDT"].Lock)
		require.Equal(t, 0.0, wallet.assets["BTC"].Free)
		require.Equal(t, 0.0, wallet.assets["BTC"].Lock)
	})

	t.Run("multiple pending orders", func(t *testing.T) {
		wallet := NewPaperWallet(context.Background(), "USDT", WithPaperAsset("USDT", 100))
		wallet.lastCandle["BTCUSDT"] = model.Candle{Close: 10}

		order, err := wallet.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 1, 10)
		require.NoError(t, err)
		require.NotEmpty(t, order)

		require.Equal(t, 90.0, wallet.assets["USDT"].Free)
		require.Equal(t, 10.0, wallet.assets["USDT"].Lock)

		order, err = wallet.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 1, 20)
		require.NoError(t, err)
		require.NotEmpty(t, order)

		require.Equal(t, 70.0, wallet.assets["USDT"].Free)
		require.Equal(t, 30.0, wallet.assets["USDT"].Lock)

		order, err = wallet.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 1, 50)
		require.NoError(t, err)
		require.NotEmpty(t, order)

		require.Equal(t, 20.0, wallet.assets["USDT"].Free)
		require.Equal(t, 80.0, wallet.assets["USDT"].Lock)

		// should execute two orders and keep one pending
		wallet.OnCandle(model.Candle{Pair: "BTCUSDT", Close: 15})
		require.Equal(t, 20.0, wallet.assets["USDT"].Free)
		require.Equal(t, 10.0, wallet.assets["USDT"].Lock)
		require.Equal(t, 0.0, wallet.assets["BTC"].Lock)
		require.Equal(t, 2.0, wallet.assets["BTC"].Free)
		require.Equal(t, 35.0, wallet.avgPrice["BTCUSDT"])
		require.Equal(t, model.OrderStatusTypeNew, wallet.orders[0].Status)
		require.Equal(t, model.OrderStatusTypeFilled, wallet.orders[1].Status)
		require.Equal(t, model.OrderStatusTypeFilled, wallet.orders[2].Status)

		// sell all bitcoin position
		order, err = wallet.CreateOrderLimit(model.SideTypeSell, "BTCUSDT", 2, 40)
		require.NoError(t, err)
		require.NotEmpty(t, order)

		require.Equal(t, 20.0, wallet.assets["USDT"].Free)
		require.Equal(t, 10.0, wallet.assets["USDT"].Lock)
		require.Equal(t, 0.0, wallet.assets["BTC"].Free)
		require.Equal(t, 2.0, wallet.assets["BTC"].Lock)

		wallet.OnCandle(model.Candle{Pair: "BTCUSDT", Close: 50, High: 50})
		require.Equal(t, 0.0, wallet.assets["BTC"].Free)
		require.Equal(t, 0.0, wallet.assets["BTC"].Lock)
		require.Equal(t, 100.0, wallet.assets["USDT"].Free)
		require.Equal(t, 10.0, wallet.assets["USDT"].Lock)

		// execute old buy position
		wallet.OnCandle(model.Candle{Pair: "BTCUSDT", Close: 9, High: 9})
		require.Equal(t, 1.0, wallet.assets["BTC"].Free)
		require.Equal(t, 0.0, wallet.assets["BTC"].Lock)
		require.Equal(t, 100.0, wallet.assets["USDT"].Free)
		require.Equal(t, 0.0, wallet.assets["USDT"].Lock)
		require.Equal(t, 10.0, wallet.avgPrice["BTCUSDT"])
	})
}

func TestPaperWallet_OrderMarket(t *testing.T) {
	wallet := NewPaperWallet(context.Background(), "USDT", WithPaperAsset("USDT", 100))
	wallet.OnCandle(model.Candle{Pair: "BTCUSDT", Close: 50})
	order, err := wallet.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 1)
	require.NoError(t, err)

	// create buy order
	require.Len(t, wallet.orders, 1)
	assert.Equal(t, model.OrderStatusTypeFilled, order.Status)
	assert.Equal(t, 1.0, order.Quantity)
	assert.Equal(t, 50.0, order.Price)
	assert.Equal(t, 50.0, wallet.assets["USDT"].Free)
	assert.Equal(t, 0.0, wallet.assets["USDT"].Lock)
	assert.Equal(t, 1.0, wallet.assets["BTC"].Free)
	assert.Equal(t, 0.0, wallet.assets["BTC"].Lock)
	assert.Equal(t, 50.0, wallet.avgPrice["BTCUSDT"])

	// insufficient funds
	order, err = wallet.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 100)
	require.Equal(t, &OrderError{
		Err:      ErrInsufficientFunds,
		Pair:     "BTCUSDT",
		Quantity: 100}, err)
	require.Empty(t, order)

	// sell
	wallet.OnCandle(model.Candle{Pair: "BTCUSDT", Close: 100})
	order, err = wallet.CreateOrderMarket(model.SideTypeSell, "BTCUSDT", 1)
	require.NoError(t, err)
	assert.Equal(t, 1.0, order.Quantity)
	assert.Equal(t, 100.0, order.Price)
	assert.Equal(t, 150.0, wallet.assets["USDT"].Free)
	assert.Equal(t, 0.0, wallet.assets["USDT"].Lock)
	assert.Equal(t, 0.0, wallet.assets["BTC"].Free)
	assert.Equal(t, 0.0, wallet.assets["BTC"].Lock)
	assert.Equal(t, 50.0, wallet.avgPrice["BTCUSDT"])
}

func TestPaperWallet_OrderOCO(t *testing.T) {
	wallet := NewPaperWallet(context.Background(), "USDT", WithPaperAsset("USDT", 50))
	wallet.OnCandle(model.Candle{Pair: "BTCUSDT", Close: 50})
	_, err := wallet.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 1)
	require.NoError(t, err)

	orders, err := wallet.CreateOrderOCO(model.SideTypeSell, "BTCUSDT", 1, 100, 40, 39)
	require.NoError(t, err)

	// create buy order
	require.Len(t, wallet.orders, 3)
	assert.Equal(t, model.OrderStatusTypeNew, orders[0].Status)
	assert.Equal(t, model.OrderStatusTypeNew, orders[1].Status)
	assert.Equal(t, 1.0, orders[0].Quantity)
	assert.Equal(t, 1.0, orders[1].Quantity)

	assert.Equal(t, 0.0, wallet.assets["USDT"].Free)
	assert.Equal(t, 0.0, wallet.assets["USDT"].Lock)
	assert.Equal(t, 0.0, wallet.assets["BTC"].Free)
	assert.Equal(t, 1.0, wallet.assets["BTC"].Lock)

	// insufficient funds
	orders, err = wallet.CreateOrderOCO(model.SideTypeSell, "BTCUSDT", 1, 100, 40, 39)
	require.Equal(t, &OrderError{
		Err:      ErrInsufficientFunds,
		Pair:     "BTC",
		Quantity: 1}, err)
	require.Nil(t, orders)

	// execute stop and cancel target
	wallet.OnCandle(model.Candle{Pair: "BTCUSDT", Close: 30})
	assert.Equal(t, 40.0, wallet.assets["USDT"].Free)
	assert.Equal(t, 0.0, wallet.assets["USDT"].Lock)
	assert.Equal(t, 0.0, wallet.assets["BTC"].Free)
	assert.Equal(t, 0.0, wallet.assets["BTC"].Lock)
	assert.Equal(t, wallet.orders[1].Status, model.OrderStatusTypeCanceled)
	assert.Equal(t, wallet.orders[2].Status, model.OrderStatusTypeFilled)
}

func TestPaperWallet_Order(t *testing.T) {
	wallet := NewPaperWallet(context.Background(), "USDT", WithPaperAsset("USDT", 100))
	expectOrder, err := wallet.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 1)
	require.NoError(t, err)
	require.Equal(t, int64(1), expectOrder.ExchangeID)

	order, err := wallet.Order("BTCUSDT", expectOrder.ExchangeID)
	require.NoError(t, err)
	require.Equal(t, expectOrder, order)
}

func TestPaperWallet_MaxDrawndown(t *testing.T) {
	tt := []struct {
		name   string
		values []AssetValue
		result float64
		start  time.Time
		end    time.Time
	}{
		{
			name: "down only",
			values: []AssetValue{
				{Time: time.Date(2019, time.January, 1, 0, 0, 0, 0, time.UTC), Value: 10},
				{Time: time.Date(2019, time.January, 2, 0, 0, 0, 0, time.UTC), Value: 5},
			},
			result: -0.5,
			start:  time.Date(2019, time.January, 1, 0, 0, 0, 0, time.UTC),
			end:    time.Date(2019, time.January, 2, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "up and down",
			values: []AssetValue{
				{Time: time.Date(2019, time.January, 1, 0, 0, 0, 0, time.UTC), Value: 1},
				{Time: time.Date(2019, time.January, 2, 0, 0, 0, 0, time.UTC), Value: 10},
				{Time: time.Date(2019, time.January, 3, 0, 0, 0, 0, time.UTC), Value: 5},
			},
			result: -0.5,
			start:  time.Date(2019, time.January, 2, 0, 0, 0, 0, time.UTC),
			end:    time.Date(2019, time.January, 3, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "down and up",
			values: []AssetValue{
				{Time: time.Date(2019, time.January, 1, 0, 0, 0, 0, time.UTC), Value: 4},
				{Time: time.Date(2019, time.January, 2, 0, 0, 0, 0, time.UTC), Value: 5},
				{Time: time.Date(2019, time.January, 3, 0, 0, 0, 0, time.UTC), Value: 4},
				{Time: time.Date(2019, time.January, 4, 0, 0, 0, 0, time.UTC), Value: 3},
				{Time: time.Date(2019, time.January, 5, 0, 0, 0, 0, time.UTC), Value: 4},
				{Time: time.Date(2019, time.January, 6, 0, 0, 0, 0, time.UTC), Value: 5},
				{Time: time.Date(2019, time.January, 7, 0, 0, 0, 0, time.UTC), Value: 6},
				{Time: time.Date(2019, time.January, 8, 0, 0, 0, 0, time.UTC), Value: 7},
				{Time: time.Date(2019, time.January, 9, 0, 0, 0, 0, time.UTC), Value: 6},
			},
			result: -0.4,
			start:  time.Date(2019, time.January, 2, 0, 0, 0, 0, time.UTC),
			end:    time.Date(2019, time.January, 4, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "two drawn downs",
			values: []AssetValue{
				{Time: time.Date(2019, time.January, 1, 0, 0, 0, 0, time.UTC), Value: 1},
				{Time: time.Date(2019, time.January, 2, 0, 0, 0, 0, time.UTC), Value: 5},
				{Time: time.Date(2019, time.January, 3, 0, 0, 0, 0, time.UTC), Value: 4},
				{Time: time.Date(2019, time.January, 4, 0, 0, 0, 0, time.UTC), Value: 7},
				{Time: time.Date(2019, time.January, 5, 0, 0, 0, 0, time.UTC), Value: 8},
				{Time: time.Date(2019, time.January, 6, 0, 0, 0, 0, time.UTC), Value: 4},
				{Time: time.Date(2019, time.January, 7, 0, 0, 0, 0, time.UTC), Value: 5},
				{Time: time.Date(2019, time.January, 8, 0, 0, 0, 0, time.UTC), Value: 2},
				{Time: time.Date(2019, time.January, 9, 0, 0, 0, 0, time.UTC), Value: 3},
			},
			result: -0.75,
			start:  time.Date(2019, time.January, 5, 0, 0, 0, 0, time.UTC),
			end:    time.Date(2019, time.January, 8, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			wallet := PaperWallet{
				equityValues: tc.values,
			}

			max, start, end := wallet.MaxDrawdown()
			assert.Equal(t, tc.result, max)
			assert.Equal(t, tc.start, start)
			assert.Equal(t, tc.end, end)
		})
	}
}

func TestPaperWallet_AssetsInfo(t *testing.T) {
	wallet := PaperWallet{}
	info := wallet.AssetsInfo("BTCUSDT")
	require.Equal(t, info.QuotePrecision, 8)
	require.Equal(t, info.BaseAsset, "BTC")
	require.Equal(t, info.QuoteAsset, "USDT")
}

func TestPaperWallet_CreateOrderStop(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		wallet := NewPaperWallet(context.Background(), "USDT", WithPaperAsset("USDT", 100))
		wallet.OnCandle(model.Candle{Pair: "BTCUSDT", Close: 100})
		_, err := wallet.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 1)
		require.NoError(t, err)

		order, err := wallet.CreateOrderStop("BTCUSDT", 1, 50)
		require.NoError(t, err)

		// create order and lock values
		require.Len(t, wallet.orders, 2)
		require.Equal(t, 1.0, order.Quantity)
		require.Equal(t, 50.0, order.Price)
		require.Equal(t, 50.0, *order.Stop)
		require.Equal(t, 0.0, wallet.assets["BTC"].Free)
		require.Equal(t, 1.0, wallet.assets["BTC"].Lock)

		// a new candle should execute order and unlock values
		wallet.OnCandle(model.Candle{Pair: "BTCUSDT", Close: 40})
		require.Equal(t, model.OrderStatusTypeFilled, wallet.orders[1].Status)
		require.Equal(t, 50.0, wallet.assets["USDT"].Free)
		require.Equal(t, 0.0, wallet.assets["USDT"].Lock)
		require.Equal(t, 0.0, wallet.assets["BTC"].Free)
		require.Equal(t, 0.0, wallet.assets["BTC"].Lock)
		require.Equal(t, 100.0, wallet.avgPrice["BTCUSDT"])
	})
}
