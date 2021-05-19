package order

import (
	"context"
	"testing"

	"github.com/rodrigo-brito/ninjabot/pkg/exchange"
	"github.com/rodrigo-brito/ninjabot/pkg/model"
	"github.com/rodrigo-brito/ninjabot/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestController_calculateProfit(t *testing.T) {
	storage, err := storage.FromMemory()
	require.NoError(t, err)
	ctx := context.Background()
	wallet := exchange.NewPaperWallet(ctx, "USDT", exchange.WithPaperAsset("USDT", 3000))
	controller := NewController(ctx, wallet, storage, NewOrderFeed(), nil)

	wallet.OnCandle(model.Candle{Symbol: "BTCUSDT", Close: 1000})
	_, err = controller.OrderMarket(model.SideTypeBuy, "BTCUSDT", 1)
	require.NoError(t, err)

	wallet.OnCandle(model.Candle{Symbol: "BTCUSDT", Close: 2000})
	_, err = controller.OrderMarket(model.SideTypeBuy, "BTCUSDT", 1)
	require.NoError(t, err)

	// close half position 1BTC with 100% of profit
	wallet.OnCandle(model.Candle{Symbol: "BTCUSDT", Close: 3000})
	sellOrder, err := controller.OrderMarket(model.SideTypeSell, "BTCUSDT", 1)
	require.NoError(t, err)

	value, profit, err := controller.calculateProfit(&sellOrder)
	require.NoError(t, err)
	assert.Equal(t, 1500.0, value)
	assert.Equal(t, 1.0, profit)

	// sell remaining BTC, 50% of loss
	wallet.OnCandle(model.Candle{Symbol: "BTCUSDT", Close: 750})
	sellOrder, err = controller.OrderMarket(model.SideTypeSell, "BTCUSDT", 1)
	require.NoError(t, err)
	value, profit, err = controller.calculateProfit(&sellOrder)
	require.NoError(t, err)
	assert.Equal(t, -750.0, value)
	assert.Equal(t, -0.5, profit)
}
