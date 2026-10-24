package order

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/storage"
)

func TestScoped_NettingAndLedgers(t *testing.T) {
	ctx := context.Background()
	wallet := exchange.NewPaperWallet(ctx, "USDT", exchange.WithPaperAsset("USDT", 1000))
	controller := NewController(ctx, wallet, storage.NewMemory(), NewSyncFeed())
	controller.SetBudget("a", 600)
	controller.SetBudget("b", 400)
	a, b := controller.Scoped("a"), controller.Scoped("b")

	wallet.OnCandle(model.Candle{Pair: "BTCUSDT", Close: 100, Complete: true})
	controller.OnCandle(model.Candle{Pair: "BTCUSDT", Close: 100, Complete: true})
	_, err := a.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 2)
	require.NoError(t, err)
	order, err := b.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 1)
	require.NoError(t, err)
	require.Equal(t, "b", order.Tag)

	asset, quote, err := a.Position("BTCUSDT")
	require.NoError(t, err)
	require.Equal(t, 2.0, asset)
	require.Equal(t, 400.0, quote, "the budget of a, minus its buy")
	asset, quote, err = b.Position("BTCUSDT")
	require.NoError(t, err)
	require.Equal(t, 1.0, asset)
	require.Equal(t, 300.0, quote)

	// a closes its position at 110; the account keeps the 1 BTC of b
	wallet.OnCandle(model.Candle{Pair: "BTCUSDT", Close: 110, Complete: true})
	controller.OnCandle(model.Candle{Pair: "BTCUSDT", Close: 110, Complete: true})
	_, err = a.CreateOrderMarket(model.SideTypeSell, "BTCUSDT", 2)
	require.NoError(t, err)

	walletAsset, _, err := wallet.Position("BTCUSDT")
	require.NoError(t, err)
	require.Equal(t, 1.0, walletAsset)
	asset, _, _ = a.Position("BTCUSDT")
	require.Zero(t, asset)

	trades := controller.Trades()
	require.Len(t, trades, 1)
	require.Equal(t, "a", trades[0].Tag)
	require.InDelta(t, 20.0, trades[0].ProfitValue, 1e-9)
	require.Contains(t, controller.Results, "a|BTCUSDT")

	require.InDelta(t, 620.0, controller.TagEquity("a"), 1e-9)
	require.InDelta(t, 410.0, controller.TagEquity("b"), 1e-9)
	require.InDelta(t, wallet.Equity(), controller.TagEquity("a")+controller.TagEquity("b"), 1e-9)
	require.Zero(t, controller.TagEquity("unknown"))
	require.Zero(t, controller.TagFees("unknown"))
}

func TestScoped_WithoutBudget(t *testing.T) {
	ctx := context.Background()
	wallet := exchange.NewPaperWallet(ctx, "USDT", exchange.WithPaperAsset("USDT", 1000))
	controller := NewController(ctx, wallet, storage.NewMemory(), NewSyncFeed())
	scoped := controller.Scoped("x")

	wallet.OnCandle(model.Candle{Pair: "BTCUSDT", Close: 100, High: 100, Low: 100, Complete: true})
	_, err := scoped.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 3)
	require.NoError(t, err)
	_, err = controller.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 1)
	require.NoError(t, err)

	asset, quote, err := scoped.Position("BTCUSDT")
	require.NoError(t, err)
	require.Equal(t, 3.0, asset, "only the holdings of the tag")
	require.Equal(t, 600.0, quote, "the shared quote balance")

	// tags survive the sync of asynchronous fills
	orders, err := scoped.CreateOrderOCO(model.SideTypeSell, "BTCUSDT", 3, 120, 90, 90)
	require.NoError(t, err)
	require.Equal(t, "x", orders[0].Tag)
	wallet.OnCandle(model.Candle{Pair: "BTCUSDT", Close: 121, High: 121, Low: 119, Complete: true})
	controller.Sync()
	trades := controller.Trades()
	require.Len(t, trades, 1)
	require.Equal(t, "x", trades[0].Tag)

	limit, err := scoped.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 1, 50)
	require.NoError(t, err)
	require.Equal(t, "x", limit.Tag)
	quoteOrder, err := scoped.CreateOrderMarketQuote(model.SideTypeBuy, "BTCUSDT", 100)
	require.NoError(t, err)
	require.Equal(t, "x", quoteOrder.Tag)
	stop, err := scoped.CreateOrderStop("BTCUSDT", 0.5, 100)
	require.NoError(t, err)
	require.Equal(t, "x", stop.Tag)
}
