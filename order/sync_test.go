package order

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/storage"
)

func TestController_Sync_InterleavedFills(t *testing.T) {
	ctx := context.Background()
	store, err := storage.FromMemory()
	require.NoError(t, err)

	wallet := exchange.NewPaperWallet(ctx, "USDT", exchange.WithPaperAsset("USDT", 1000))
	feed := NewSyncFeed()
	var published []model.Order
	feed.Subscribe("BTCUSDT", func(o model.Order) { published = append(published, o) }, false)
	controller := NewController(ctx, wallet, store, feed)

	start := time.Date(2021, time.January, 1, 0, 0, 0, 0, time.UTC)
	candle := func(i int, closePrice, high, low float64) model.Candle {
		return model.Candle{Pair: "BTCUSDT", Time: start.Add(time.Duration(i) * time.Hour),
			Close: closePrice, High: high, Low: low, Complete: true}
	}

	wallet.OnCandle(candle(0, 100, 100, 100))
	_, err = controller.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 1)
	require.NoError(t, err)
	limit, err := controller.CreateOrderLimit(model.SideTypeSell, "BTCUSDT", 1, 110)
	require.NoError(t, err)

	// the limit sell fills on this candle; the engine syncs right away
	wallet.OnCandle(candle(1, 108, 111, 104))
	require.Len(t, wallet.TakeFills(), 1)
	controller.Sync()

	trades := controller.Trades()
	require.Len(t, trades, 1)
	require.InDelta(t, 10.0, trades[0].ProfitValue, 1e-9)
	require.InDelta(t, 0.1, trades[0].ProfitPercent, 1e-9)
	require.Equal(t, start.Add(time.Hour), trades[0].ClosedAt)
	require.Equal(t, limit.ID, trades[0].OrderID)

	// a new market buy opens a fresh position instead of averaging the old one
	wallet.OnCandle(candle(2, 105, 105, 105))
	_, err = controller.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 1)
	require.NoError(t, err)
	_, err = controller.CreateOrderMarket(model.SideTypeSell, "BTCUSDT", 1)
	require.NoError(t, err)

	trades = controller.Trades()
	require.Len(t, trades, 2)
	require.InDelta(t, 0.0, trades[1].ProfitValue, 1e-9)

	// synchronous feed: market, limit, limit fill, market, market — in order
	require.Len(t, published, 5)
	require.Equal(t, model.OrderStatusTypeFilled, published[2].Status)
	require.Equal(t, model.OrderTypeLimit, published[2].Type)
	require.Equal(t, model.OrderTypeMarket, published[3].Type)
}

func TestFeed_Synchronous(t *testing.T) {
	feed := NewSyncFeed()
	require.True(t, feed.Synchronous())
	require.False(t, NewOrderFeed().Synchronous())

	var got []int64
	feed.Subscribe("BTCUSDT", func(o model.Order) { got = append(got, o.ID) }, false)
	feed.Start() // no-op
	feed.Publish(model.Order{ID: 1, Pair: "BTCUSDT"}, false)
	feed.Publish(model.Order{ID: 2, Pair: "ETHUSDT"}, false) // no subscriber
	feed.Publish(model.Order{ID: 3, Pair: "BTCUSDT"}, false)
	require.Equal(t, []int64{1, 3}, got)
}
