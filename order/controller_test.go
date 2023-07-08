package order

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/storage"
)

func TestController_updatePosition(t *testing.T) {
	t.Run("market orders", func(t *testing.T) {
		storage, err := storage.FromMemory()
		require.NoError(t, err)
		ctx := context.Background()
		wallet := exchange.NewPaperWallet(ctx, "USDT", exchange.WithPaperAsset("USDT", 3000))
		controller := NewController(ctx, wallet, storage, NewOrderFeed())

		wallet.OnCandle(model.Candle{Pair: "BTCUSDT", Close: 1000})
		_, err = controller.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 1)
		require.NoError(t, err)

		require.Equal(t, 1000.0, controller.position["BTCUSDT"].AvgPrice)
		require.Equal(t, 1.0, controller.position["BTCUSDT"].Quantity)
		assert.Equal(t, model.SideTypeBuy, controller.position["BTCUSDT"].Side)

		wallet.OnCandle(model.Candle{Pair: "BTCUSDT", Close: 2000})
		_, err = controller.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 1)
		require.NoError(t, err)

		require.Equal(t, 1500.0, controller.position["BTCUSDT"].AvgPrice)
		require.Equal(t, 2.0, controller.position["BTCUSDT"].Quantity)

		// close half position 1BTC with 100% of profit
		wallet.OnCandle(model.Candle{Pair: "BTCUSDT", Close: 3000})
		order, err := controller.CreateOrderMarket(model.SideTypeSell, "BTCUSDT", 1)
		require.NoError(t, err)

		assert.Equal(t, 1500.0, controller.position["BTCUSDT"].AvgPrice)
		assert.Equal(t, 1.0, controller.position["BTCUSDT"].Quantity)

		assert.Equal(t, 1500.0, order.ProfitValue)
		assert.Equal(t, 1.0, order.Profit)

		// sell remaining BTC, 50% of loss
		wallet.OnCandle(model.Candle{Pair: "BTCUSDT", Close: 750})
		order, err = controller.CreateOrderMarket(model.SideTypeSell, "BTCUSDT", 1)
		require.NoError(t, err)

		assert.Nil(t, controller.position["BTCUSDT"]) // close position
		assert.Equal(t, -750.0, order.ProfitValue)
		assert.Equal(t, -0.5, order.Profit)
	})

	t.Run("limit order", func(t *testing.T) {
		storage, err := storage.FromMemory()
		require.NoError(t, err)
		ctx := context.Background()
		wallet := exchange.NewPaperWallet(ctx, "USDT", exchange.WithPaperAsset("USDT", 3000))
		controller := NewController(ctx, wallet, storage, NewOrderFeed())
		wallet.OnCandle(model.Candle{Time: time.Now(), Pair: "BTCUSDT", High: 1500, Close: 1500})

		_, err = controller.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 1, 1000)
		require.NoError(t, err)

		// should execute previous order
		wallet.OnCandle(model.Candle{Time: time.Now(), Pair: "BTCUSDT", High: 1000, Close: 1000})
		controller.updateOrders()

		require.Equal(t, 1000.0, controller.position["BTCUSDT"].AvgPrice)
		require.Equal(t, 1.0, controller.position["BTCUSDT"].Quantity)

		_, err = controller.CreateOrderLimit(model.SideTypeSell, "BTCUSDT", 1, 2000)
		require.NoError(t, err)

		// should execute previous order
		wallet.OnCandle(model.Candle{Time: time.Now(), Pair: "BTCUSDT", High: 2000, Close: 2000})
		controller.updateOrders()

		require.Nil(t, controller.position["BTCUSDT"])
		require.Len(t, controller.Results["BTCUSDT"].WinLong, 1)
		require.Equal(t, 1000.0, controller.Results["BTCUSDT"].WinLong[0])
		require.Len(t, controller.Results["BTCUSDT"].WinLongPercent, 1)
		require.Equal(t, 1.0, controller.Results["BTCUSDT"].WinLongPercent[0])
	})

	t.Run("oco order limit maker", func(t *testing.T) {
		storage, err := storage.FromMemory()
		require.NoError(t, err)
		ctx := context.Background()
		wallet := exchange.NewPaperWallet(ctx, "USDT", exchange.WithPaperAsset("USDT", 3000))
		controller := NewController(ctx, wallet, storage, NewOrderFeed())
		wallet.OnCandle(model.Candle{Time: time.Now(), Pair: "BTCUSDT", High: 1500, Close: 1500})

		_, err = controller.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 1, 1000)
		require.NoError(t, err)

		// should execute previous order
		wallet.OnCandle(model.Candle{Time: time.Now(), Pair: "BTCUSDT", High: 1000, Close: 1000})
		controller.updateOrders()

		_, err = controller.CreateOrderOCO(model.SideTypeSell, "BTCUSDT", 1, 2000, 500, 500)
		require.NoError(t, err)

		// should execute previous order
		wallet.OnCandle(model.Candle{Time: time.Now(), Pair: "BTCUSDT", High: 2000, Close: 2000})
		controller.updateOrders()

		require.Nil(t, controller.position["BTCUSDT"])
		require.Len(t, controller.Results["BTCUSDT"].WinLong, 1)
		require.Equal(t, 1000.0, controller.Results["BTCUSDT"].WinLong[0])
		require.Len(t, controller.Results["BTCUSDT"].WinLongPercent, 1)
		require.Equal(t, 1.0, controller.Results["BTCUSDT"].WinLongPercent[0])
	})

	t.Run("oco stop sell", func(t *testing.T) {
		storage, err := storage.FromMemory()
		require.NoError(t, err)
		ctx := context.Background()
		wallet := exchange.NewPaperWallet(ctx, "USDT", exchange.WithPaperAsset("USDT", 3000))
		controller := NewController(ctx, wallet, storage, NewOrderFeed())
		wallet.OnCandle(model.Candle{Time: time.Now(), Pair: "BTCUSDT", Close: 1500, Low: 1500})

		_, err = controller.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 0.5, 1000)
		require.NoError(t, err)

		_, err = controller.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 1.5, 1000)
		require.NoError(t, err)

		// should execute previous order
		wallet.OnCandle(model.Candle{Time: time.Now(), Pair: "BTCUSDT", Close: 1000, Low: 1000})
		controller.updateOrders()

		assert.Equal(t, 1000.0, controller.position["BTCUSDT"].AvgPrice)
		assert.Equal(t, 2.0, controller.position["BTCUSDT"].Quantity)

		_, err = controller.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 1.0)
		require.NoError(t, err)

		assert.Equal(t, 1000.0, controller.position["BTCUSDT"].AvgPrice)
		assert.Equal(t, 3.0, controller.position["BTCUSDT"].Quantity)

		_, err = controller.CreateOrderOCO(model.SideTypeSell, "BTCUSDT", 1, 2000, 500, 500)
		require.NoError(t, err)

		// should execute previous order
		wallet.OnCandle(model.Candle{Time: time.Now(), Pair: "BTCUSDT", Close: 400, Low: 400})
		controller.updateOrders()

		assert.Equal(t, 1000.0, controller.position["BTCUSDT"].AvgPrice)
		assert.Equal(t, 2.0, controller.position["BTCUSDT"].Quantity)

		require.Len(t, controller.Results["BTCUSDT"].LoseLong, 1)
		require.Equal(t, -500.0, controller.Results["BTCUSDT"].LoseLong[0])
		require.Len(t, controller.Results["BTCUSDT"].LoseLongPercent, 1)
		require.Equal(t, -0.5, controller.Results["BTCUSDT"].LoseLongPercent[0])
	})

	t.Run("short market", func(t *testing.T) {
		storage, err := storage.FromMemory()
		require.NoError(t, err)
		ctx := context.Background()

		wallet := exchange.NewPaperWallet(ctx, "USDT", exchange.WithPaperAsset("USDT", 0),
			exchange.WithPaperAsset("BTC", 2))
		controller := NewController(ctx, wallet, storage, NewOrderFeed())
		wallet.OnCandle(model.Candle{Time: time.Now(), Pair: "BTCUSDT", Close: 1500, Low: 1500})

		_, err = controller.CreateOrderMarket(model.SideTypeSell, "BTCUSDT", 1)
		require.NoError(t, err)

		assert.Equal(t, model.SideTypeSell, controller.position["BTCUSDT"].Side)
		assert.Equal(t, 1500.0, controller.position["BTCUSDT"].AvgPrice)
		assert.Equal(t, 1.0, controller.position["BTCUSDT"].Quantity)
	})
}

func TestController_PositionValue(t *testing.T) {
	storage, err := storage.FromMemory()
	require.NoError(t, err)
	ctx := context.Background()
	wallet := exchange.NewPaperWallet(ctx, "USDT", exchange.WithPaperAsset("USDT", 3000))
	controller := NewController(ctx, wallet, storage, NewOrderFeed())

	lastCandle := model.Candle{Time: time.Now(), Pair: "BTCUSDT", Close: 1500, Low: 1500}

	// update wallet and controller
	wallet.OnCandle(lastCandle)
	controller.OnCandle(lastCandle)

	_, err = controller.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 1.0)
	require.NoError(t, err)

	value, err := controller.PositionValue("BTCUSDT")
	require.NoError(t, err)
	assert.Equal(t, 1500.0, value)
}

func TestController_Position(t *testing.T) {
	storage, err := storage.FromMemory()
	require.NoError(t, err)
	ctx := context.Background()
	wallet := exchange.NewPaperWallet(ctx, "USDT", exchange.WithPaperAsset("USDT", 3000))
	controller := NewController(ctx, wallet, storage, NewOrderFeed())

	lastCandle := model.Candle{Time: time.Now(), Pair: "BTCUSDT", Close: 1500, Low: 1500}

	// update wallet and controller
	wallet.OnCandle(lastCandle)
	controller.OnCandle(lastCandle)

	_, err = controller.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 1.0)
	require.NoError(t, err)

	asset, quote, err := controller.Position("BTCUSDT")
	require.NoError(t, err)
	assert.Equal(t, 1.0, asset)
	assert.Equal(t, 1500.0, quote)
}
