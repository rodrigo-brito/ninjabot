package exchange

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/model"
)

const futuresExchangeInfoResponse = `{
	"timezone": "UTC",
	"serverTime": 1600000000000,
	"symbols": [{
		"symbol": "BTCUSDT",
		"status": "TRADING",
		"baseAsset": "BTC",
		"baseAssetPrecision": 8,
		"quoteAsset": "USDT",
		"quotePrecision": 8,
		"quotePrecisionInt": 8,
		"filters": [
			{"filterType": "LOT_SIZE", "minQty": "0.00100000", "maxQty": "100.00000000", "stepSize": "0.00100000"},
			{"filterType": "PRICE_FILTER", "minPrice": "0.01000000", "maxPrice": "1000000.00000000", "tickSize": "0.01000000"}
		]
	}]
}`

const futuresOrderResponse = `{
	"symbol": "BTCUSDT",
	"orderId": 50,
	"clientOrderId": "abc",
	"price": "9000.00",
	"origQty": "1.000",
	"executedQty": "0.000",
	"cumQuote": "0.00",
	"status": "NEW",
	"timeInForce": "GTC",
	"type": "LIMIT",
	"side": "BUY",
	"updateTime": 1600000000000,
	"time": 1600000000000
}`

// newFuturesServer starts a fake Binance Futures REST API, always answering
// the ping and exchangeInfo calls made on startup.
func newFuturesServer(t *testing.T, routes map[route]binanceHandler) *httptest.Server {
	t.Helper()

	handlers := map[route]binanceHandler{
		"GET /fapi/v1/ping":         ok(`{}`),
		"GET /fapi/v1/exchangeInfo": ok(futuresExchangeInfoResponse),
	}
	for key, handler := range routes {
		handlers[key] = handler
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler, found := handlers[route(r.Method+" "+r.URL.Path)]
		if !found {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":-1121,"msg":"unexpected route"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(handler.status)
		_, _ = w.Write([]byte(handler.body))
	}))
	t.Cleanup(server.Close)

	return server
}

// useFuturesServer points the futures client at url until the test ends.
func useFuturesServer(t *testing.T, url string) {
	t.Helper()

	previous := futures.BaseApiMainUrl
	futures.BaseApiMainUrl = url
	t.Cleanup(func() { futures.BaseApiMainUrl = previous })
}

func newTestBinanceFuture(t *testing.T, routes map[route]binanceHandler,
	options ...BinanceFutureOption) *BinanceFuture {
	t.Helper()

	server := newFuturesServer(t, routes)
	useFuturesServer(t, server.URL)

	options = append([]BinanceFutureOption{WithBinanceFutureCredentials("key", "secret")}, options...)
	exchange, err := NewBinanceFuture(context.Background(), options...)
	require.NoError(t, err)

	return exchange
}

func TestNewBinanceFuture(t *testing.T) {
	t.Run("loads the asset limits from the exchange info", func(t *testing.T) {
		exchange := newTestBinanceFuture(t, nil)

		info := exchange.AssetsInfo("BTCUSDT")
		require.Equal(t, "BTC", info.BaseAsset)
		require.Equal(t, "USDT", info.QuoteAsset)
		require.Equal(t, 0.001, info.MinQuantity)
		require.Equal(t, 100.0, info.MaxQuantity)
		require.Equal(t, 0.01, info.TickSize)
	})

	t.Run("fails when the exchange is unreachable", func(t *testing.T) {
		useFuturesServer(t, "http://127.0.0.1:1")

		_, err := NewBinanceFuture(context.Background())

		require.ErrorContains(t, err, "binance ping fail")
	})

	t.Run("fails when the exchange info request fails", func(t *testing.T) {
		server := newFuturesServer(t, map[route]binanceHandler{
			"GET /fapi/v1/exchangeInfo": failure("service unavailable"),
		})
		useFuturesServer(t, server.URL)

		_, err := NewBinanceFuture(context.Background())

		require.ErrorContains(t, err, "service unavailable")
	})

	t.Run("applies the leverage and margin type of each pair", func(t *testing.T) {
		exchange := newTestBinanceFuture(t, map[route]binanceHandler{
			"POST /fapi/v1/leverage":   ok(`{"leverage": 10, "maxNotionalValue": "1000", "symbol": "BTCUSDT"}`),
			"POST /fapi/v1/marginType": ok(`{"code": 200, "msg": "success"}`),
		}, WithBinanceFutureLeverage("btcusdt", 10, MarginTypeIsolated))

		require.Len(t, exchange.PairOptions, 1)
		require.Equal(t, "BTCUSDT", exchange.PairOptions[0].Pair, "the pair is normalized to upper case")
		require.Equal(t, 10, exchange.PairOptions[0].Leverage)
		require.Equal(t, MarginTypeIsolated, exchange.PairOptions[0].MarginType)
	})

	t.Run("tolerates a margin type that is already set", func(t *testing.T) {
		server := newFuturesServer(t, map[route]binanceHandler{
			"POST /fapi/v1/leverage": ok(`{"leverage": 10, "symbol": "BTCUSDT"}`),
			"POST /fapi/v1/marginType": {
				status: http.StatusBadRequest,
				body:   `{"code":-4046,"msg":"No need to change margin type."}`,
			},
		})
		useFuturesServer(t, server.URL)

		_, err := NewBinanceFuture(context.Background(),
			WithBinanceFutureLeverage("BTCUSDT", 10, MarginTypeCrossed))

		require.NoError(t, err)
	})

	t.Run("fails when the leverage cannot be set", func(t *testing.T) {
		server := newFuturesServer(t, map[route]binanceHandler{
			"POST /fapi/v1/leverage": failure("invalid leverage"),
		})
		useFuturesServer(t, server.URL)

		_, err := NewBinanceFuture(context.Background(),
			WithBinanceFutureLeverage("BTCUSDT", 200, MarginTypeIsolated))

		require.ErrorContains(t, err, "invalid leverage")
	})

	t.Run("fails on an unexpected margin type error", func(t *testing.T) {
		server := newFuturesServer(t, map[route]binanceHandler{
			"POST /fapi/v1/leverage":   ok(`{"leverage": 10, "symbol": "BTCUSDT"}`),
			"POST /fapi/v1/marginType": failure("invalid margin type"),
		})
		useFuturesServer(t, server.URL)

		_, err := NewBinanceFuture(context.Background(),
			WithBinanceFutureLeverage("BTCUSDT", 10, MarginTypeIsolated))

		require.ErrorContains(t, err, "invalid margin type")
	})

	t.Run("enables Heikin Ashi candles", func(t *testing.T) {
		exchange := newTestBinanceFuture(t, nil, WithBinanceFuturesHeikinAshiCandle())

		require.True(t, exchange.HeikinAshi)
	})
}

func TestBinanceFutureValidate(t *testing.T) {
	exchange := newTestBinanceFuture(t, nil)

	require.ErrorIs(t, exchange.validate("UNKNOWN", 1), ErrInvalidAsset)
	require.ErrorIs(t, exchange.validate("BTCUSDT", 0.0001), ErrInvalidQuantity)
	require.ErrorIs(t, exchange.validate("BTCUSDT", 1000), ErrInvalidQuantity)
	require.NoError(t, exchange.validate("BTCUSDT", 1))
}

func TestBinanceFutureFormat(t *testing.T) {
	exchange := newTestBinanceFuture(t, nil)

	t.Run("rounds to the pair precision", func(t *testing.T) {
		require.Equal(t, "1.111", exchange.formatQuantity("BTCUSDT", 1.1111111))
		require.Equal(t, "9000.12", exchange.formatPrice("BTCUSDT", 9000.123456))
	})

	t.Run("falls back to the raw value for unknown pairs", func(t *testing.T) {
		require.Equal(t, "1.1111111", exchange.formatQuantity("UNKNOWN", 1.1111111))
		require.Equal(t, "1.1111111", exchange.formatPrice("UNKNOWN", 1.1111111))
	})
}

func TestBinanceFutureCandles(t *testing.T) {
	routes := map[route]binanceHandler{"GET /fapi/v1/klines": ok(klinesResponse)}

	t.Run("CandlesByLimit discards the incomplete last candle", func(t *testing.T) {
		exchange := newTestBinanceFuture(t, routes)

		candles, err := exchange.CandlesByLimit(context.Background(), "BTCUSDT", "1h", 1)

		require.NoError(t, err)
		require.Len(t, candles, 1)
		require.Equal(t, 105.0, candles[0].Close)
	})

	t.Run("CandlesByPeriod keeps every candle", func(t *testing.T) {
		exchange := newTestBinanceFuture(t, routes)

		candles, err := exchange.CandlesByPeriod(context.Background(), "BTCUSDT", "1h",
			time.Unix(1600000000, 0), time.Unix(1600007200, 0))

		require.NoError(t, err)
		require.Len(t, candles, 2)
	})

	t.Run("converts to Heikin Ashi when enabled", func(t *testing.T) {
		exchange := newTestBinanceFuture(t, routes, WithBinanceFuturesHeikinAshiCandle())

		byLimit, err := exchange.CandlesByLimit(context.Background(), "BTCUSDT", "1h", 1)
		require.NoError(t, err)
		require.NotEqual(t, 105.0, byLimit[0].Close)

		byPeriod, err := exchange.CandlesByPeriod(context.Background(), "BTCUSDT", "1h",
			time.Unix(1600000000, 0), time.Unix(1600007200, 0))
		require.NoError(t, err)
		require.NotEqual(t, 105.0, byPeriod[0].Close)
	})

	t.Run("LastQuote returns the last close", func(t *testing.T) {
		exchange := newTestBinanceFuture(t, routes)

		quote, err := exchange.LastQuote(context.Background(), "BTCUSDT")

		require.NoError(t, err)
		require.Equal(t, 105.0, quote)
	})

	t.Run("propagates request errors", func(t *testing.T) {
		exchange := newTestBinanceFuture(t, map[route]binanceHandler{
			"GET /fapi/v1/klines": failure("invalid symbol"),
		})

		_, err := exchange.CandlesByLimit(context.Background(), "BTCUSDT", "1h", 1)
		require.ErrorContains(t, err, "invalid symbol")

		_, err = exchange.CandlesByPeriod(context.Background(), "BTCUSDT", "1h", time.Now(), time.Now())
		require.ErrorContains(t, err, "invalid symbol")

		quote, err := exchange.LastQuote(context.Background(), "BTCUSDT")
		require.Error(t, err)
		require.Zero(t, quote)
	})
}

func TestBinanceFutureCreateOrder(t *testing.T) {
	t.Run("limit order keeps the requested price", func(t *testing.T) {
		exchange := newTestBinanceFuture(t, map[route]binanceHandler{
			"POST /fapi/v1/order": ok(futuresOrderResponse),
		})

		order, err := exchange.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 1, 9000)

		require.NoError(t, err)
		require.Equal(t, int64(50), order.ExchangeID)
		require.Equal(t, 9000.0, order.Price)
		require.Equal(t, 1.0, order.Quantity)
		require.Equal(t, model.OrderStatusTypeNew, order.Status)
	})

	t.Run("market order uses the average fill price", func(t *testing.T) {
		exchange := newTestBinanceFuture(t, map[route]binanceHandler{
			"POST /fapi/v1/order": ok(`{
				"symbol": "BTCUSDT", "orderId": 51, "price": "0.00", "origQty": "2.000",
				"executedQty": "2.000", "cumQuote": "20000.00", "status": "FILLED",
				"timeInForce": "GTC", "type": "MARKET", "side": "BUY", "updateTime": 1600000000000
			}`),
		})

		order, err := exchange.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 2)

		require.NoError(t, err)
		require.Equal(t, 10000.0, order.Price)
		require.Equal(t, 2.0, order.Quantity)
	})

	t.Run("stop order", func(t *testing.T) {
		exchange := newTestBinanceFuture(t, map[route]binanceHandler{
			"POST /fapi/v1/order": ok(`{
				"symbol": "BTCUSDT", "orderId": 52, "price": "8000.00", "origQty": "1.000",
				"executedQty": "0.000", "cumQuote": "0.00", "status": "NEW",
				"timeInForce": "GTC", "type": "STOP_MARKET", "side": "SELL", "updateTime": 1600000000000
			}`),
		})

		order, err := exchange.CreateOrderStop("BTCUSDT", 1, 8000)

		require.NoError(t, err)
		require.Equal(t, 8000.0, order.Price)
		require.Equal(t, model.SideTypeSell, order.Side)
	})

	t.Run("rejects invalid quantities before hitting the exchange", func(t *testing.T) {
		exchange := newTestBinanceFuture(t, nil)

		_, err := exchange.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 0.00001, 100)
		require.ErrorIs(t, err, ErrInvalidQuantity)

		_, err = exchange.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 0.00001)
		require.ErrorIs(t, err, ErrInvalidQuantity)

		_, err = exchange.CreateOrderStop("BTCUSDT", 0.00001, 100)
		require.ErrorIs(t, err, ErrInvalidQuantity)
	})

	t.Run("propagates exchange errors", func(t *testing.T) {
		exchange := newTestBinanceFuture(t, map[route]binanceHandler{
			"POST /fapi/v1/order": failure("insufficient margin"),
		})

		_, err := exchange.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 1, 100)
		require.ErrorContains(t, err, "insufficient margin")

		_, err = exchange.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 1)
		require.ErrorContains(t, err, "insufficient margin")

		_, err = exchange.CreateOrderStop("BTCUSDT", 1, 100)
		require.ErrorContains(t, err, "insufficient margin")
	})

	t.Run("OCO and market quote orders are not supported", func(t *testing.T) {
		exchange := newTestBinanceFuture(t, nil)

		require.Panics(t, func() {
			_, _ = exchange.CreateOrderOCO(model.SideTypeSell, "BTCUSDT", 1, 1, 1, 1)
		})
		require.Panics(t, func() {
			_, _ = exchange.CreateOrderMarketQuote(model.SideTypeBuy, "BTCUSDT", 1)
		})
	})
}

func TestBinanceFutureOrders(t *testing.T) {
	t.Run("Order fetches a single order", func(t *testing.T) {
		exchange := newTestBinanceFuture(t, map[route]binanceHandler{
			"GET /fapi/v1/order": ok(futuresOrderResponse),
		})

		order, err := exchange.Order("BTCUSDT", 50)

		require.NoError(t, err)
		require.Equal(t, int64(50), order.ExchangeID)
		require.Equal(t, 9000.0, order.Price)
	})

	t.Run("Orders lists the recent orders", func(t *testing.T) {
		exchange := newTestBinanceFuture(t, map[route]binanceHandler{
			"GET /fapi/v1/allOrders": ok("[" + futuresOrderResponse + "]"),
		})

		orders, err := exchange.Orders("BTCUSDT", 10)

		require.NoError(t, err)
		require.Len(t, orders, 1)
	})

	t.Run("Cancel removes an open order", func(t *testing.T) {
		exchange := newTestBinanceFuture(t, map[route]binanceHandler{
			"DELETE /fapi/v1/order": ok(futuresOrderResponse),
		})

		require.NoError(t, exchange.Cancel(model.Order{Pair: "BTCUSDT", ExchangeID: 50}))
	})

	t.Run("propagates exchange errors", func(t *testing.T) {
		exchange := newTestBinanceFuture(t, map[route]binanceHandler{
			"GET /fapi/v1/order":     failure("order not found"),
			"GET /fapi/v1/allOrders": failure("order not found"),
			"DELETE /fapi/v1/order":  failure("order not found"),
		})

		_, err := exchange.Order("BTCUSDT", 50)
		require.ErrorContains(t, err, "order not found")

		_, err = exchange.Orders("BTCUSDT", 10)
		require.ErrorContains(t, err, "order not found")

		require.ErrorContains(t, exchange.Cancel(model.Order{Pair: "BTCUSDT"}), "order not found")
	})
}

func TestNewFutureOrder(t *testing.T) {
	t.Run("filled order reports the average fill price", func(t *testing.T) {
		order := newFutureOrder(&futures.Order{
			OrderID:          60,
			Symbol:           "BTCUSDT",
			Side:             futures.SideTypeSell,
			Type:             futures.OrderTypeMarket,
			Status:           futures.OrderStatusTypeFilled,
			Price:            "0",
			OrigQuantity:     "2",
			ExecutedQuantity: "2",
			CumQuote:         "19000",
			Time:             1600000000000,
			UpdateTime:       1600000001000,
		})

		require.Equal(t, 9500.0, order.Price)
		require.Equal(t, 2.0, order.Quantity)
		require.Equal(t, model.SideTypeSell, order.Side)
	})

	t.Run("open order keeps the requested price", func(t *testing.T) {
		order := newFutureOrder(&futures.Order{
			OrderID:          61,
			Symbol:           "BTCUSDT",
			Side:             futures.SideTypeBuy,
			Type:             futures.OrderTypeLimit,
			Status:           futures.OrderStatusTypeNew,
			Price:            "9000",
			OrigQuantity:     "2",
			ExecutedQuantity: "0",
			CumQuote:         "0",
		})

		require.Equal(t, 9000.0, order.Price)
		require.Equal(t, 2.0, order.Quantity)
	})
}

func TestBinanceFutureAccount(t *testing.T) {
	const accountResponse = `{
		"assets": [
			{"asset": "USDT", "walletBalance": "1000.00000000"},
			{"asset": "BUSD", "walletBalance": "0.00000000"}
		],
		"positions": [
			{"symbol": "BTCUSDT", "positionAmt": "1.50000000", "leverage": "10", "positionSide": "LONG"},
			{"symbol": "ETHUSDT", "positionAmt": "2.00000000", "leverage": "5", "positionSide": "SHORT"},
			{"symbol": "BNBUSDT", "positionAmt": "0.00000000", "leverage": "5", "positionSide": "LONG"}
		]
	}`

	t.Run("maps open positions and funded assets", func(t *testing.T) {
		exchange := newTestBinanceFuture(t, map[route]binanceHandler{
			"GET /fapi/v2/account": ok(accountResponse),
		})

		account, err := exchange.Account()

		require.NoError(t, err)
		require.Len(t, account.Balances, 3, "empty positions and assets are skipped")

		btc, _ := account.Balance("BTC", "USDT")
		require.Equal(t, 1.5, btc.Free)
		require.Equal(t, 10.0, btc.Leverage)

		eth, _ := account.Balance("ETH", "USDT")
		require.Equal(t, -2.0, eth.Free, "short positions are reported as a negative balance")
	})

	t.Run("Position sums the asset and quote balances", func(t *testing.T) {
		exchange := newTestBinanceFuture(t, map[route]binanceHandler{
			"GET /fapi/v2/account": ok(accountResponse),
		})

		asset, quote, err := exchange.Position("BTCUSDT")

		require.NoError(t, err)
		require.Equal(t, 1.5, asset)
		require.Equal(t, 1000.0, quote)
	})

	t.Run("fails on malformed numbers", func(t *testing.T) {
		malformed := map[string]string{
			"position amount":   `{"positions": [{"symbol": "BTCUSDT", "positionAmt": "abc", "leverage": "10"}], "assets": []}`,
			"position leverage": `{"positions": [{"symbol": "BTCUSDT", "positionAmt": "1", "leverage": "abc"}], "assets": []}`,
			"wallet balance":    `{"positions": [], "assets": [{"asset": "USDT", "walletBalance": "abc"}]}`,
		}

		for name, body := range malformed {
			t.Run(name, func(t *testing.T) {
				exchange := newTestBinanceFuture(t, map[route]binanceHandler{
					"GET /fapi/v2/account": ok(body),
				})

				_, err := exchange.Account()
				require.Error(t, err)
			})
		}
	})

	t.Run("propagates exchange errors", func(t *testing.T) {
		exchange := newTestBinanceFuture(t, map[route]binanceHandler{
			"GET /fapi/v2/account": failure("invalid signature"),
		})

		_, err := exchange.Account()
		require.ErrorContains(t, err, "invalid signature")

		_, _, err = exchange.Position("BTCUSDT")
		require.ErrorContains(t, err, "invalid signature")
	})
}

func TestFutureCandleFromKline(t *testing.T) {
	candle := FutureCandleFromKline("BTCUSDT", futures.Kline{
		OpenTime: 1600000000000,
		Open:     "100.5",
		Close:    "105.5",
		High:     "110.5",
		Low:      "95.5",
		Volume:   "12.5",
	})

	require.Equal(t, "BTCUSDT", candle.Pair)
	require.Equal(t, 105.5, candle.Close)
	require.True(t, candle.Complete)
	require.NotNil(t, candle.Metadata)
}

func TestFutureCandleFromWsKline(t *testing.T) {
	tests := []struct {
		name         string
		isFinal      bool
		wantComplete bool
	}{
		{"partial kline", false, false},
		{"closed kline", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candle := FutureCandleFromWsKline("BTCUSDT", futures.WsKline{
				StartTime: 1600000000000,
				Open:      "100.5",
				Close:     "105.5",
				High:      "110.5",
				Low:       "95.5",
				Volume:    "12.5",
				IsFinal:   tt.isFinal,
			})

			require.Equal(t, tt.wantComplete, candle.Complete)
			require.Equal(t, 105.5, candle.Close)
		})
	}
}
