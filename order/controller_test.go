package order

import (
	"context"
	"testing"
	"time"

	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestController_calculateProfit(t *testing.T) {
	t.Run("market orders", func(t *testing.T) {
		storage, err := storage.FromMemory()
		require.NoError(t, err)
		ctx := context.Background()
		wallet := exchange.NewPaperWallet(ctx, "USDT", exchange.WithPaperAsset("USDT", 3000))
		controller := NewController(ctx, wallet, storage, NewOrderFeed(), nil)

		wallet.OnCandle(model.Candle{Symbol: "BTCUSDT", Close: 1000})
		_, err = controller.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 1)
		require.NoError(t, err)

		wallet.OnCandle(model.Candle{Symbol: "BTCUSDT", Close: 2000})
		_, err = controller.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 1)
		require.NoError(t, err)

		// close half position 1BTC with 100% of profit
		wallet.OnCandle(model.Candle{Symbol: "BTCUSDT", Close: 3000})
		sellOrder, err := controller.CreateOrderMarket(model.SideTypeSell, "BTCUSDT", 1)
		require.NoError(t, err)

		value, profit, volume, err := controller.calculateProfit(&sellOrder)
		require.NoError(t, err)
		assert.Equal(t, 1500.0, value)
		assert.Equal(t, 1.0, profit)
		assert.Equal(t, 3000.0, volume)

		// sell remaining BTC, 50% of loss
		wallet.OnCandle(model.Candle{Symbol: "BTCUSDT", Close: 750})
		sellOrder, err = controller.CreateOrderMarket(model.SideTypeSell, "BTCUSDT", 1)
		require.NoError(t, err)
		value, profit, volume, err = controller.calculateProfit(&sellOrder)
		require.NoError(t, err)
		assert.Equal(t, -750.0, value)
		assert.Equal(t, -0.5, profit)
		assert.Equal(t, 6000.0, volume)
	})

	t.Run("limit order", func(t *testing.T) {
		storage, err := storage.FromMemory()
		require.NoError(t, err)
		ctx := context.Background()
		wallet := exchange.NewPaperWallet(ctx, "USDT", exchange.WithPaperAsset("USDT", 3000))
		controller := NewController(ctx, wallet, storage, NewOrderFeed(), nil)
		wallet.OnCandle(model.Candle{Time: time.Now(), Symbol: "BTCUSDT", High: 1500, Close: 1500})

		_, err = controller.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 1, 1000)
		require.NoError(t, err)

		// should execute previous order
		wallet.OnCandle(model.Candle{Time: time.Now(), Symbol: "BTCUSDT", High: 1000, Close: 1000})

		sellOrder, err := controller.CreateOrderLimit(model.SideTypeSell, "BTCUSDT", 1, 2000)
		require.NoError(t, err)

		// should execute previous order
		wallet.OnCandle(model.Candle{Time: time.Now(), Symbol: "BTCUSDT", High: 2000, Close: 2000})
		controller.updateOrders()

		value, profit, volume, err := controller.calculateProfit(&sellOrder)
		require.NoError(t, err)
		assert.Equal(t, 1000.0, value)
		assert.Equal(t, 1.0, profit)
		assert.Equal(t, 7750.0, volume)
	})

	t.Run("oco order limit maker", func(t *testing.T) {
		storage, err := storage.FromMemory()
		require.NoError(t, err)
		ctx := context.Background()
		wallet := exchange.NewPaperWallet(ctx, "USDT", exchange.WithPaperAsset("USDT", 3000))
		controller := NewController(ctx, wallet, storage, NewOrderFeed(), nil)
		wallet.OnCandle(model.Candle{Time: time.Now(), Symbol: "BTCUSDT", High: 1500, Close: 1500})

		_, err = controller.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 1, 1000)
		require.NoError(t, err)

		// should execute previous order
		wallet.OnCandle(model.Candle{Time: time.Now(), Symbol: "BTCUSDT", High: 1000, Close: 1000})

		sellOrder, err := controller.CreateOrderOCO(model.SideTypeSell, "BTCUSDT", 1, 2000, 500, 500)
		require.NoError(t, err)

		// should execute previous order
		wallet.OnCandle(model.Candle{Time: time.Now(), Symbol: "BTCUSDT", High: 2000, Close: 2000})
		controller.updateOrders()

		value, profit, volume, err := controller.calculateProfit(&sellOrder[0])
		require.NoError(t, err)
		assert.Equal(t, 1000.0, value)
		assert.Equal(t, 1.0, profit)
		assert.Equal(t, 10750.0, volume)
	})

	t.Run("oco stop sell", func(t *testing.T) {
		storage, err := storage.FromMemory()
		require.NoError(t, err)
		ctx := context.Background()
		wallet := exchange.NewPaperWallet(ctx, "USDT", exchange.WithPaperAsset("USDT", 3000))
		controller := NewController(ctx, wallet, storage, NewOrderFeed(), nil)
		wallet.OnCandle(model.Candle{Time: time.Now(), Symbol: "BTCUSDT", Close: 1500, Low: 1500})

		_, err = controller.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 0.5, 1000)
		require.NoError(t, err)

		_, err = controller.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 1.5, 1000)
		require.NoError(t, err)

		// should execute previous order
		wallet.OnCandle(model.Candle{Time: time.Now(), Symbol: "BTCUSDT", Close: 1000, Low: 1000})

		_, err = controller.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 1.0)
		require.NoError(t, err)

		sellOrder, err := controller.CreateOrderOCO(model.SideTypeSell, "BTCUSDT", 1, 2000, 500, 500)
		require.NoError(t, err)

		// should execute previous order
		wallet.OnCandle(model.Candle{Time: time.Now(), Symbol: "BTCUSDT", Close: 400, Low: 400})
		controller.updateOrders()

		value, profit, volume, err := controller.calculateProfit(&sellOrder[1])
		require.NoError(t, err)
		assert.Equal(t, -500.0, value)
		assert.Equal(t, -0.5, profit)
		assert.Equal(t, 15750.0, volume)
	})
}
