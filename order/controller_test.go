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

func TestController_StopGatesCreateOrder(t *testing.T) {
	storage, err := storage.FromMemory()
	require.NoError(t, err)
	ctx := context.Background()
	wallet := exchange.NewPaperWallet(ctx, "USDT",
		exchange.WithPaperAsset("USDT", 3000),
		exchange.WithPaperAsset("BTC", 1),
	)
	controller := NewController(ctx, wallet, storage, NewOrderFeed())
	defer controller.Stop()

	wallet.OnCandle(model.Candle{Pair: "BTCUSDT", Close: 1000})

	// Zero-value status (never Start()'d) must still accept orders — public API.
	_, err = controller.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 0.1)
	require.NoError(t, err)

	controller.Start()
	pending, err := controller.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 0.1, 900)
	require.NoError(t, err)

	controller.Stop()

	_, err = controller.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 0.1)
	require.ErrorIs(t, err, ErrBotStopped)

	_, err = controller.CreateOrderMarketQuote(model.SideTypeBuy, "BTCUSDT", 100)
	require.ErrorIs(t, err, ErrBotStopped)

	_, err = controller.CreateOrderLimit(model.SideTypeSell, "BTCUSDT", 0.1, 1100)
	require.ErrorIs(t, err, ErrBotStopped)

	_, err = controller.CreateOrderStop("BTCUSDT", 0.1, 800)
	require.ErrorIs(t, err, ErrBotStopped)

	_, err = controller.CreateOrderOCO(model.SideTypeSell, "BTCUSDT", 0.1, 1100, 900, 890)
	require.ErrorIs(t, err, ErrBotStopped)

	// Cancel stays available while stopped so open orders can be cleaned up.
	require.NoError(t, controller.Cancel(pending))

	canceled, err := controller.Order("BTCUSDT", pending.ExchangeID)
	require.NoError(t, err)
	assert.Equal(t, model.OrderStatusTypeCanceled, canceled.Status)

	controller.Start()
	_, err = controller.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 0.1)
	require.NoError(t, err)
}

func TestPosition_Update(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	newOrder := func(side model.SideType, quantity, price float64) *model.Order {
		return &model.Order{
			Pair:      "BTCUSDT",
			Side:      side,
			Type:      model.OrderTypeMarket,
			Quantity:  quantity,
			Price:     price,
			CreatedAt: base.Add(time.Hour),
		}
	}

	t.Run("short closed in profit", func(t *testing.T) {
		position := &Position{Side: model.SideTypeSell, AvgPrice: 1000, Quantity: 2, CreatedAt: base}
		order := newOrder(model.SideTypeBuy, 2, 800)

		result, finished := position.Update(order)

		require.NotNil(t, result)
		assert.True(t, finished)
		assert.Equal(t, model.SideTypeSell, result.Side)
		assert.InDelta(t, 0.2, order.Profit, 1e-9)
		assert.InDelta(t, 400.0, order.ProfitValue, 1e-9)
	})

	t.Run("short closed in loss", func(t *testing.T) {
		position := &Position{Side: model.SideTypeSell, AvgPrice: 1000, Quantity: 1, CreatedAt: base}
		order := newOrder(model.SideTypeBuy, 1, 1200)

		result, finished := position.Update(order)

		require.NotNil(t, result)
		assert.True(t, finished)
		assert.Equal(t, model.SideTypeSell, result.Side)
		assert.InDelta(t, -0.2, order.Profit, 1e-9)
		assert.InDelta(t, -200.0, order.ProfitValue, 1e-9)
	})

	t.Run("short partially closed", func(t *testing.T) {
		position := &Position{Side: model.SideTypeSell, AvgPrice: 1000, Quantity: 2, CreatedAt: base}
		order := newOrder(model.SideTypeBuy, 1, 900)

		result, finished := position.Update(order)

		require.NotNil(t, result)
		assert.False(t, finished)
		assert.InDelta(t, 0.1, order.Profit, 1e-9)
		assert.InDelta(t, 100.0, order.ProfitValue, 1e-9)
		assert.Equal(t, model.SideTypeSell, position.Side)
		assert.Equal(t, 1.0, position.Quantity)
		assert.Equal(t, 1000.0, position.AvgPrice)
	})

	t.Run("long flipped to short", func(t *testing.T) {
		position := &Position{Side: model.SideTypeBuy, AvgPrice: 1000, Quantity: 1, CreatedAt: base}
		order := newOrder(model.SideTypeSell, 3, 1100)

		result, finished := position.Update(order)

		require.NotNil(t, result)
		assert.False(t, finished)

		// the closed leg is the old long: 1 unit bought at 1000, sold at 1100
		assert.Equal(t, model.SideTypeBuy, result.Side)
		assert.InDelta(t, 0.1, order.Profit, 1e-9)
		assert.InDelta(t, 100.0, order.ProfitValue, 1e-9)

		// the remainder opens a short at the order price
		assert.Equal(t, model.SideTypeSell, position.Side)
		assert.Equal(t, 2.0, position.Quantity)
		assert.Equal(t, 1100.0, position.AvgPrice)
		assert.Equal(t, order.CreatedAt, position.CreatedAt)
	})

	t.Run("short flipped to long", func(t *testing.T) {
		position := &Position{Side: model.SideTypeSell, AvgPrice: 1000, Quantity: 1, CreatedAt: base}
		order := newOrder(model.SideTypeBuy, 2, 900)

		result, finished := position.Update(order)

		require.NotNil(t, result)
		assert.False(t, finished)

		// the closed leg is the old short: sold at 1000, bought back at 900
		assert.Equal(t, model.SideTypeSell, result.Side)
		assert.InDelta(t, 0.1, order.Profit, 1e-9)
		assert.InDelta(t, 100.0, order.ProfitValue, 1e-9)

		// the remainder opens a long at the order price
		assert.Equal(t, model.SideTypeBuy, position.Side)
		assert.Equal(t, 1.0, position.Quantity)
		assert.Equal(t, 900.0, position.AvgPrice)
	})

	t.Run("fees are deducted from the profit", func(t *testing.T) {
		position := &Position{Side: model.SideTypeBuy, AvgPrice: 1000, Quantity: 1, Fee: 1, CreatedAt: base}
		order := newOrder(model.SideTypeSell, 1, 1100)
		order.Fee = 1.1

		result, finished := position.Update(order)

		require.NotNil(t, result)
		assert.True(t, finished)
		assert.InDelta(t, 97.9, order.ProfitValue, 1e-9) // 100 - 1 - 1.1
		assert.InDelta(t, 0.0979, order.Profit, 1e-9)
		assert.InDelta(t, order.ProfitValue, result.ProfitValue, 1e-9)
	})

	t.Run("only the closed share of the fees is settled", func(t *testing.T) {
		position := &Position{Side: model.SideTypeBuy, AvgPrice: 1000, Quantity: 2, Fee: 2, CreatedAt: base}
		order := newOrder(model.SideTypeSell, 1, 1100)
		order.Fee = 1.1

		result, finished := position.Update(order)

		require.NotNil(t, result)
		assert.False(t, finished)
		assert.InDelta(t, 97.9, order.ProfitValue, 1e-9) // 100 - 1 (half the entry) - 1.1
		assert.Equal(t, 1.0, position.Quantity)
		assert.InDelta(t, 1.0, position.Fee, 1e-9) // the fee of the open half
	})

	t.Run("a flip carries the unused fee to the new position", func(t *testing.T) {
		position := &Position{Side: model.SideTypeBuy, AvgPrice: 1000, Quantity: 1, Fee: 1, CreatedAt: base}
		order := newOrder(model.SideTypeSell, 3, 1100)
		order.Fee = 3.3

		result, finished := position.Update(order)

		require.NotNil(t, result)
		assert.False(t, finished)
		assert.InDelta(t, 97.9, order.ProfitValue, 1e-9) // 100 - 1 - 1.1 (a third of the exit)
		assert.Equal(t, 2.0, position.Quantity)
		assert.InDelta(t, 2.2, position.Fee, 1e-9) // the remaining two thirds
	})

	t.Run("adding to a position accumulates the fees", func(t *testing.T) {
		position := &Position{Side: model.SideTypeBuy, AvgPrice: 1000, Quantity: 1, Fee: 1, CreatedAt: base}
		order := newOrder(model.SideTypeBuy, 1, 1200)
		order.Fee = 1.2

		result, finished := position.Update(order)

		require.Nil(t, result)
		assert.False(t, finished)
		assert.InDelta(t, 2.2, position.Fee, 1e-9)
	})

	t.Run("stop loss settles at the stop price", func(t *testing.T) {
		stop := 950.0
		position := &Position{Side: model.SideTypeSell, AvgPrice: 1000, Quantity: 1, CreatedAt: base}
		order := newOrder(model.SideTypeBuy, 1, 990)
		order.Type = model.OrderTypeStopLoss
		order.Stop = &stop

		result, finished := position.Update(order)

		require.NotNil(t, result)
		assert.True(t, finished)
		assert.InDelta(t, 0.05, order.Profit, 1e-9)
		assert.InDelta(t, 50.0, order.ProfitValue, 1e-9)
	})
}
