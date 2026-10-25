package exchange

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/model"
)

// route identifies a fake Binance endpoint, e.g. "POST /api/v3/order".
type route string

// binanceHandler is the reply of the fake exchange for one route. Returning a
// non-200 status makes go-binance surface an APIError.
type binanceHandler struct {
	status int
	body   string
}

func ok(body string) binanceHandler { return binanceHandler{status: http.StatusOK, body: body} }

func failure(message string) binanceHandler {
	return binanceHandler{status: http.StatusBadRequest, body: fmt.Sprintf(`{"code":-1013,"msg":%q}`, message)}
}

const exchangeInfoResponse = `{
	"timezone": "UTC",
	"serverTime": 1600000000000,
	"symbols": [{
		"symbol": "BTCUSDT",
		"status": "TRADING",
		"baseAsset": "BTC",
		"baseAssetPrecision": 8,
		"quoteAsset": "USDT",
		"quotePrecision": 8,
		"quoteAssetPrecision": 8,
		"orderTypes": ["LIMIT", "MARKET"],
		"filters": [
			{"filterType": "LOT_SIZE", "minQty": "0.00100000", "maxQty": "100.00000000", "stepSize": "0.00100000"},
			{"filterType": "PRICE_FILTER", "minPrice": "0.01000000", "maxPrice": "1000000.00000000", "tickSize": "0.01000000"},
			{"filterType": "MIN_NOTIONAL", "minNotional": "10.00000000"}
		]
	}]
}`

const klinesResponse = `[
	[1600000000000, "100.0", "110.0", "90.0", "105.0", "10.0", 1600003599999, "1000.0", 10, "5.0", "500.0", "0"],
	[1600003600000, "105.0", "115.0", "95.0", "110.0", "12.0", 1600007199999, "1200.0", 12, "6.0", "600.0", "0"]
]`

const marketOrderResponse = `{
	"symbol": "BTCUSDT",
	"orderId": 28,
	"clientOrderId": "abc",
	"transactTime": 1600000000000,
	"price": "0.00000000",
	"origQty": "1.00000000",
	"executedQty": "1.00000000",
	"cummulativeQuoteQty": "10000.00000000",
	"status": "FILLED",
	"timeInForce": "GTC",
	"type": "MARKET",
	"side": "BUY"
}`

// newBinanceServer starts a fake Binance REST API. The ping and exchangeInfo
// routes are always available, since NewBinance calls them on startup.
func newBinanceServer(t *testing.T, routes map[route]binanceHandler) *httptest.Server {
	t.Helper()

	handlers := map[route]binanceHandler{
		"GET /api/v3/ping":         ok(`{}`),
		"GET /api/v3/exchangeInfo": ok(exchangeInfoResponse),
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

// newTestBinance points the Binance client at a fake REST API and restores the
// package level endpoints afterwards.
func newTestBinance(t *testing.T, routes map[route]binanceHandler, options ...BinanceOption) *Binance {
	t.Helper()

	server := newBinanceServer(t, routes)
	restoreBinanceEndpoints(t)

	options = append([]BinanceOption{
		WithCustomMainAPIEndpoint(server.URL, "ws://localhost", "ws://localhost"),
		WithBinanceCredentials("key", "secret"),
	}, options...)

	exchange, err := NewBinance(context.Background(), options...)
	require.NoError(t, err)

	return exchange
}

func restoreBinanceEndpoints(t *testing.T) {
	t.Helper()

	api, ws, combined := binance.BaseAPIMainURL, binance.BaseWsMainURL, binance.BaseCombinedMainURL
	testAPI, testWs, testCombined := binance.BaseAPITestnetURL, binance.BaseWsTestnetURL, binance.BaseCombinedTestnetURL
	testnet := binance.UseTestnet

	t.Cleanup(func() {
		binance.BaseAPIMainURL, binance.BaseWsMainURL, binance.BaseCombinedMainURL = api, ws, combined
		binance.BaseAPITestnetURL, binance.BaseWsTestnetURL, binance.BaseCombinedTestnetURL = testAPI, testWs, testCombined
		binance.UseTestnet = testnet
	})
}

func TestNewBinance(t *testing.T) {
	t.Run("loads the asset limits from the exchange info", func(t *testing.T) {
		exchange := newTestBinance(t, nil)

		info := exchange.AssetsInfo("BTCUSDT")
		require.Equal(t, "BTC", info.BaseAsset)
		require.Equal(t, "USDT", info.QuoteAsset)
		require.Equal(t, 0.001, info.MinQuantity)
		require.Equal(t, 100.0, info.MaxQuantity)
		require.Equal(t, 0.001, info.StepSize)
		require.Equal(t, 0.01, info.MinPrice)
		require.Equal(t, 0.01, info.TickSize)
		require.Equal(t, 8, info.BaseAssetPrecision)
	})

	t.Run("fails when the exchange is unreachable", func(t *testing.T) {
		restoreBinanceEndpoints(t)
		binance.BaseAPIMainURL = "http://127.0.0.1:1"

		_, err := NewBinance(context.Background())

		require.ErrorContains(t, err, "binance ping fail")
	})

	t.Run("fails when the exchange info request fails", func(t *testing.T) {
		server := newBinanceServer(t, map[route]binanceHandler{
			"GET /api/v3/exchangeInfo": failure("service unavailable"),
		})
		restoreBinanceEndpoints(t)

		_, err := NewBinance(context.Background(),
			WithCustomMainAPIEndpoint(server.URL, "ws://localhost", "ws://localhost"))

		require.ErrorContains(t, err, "service unavailable")
	})

	t.Run("applies the optional settings", func(t *testing.T) {
		var fetched bool
		exchange := newTestBinance(t, nil,
			WithBinanceHeikinAshiCandle(),
			WithMetadataFetcher(func(string, time.Time) (string, float64) {
				fetched = true
				return "funding", 0.01
			}),
		)

		require.True(t, exchange.HeikinAshi)
		require.Len(t, exchange.MetadataFetchers, 1)

		key, value := exchange.MetadataFetchers[0]("BTCUSDT", time.Now())
		require.True(t, fetched)
		require.Equal(t, "funding", key)
		require.Equal(t, 0.01, value)
	})

	t.Run("WithTestNet switches the client to the testnet", func(t *testing.T) {
		restoreBinanceEndpoints(t)
		binance.UseTestnet = false

		WithTestNet()(&Binance{})

		require.True(t, binance.UseTestnet)
	})

	t.Run("WithCustomTestnetAPIEndpoint overrides the testnet endpoints", func(t *testing.T) {
		restoreBinanceEndpoints(t)

		WithCustomTestnetAPIEndpoint("http://api", "ws://ws", "ws://combined")(&Binance{})

		require.Equal(t, "http://api", binance.BaseAPITestnetURL)
		require.Equal(t, "ws://ws", binance.BaseWsTestnetURL)
		require.Equal(t, "ws://combined", binance.BaseCombinedTestnetURL)
	})
}

func TestBinanceValidate(t *testing.T) {
	exchange := newTestBinance(t, nil)

	t.Run("rejects unknown pairs", func(t *testing.T) {
		err := exchange.validate("UNKNOWN", 1)
		require.ErrorIs(t, err, ErrInvalidAsset)
	})

	t.Run("rejects quantities outside the lot size", func(t *testing.T) {
		require.ErrorIs(t, exchange.validate("BTCUSDT", 0.0001), ErrInvalidQuantity)
		require.ErrorIs(t, exchange.validate("BTCUSDT", 1000), ErrInvalidQuantity)
	})

	t.Run("accepts a valid quantity", func(t *testing.T) {
		require.NoError(t, exchange.validate("BTCUSDT", 1))
	})
}

func TestBinanceCandles(t *testing.T) {
	routes := map[route]binanceHandler{"GET /api/v3/klines": ok(klinesResponse)}

	t.Run("CandlesByLimit discards the incomplete last candle", func(t *testing.T) {
		exchange := newTestBinance(t, routes)

		candles, err := exchange.CandlesByLimit(context.Background(), "BTCUSDT", "1h", 1)

		require.NoError(t, err)
		require.Len(t, candles, 1)
		require.Equal(t, 105.0, candles[0].Close)
		require.True(t, candles[0].Complete)
	})

	t.Run("CandlesByPeriod keeps every candle", func(t *testing.T) {
		exchange := newTestBinance(t, routes)

		candles, err := exchange.CandlesByPeriod(context.Background(), "BTCUSDT", "1h",
			time.Unix(1600000000, 0), time.Unix(1600007200, 0))

		require.NoError(t, err)
		require.Len(t, candles, 2)
		require.Equal(t, 110.0, candles[1].Close)
	})

	t.Run("converts to Heikin Ashi when enabled", func(t *testing.T) {
		exchange := newTestBinance(t, routes, WithBinanceHeikinAshiCandle())

		candles, err := exchange.CandlesByPeriod(context.Background(), "BTCUSDT", "1h",
			time.Unix(1600000000, 0), time.Unix(1600007200, 0))

		require.NoError(t, err)
		require.NotEqual(t, 105.0, candles[0].Close, "the Heikin Ashi close averages the OHLC values")
	})

	t.Run("LastQuote returns the last close", func(t *testing.T) {
		exchange := newTestBinance(t, routes)

		quote, err := exchange.LastQuote(context.Background(), "BTCUSDT")

		require.NoError(t, err)
		require.Equal(t, 105.0, quote)
	})

	t.Run("propagates request errors", func(t *testing.T) {
		exchange := newTestBinance(t, map[route]binanceHandler{
			"GET /api/v3/klines": failure("invalid symbol"),
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

func TestBinanceCreateOrder(t *testing.T) {
	t.Run("market order uses the average fill price", func(t *testing.T) {
		exchange := newTestBinance(t, map[route]binanceHandler{
			"POST /api/v3/order": ok(marketOrderResponse),
		})

		order, err := exchange.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 1)

		require.NoError(t, err)
		require.Equal(t, int64(28), order.ExchangeID)
		require.Equal(t, "BTCUSDT", order.Pair)
		require.Equal(t, model.SideTypeBuy, order.Side)
		require.Equal(t, model.OrderTypeMarket, order.Type)
		require.Equal(t, model.OrderStatusTypeFilled, order.Status)
		require.Equal(t, 10000.0, order.Price)
		require.Equal(t, 1.0, order.Quantity)
	})

	t.Run("market quote order uses the average fill price", func(t *testing.T) {
		exchange := newTestBinance(t, map[route]binanceHandler{
			"POST /api/v3/order": ok(marketOrderResponse),
		})

		order, err := exchange.CreateOrderMarketQuote(model.SideTypeBuy, "BTCUSDT", 1)

		require.NoError(t, err)
		require.Equal(t, 10000.0, order.Price)
		require.Equal(t, 1.0, order.Quantity)
	})

	t.Run("limit order keeps the requested price", func(t *testing.T) {
		exchange := newTestBinance(t, map[route]binanceHandler{
			"POST /api/v3/order": ok(`{
				"symbol": "BTCUSDT", "orderId": 29, "transactTime": 1600000000000,
				"price": "9000.00", "origQty": "2.00", "executedQty": "0.00",
				"cummulativeQuoteQty": "0.00", "status": "NEW", "timeInForce": "GTC",
				"type": "LIMIT", "side": "BUY"
			}`),
		})

		order, err := exchange.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 2, 9000)

		require.NoError(t, err)
		require.Equal(t, 9000.0, order.Price)
		require.Equal(t, 2.0, order.Quantity)
		require.Equal(t, model.OrderStatusTypeNew, order.Status)
	})

	t.Run("stop order", func(t *testing.T) {
		exchange := newTestBinance(t, map[route]binanceHandler{
			"POST /api/v3/order": ok(`{
				"symbol": "BTCUSDT", "orderId": 30, "transactTime": 1600000000000,
				"price": "8000.00", "origQty": "1.00", "executedQty": "0.00",
				"cummulativeQuoteQty": "0.00", "status": "NEW", "timeInForce": "GTC",
				"type": "STOP_LOSS", "side": "SELL"
			}`),
		})

		order, err := exchange.CreateOrderStop("BTCUSDT", 1, 8000)

		require.NoError(t, err)
		require.Equal(t, 8000.0, order.Price)
		require.Equal(t, model.SideTypeSell, order.Side)
	})

	t.Run("OCO order returns both legs and keeps the stop price", func(t *testing.T) {
		exchange := newTestBinance(t, map[route]binanceHandler{
			"POST /api/v3/order/oco": ok(`{
				"orderListId": 5,
				"contingencyType": "OCO",
				"transactionTime": 1600000000000,
				"symbol": "BTCUSDT",
				"orders": [{"symbol": "BTCUSDT", "orderId": 31}, {"symbol": "BTCUSDT", "orderId": 32}],
				"orderReports": [
					{"symbol": "BTCUSDT", "orderId": 31, "orderListId": 5, "price": "12000.00",
					 "origQty": "1.00", "status": "NEW", "type": "LIMIT_MAKER", "side": "SELL"},
					{"symbol": "BTCUSDT", "orderId": 32, "orderListId": 5, "price": "8000.00",
					 "stopPrice": "8100.00", "origQty": "1.00", "status": "NEW",
					 "type": "STOP_LOSS_LIMIT", "side": "SELL"}
				]
			}`),
		})

		orders, err := exchange.CreateOrderOCO(model.SideTypeSell, "BTCUSDT", 1, 12000, 8100, 8000)

		require.NoError(t, err)
		require.Len(t, orders, 2)

		limit, stop := orders[0], orders[1]
		require.Equal(t, model.OrderTypeLimitMaker, limit.Type)
		require.Nil(t, limit.Stop)
		require.NotNil(t, limit.GroupID)
		require.Equal(t, int64(5), *limit.GroupID)

		require.Equal(t, model.OrderTypeStopLossLimit, stop.Type)
		require.NotNil(t, stop.Stop)
		require.Equal(t, 8100.0, *stop.Stop)
	})

	t.Run("rejects invalid quantities before hitting the exchange", func(t *testing.T) {
		exchange := newTestBinance(t, nil) // no order route: a request would 404

		_, err := exchange.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 0.00001)
		require.ErrorIs(t, err, ErrInvalidQuantity)

		_, err = exchange.CreateOrderMarketQuote(model.SideTypeBuy, "BTCUSDT", 0.00001)
		require.ErrorIs(t, err, ErrInvalidQuantity)

		_, err = exchange.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 0.00001, 100)
		require.ErrorIs(t, err, ErrInvalidQuantity)

		_, err = exchange.CreateOrderStop("BTCUSDT", 0.00001, 100)
		require.ErrorIs(t, err, ErrInvalidQuantity)

		_, err = exchange.CreateOrderOCO(model.SideTypeSell, "BTCUSDT", 0.00001, 1, 1, 1)
		require.ErrorIs(t, err, ErrInvalidQuantity)
	})

	t.Run("propagates exchange errors", func(t *testing.T) {
		exchange := newTestBinance(t, map[route]binanceHandler{
			"POST /api/v3/order":     failure("insufficient balance"),
			"POST /api/v3/order/oco": failure("insufficient balance"),
		})

		_, err := exchange.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 1)
		require.ErrorContains(t, err, "insufficient balance")

		_, err = exchange.CreateOrderMarketQuote(model.SideTypeBuy, "BTCUSDT", 1)
		require.ErrorContains(t, err, "insufficient balance")

		_, err = exchange.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 1, 100)
		require.ErrorContains(t, err, "insufficient balance")

		_, err = exchange.CreateOrderStop("BTCUSDT", 1, 100)
		require.ErrorContains(t, err, "insufficient balance")

		_, err = exchange.CreateOrderOCO(model.SideTypeSell, "BTCUSDT", 1, 1, 1, 1)
		require.ErrorContains(t, err, "insufficient balance")
	})
}

func TestBinanceOrders(t *testing.T) {
	const orderResponse = `{
		"symbol": "BTCUSDT", "orderId": 40, "orderListId": -1, "price": "9000.00",
		"origQty": "1.00", "executedQty": "0.00", "cummulativeQuoteQty": "0.00",
		"status": "NEW", "timeInForce": "GTC", "type": "LIMIT", "side": "BUY",
		"stopPrice": "0.00", "time": 1600000000000, "updateTime": 1600000000000
	}`

	t.Run("Order fetches a single order", func(t *testing.T) {
		exchange := newTestBinance(t, map[route]binanceHandler{
			"GET /api/v3/order": ok(orderResponse),
		})

		order, err := exchange.Order("BTCUSDT", 40)

		require.NoError(t, err)
		require.Equal(t, int64(40), order.ExchangeID)
		require.Equal(t, 9000.0, order.Price)
	})

	t.Run("Orders lists the recent orders", func(t *testing.T) {
		exchange := newTestBinance(t, map[route]binanceHandler{
			"GET /api/v3/allOrders": ok("[" + orderResponse + "]"),
		})

		orders, err := exchange.Orders("BTCUSDT", 10)

		require.NoError(t, err)
		require.Len(t, orders, 1)
		require.Equal(t, int64(40), orders[0].ExchangeID)
	})

	t.Run("Cancel removes an open order", func(t *testing.T) {
		exchange := newTestBinance(t, map[route]binanceHandler{
			"DELETE /api/v3/order": ok(`{"symbol": "BTCUSDT", "orderId": 40, "status": "CANCELED"}`),
		})

		require.NoError(t, exchange.Cancel(model.Order{Pair: "BTCUSDT", ExchangeID: 40}))
	})

	t.Run("propagates exchange errors", func(t *testing.T) {
		exchange := newTestBinance(t, map[route]binanceHandler{
			"GET /api/v3/order":     failure("order not found"),
			"GET /api/v3/allOrders": failure("order not found"),
			"DELETE /api/v3/order":  failure("order not found"),
		})

		_, err := exchange.Order("BTCUSDT", 40)
		require.ErrorContains(t, err, "order not found")

		_, err = exchange.Orders("BTCUSDT", 10)
		require.ErrorContains(t, err, "order not found")

		require.ErrorContains(t, exchange.Cancel(model.Order{Pair: "BTCUSDT"}), "order not found")
	})
}

func TestBinanceAccount(t *testing.T) {
	const accountResponse = `{
		"makerCommission": 10, "takerCommission": 10, "canTrade": true,
		"balances": [
			{"asset": "BTC", "free": "1.50000000", "locked": "0.50000000"},
			{"asset": "USDT", "free": "1000.00000000", "locked": "0.00000000"}
		]
	}`

	t.Run("maps the balances", func(t *testing.T) {
		exchange := newTestBinance(t, map[route]binanceHandler{
			"GET /api/v3/account": ok(accountResponse),
		})

		account, err := exchange.Account()

		require.NoError(t, err)
		require.Len(t, account.Balances, 2)
		require.Equal(t, 1.5, account.Balances[0].Free)
		require.Equal(t, 0.5, account.Balances[0].Lock)
	})

	t.Run("Position sums the free and locked amounts", func(t *testing.T) {
		exchange := newTestBinance(t, map[route]binanceHandler{
			"GET /api/v3/account": ok(accountResponse),
		})

		asset, quote, err := exchange.Position("BTCUSDT")

		require.NoError(t, err)
		require.Equal(t, 2.0, asset)
		require.Equal(t, 1000.0, quote)
	})

	t.Run("fails on a malformed balance", func(t *testing.T) {
		exchange := newTestBinance(t, map[route]binanceHandler{
			"GET /api/v3/account": ok(`{"balances": [{"asset": "BTC", "free": "abc", "locked": "0"}]}`),
		})

		_, err := exchange.Account()
		require.Error(t, err)
	})

	t.Run("fails on a malformed locked balance", func(t *testing.T) {
		exchange := newTestBinance(t, map[route]binanceHandler{
			"GET /api/v3/account": ok(`{"balances": [{"asset": "BTC", "free": "1", "locked": "abc"}]}`),
		})

		_, err := exchange.Account()
		require.Error(t, err)
	})

	t.Run("propagates exchange errors", func(t *testing.T) {
		exchange := newTestBinance(t, map[route]binanceHandler{
			"GET /api/v3/account": failure("invalid signature"),
		})

		_, err := exchange.Account()
		require.ErrorContains(t, err, "invalid signature")

		_, _, err = exchange.Position("BTCUSDT")
		require.ErrorContains(t, err, "invalid signature")
	})
}

func TestCandleFromKline(t *testing.T) {
	candle := CandleFromKline("BTCUSDT", binance.Kline{
		OpenTime: 1600000000000,
		Open:     "100.5",
		Close:    "105.5",
		High:     "110.5",
		Low:      "95.5",
		Volume:   "12.5",
	})

	require.Equal(t, "BTCUSDT", candle.Pair)
	require.Equal(t, time.Unix(1600000000, 0), candle.Time)
	require.Equal(t, 100.5, candle.Open)
	require.Equal(t, 105.5, candle.Close)
	require.Equal(t, 110.5, candle.High)
	require.Equal(t, 95.5, candle.Low)
	require.Equal(t, 12.5, candle.Volume)
	require.True(t, candle.Complete, "closed klines are always complete")
	require.NotNil(t, candle.Metadata)
}

func TestCandleFromWsKline(t *testing.T) {
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
			candle := CandleFromWsKline("BTCUSDT", binance.WsKline{
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

// The fake server replies with JSON, so a decoding failure would point at a
// malformed fixture rather than at the exchange wrapper.
func TestFixturesAreValidJSON(t *testing.T) {
	for name, fixture := range map[string]string{
		"exchangeInfo": exchangeInfoResponse,
		"klines":       klinesResponse,
		"marketOrder":  marketOrderResponse,
	} {
		t.Run(name, func(t *testing.T) {
			var target any
			require.NoError(t, json.Unmarshal([]byte(fixture), &target))
		})
	}
}

func TestBinanceFormat(t *testing.T) {
	exchange := newTestBinance(t, nil)

	t.Run("rounds to the pair precision", func(t *testing.T) {
		require.Equal(t, "1.111", exchange.formatQuantity("BTCUSDT", 1.1111111))
		require.Equal(t, "9000.12", exchange.formatPrice("BTCUSDT", 9000.123456))
	})

	t.Run("falls back to the raw value for unknown pairs", func(t *testing.T) {
		require.Equal(t, "1.1111111", exchange.formatQuantity("UNKNOWN", 1.1111111))
		require.Equal(t, "1.1111111", exchange.formatPrice("UNKNOWN", 1.1111111))
	})
}

// A malformed numeric field in the exchange response must surface as an error
// instead of a silently zeroed order.
func TestBinanceCreateOrderMalformedResponse(t *testing.T) {
	tests := []struct {
		name string
		body string
		run  func(*Binance) error
	}{
		{
			name: "limit order with an invalid price",
			body: `{"symbol":"BTCUSDT","orderId":1,"price":"abc","origQty":"1","status":"NEW","type":"LIMIT","side":"BUY"}`,
			run: func(b *Binance) error {
				_, err := b.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 1, 100)
				return err
			},
		},
		{
			name: "limit order with an invalid quantity",
			body: `{"symbol":"BTCUSDT","orderId":1,"price":"100","origQty":"abc","status":"NEW","type":"LIMIT","side":"BUY"}`,
			run: func(b *Binance) error {
				_, err := b.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 1, 100)
				return err
			},
		},
		{
			name: "market order with an invalid cost",
			body: `{"symbol":"BTCUSDT","orderId":1,"cummulativeQuoteQty":"abc","executedQty":"1","status":"FILLED","type":"MARKET","side":"BUY"}`,
			run: func(b *Binance) error {
				_, err := b.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 1)
				return err
			},
		},
		{
			name: "market order with an invalid quantity",
			body: `{"symbol":"BTCUSDT","orderId":1,"cummulativeQuoteQty":"100","executedQty":"abc","status":"FILLED","type":"MARKET","side":"BUY"}`,
			run: func(b *Binance) error {
				_, err := b.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 1)
				return err
			},
		},
		{
			name: "market quote order with an invalid cost",
			body: `{"symbol":"BTCUSDT","orderId":1,"cummulativeQuoteQty":"abc","executedQty":"1","status":"FILLED","type":"MARKET","side":"BUY"}`,
			run: func(b *Binance) error {
				_, err := b.CreateOrderMarketQuote(model.SideTypeBuy, "BTCUSDT", 1)
				return err
			},
		},
		{
			name: "market quote order with an invalid quantity",
			body: `{"symbol":"BTCUSDT","orderId":1,"cummulativeQuoteQty":"100","executedQty":"abc","status":"FILLED","type":"MARKET","side":"BUY"}`,
			run: func(b *Binance) error {
				_, err := b.CreateOrderMarketQuote(model.SideTypeBuy, "BTCUSDT", 1)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exchange := newTestBinance(t, map[route]binanceHandler{"POST /api/v3/order": ok(tt.body)})

			require.Error(t, tt.run(exchange))
		})
	}
}

// Both custom endpoint options abort the process when a URL is missing.
func TestCustomAPIEndpointRequiresEveryURL(t *testing.T) {
	tests := map[string]func(){
		"main":    func() { WithCustomMainAPIEndpoint("", "ws://ws", "ws://combined") },
		"testnet": func() { WithCustomTestnetAPIEndpoint("http://api", "", "ws://combined") },
	}

	for name, configure := range tests {
		t.Run(name, func(t *testing.T) {
			logger := logrus.StandardLogger()
			previousOut, previousExit := logger.Out, logger.ExitFunc

			var buffer bytes.Buffer
			var exitCode int
			logger.Out = &buffer
			logger.ExitFunc = func(code int) { exitCode = code }
			t.Cleanup(func() { logger.Out, logger.ExitFunc = previousOut, previousExit })

			configure()

			require.Equal(t, 1, exitCode, "a missing URL must abort the bot")
			require.Contains(t, buffer.String(), "missing url parameters")
		})
	}
}

func TestFormatQuantity(t *testing.T) {
	binance := Binance{assetsInfo: map[string]model.AssetInfo{
		"BTCUSDT": {
			StepSize:           0.00001000,
			TickSize:           0.00001000,
			BaseAssetPrecision: 5,
			QuotePrecision:     5,
		},
		"BATUSDT": {
			StepSize:           0.01,
			TickSize:           0.01,
			BaseAssetPrecision: 2,
			QuotePrecision:     2,
		},
	}}

	tt := []struct {
		pair     string
		quantity float64
		expected string
	}{
		{"BTCUSDT", 1.1, "1.1"},
		{"BTCUSDT", 11, "11"},
		{"BTCUSDT", 11, "11"},
		{"BTCUSDT", 1.1111111111, "1.11111"},
		{"BTCUSDT", 1.9999999999999, "1.99999"},
		{"BTCUSDT", 1111111.1111111111, "1111111.11111"},
		{"BATUSDT", 111.111, "111.11"},
		{"BATUSDT", 9.9999999999, "9.99"},
		{"BATUSDT", 9.9999999999, "9.99"},
		{"BATUSDT", 10, "10"},
		{"BATUSDT", 10.11111, "10.11"},
		{"BATUSDT", 0.01, "0.01"},
	}

	for _, tc := range tt {
		t.Run(fmt.Sprintf("given %f %s", tc.quantity, tc.pair), func(t *testing.T) {
			require.Equal(t, tc.expected, binance.formatQuantity(tc.pair, tc.quantity))
			require.Equal(t, tc.expected, binance.formatPrice(tc.pair, tc.quantity))
		})
	}
}

func TestNewOrder(t *testing.T) {
	t.Run("filled order reports the average fill price", func(t *testing.T) {
		order := newOrder(&binance.Order{
			OrderID:                  10,
			Symbol:                   "BTCUSDT",
			Side:                     binance.SideTypeSell,
			Type:                     binance.OrderTypeStopLossLimit,
			Status:                   binance.OrderStatusTypeFilled,
			Price:                    "900",
			StopPrice:                "950",
			OrigQuantity:             "2",
			ExecutedQuantity:         "2",
			CummulativeQuoteQuantity: "1900",
			OrderListId:              7,
		})

		require.Equal(t, 950.0, order.Price) // 1900 / 2
		require.Equal(t, 2.0, order.Quantity)
		require.NotNil(t, order.Stop)
		require.Equal(t, 950.0, *order.Stop)
		require.NotNil(t, order.GroupID)
		require.Equal(t, int64(7), *order.GroupID)
	})

	t.Run("open order keeps the requested price and no group", func(t *testing.T) {
		order := newOrder(&binance.Order{
			OrderID:                  11,
			Symbol:                   "BTCUSDT",
			Side:                     binance.SideTypeBuy,
			Type:                     binance.OrderTypeLimit,
			Status:                   binance.OrderStatusTypeNew,
			Price:                    "900",
			StopPrice:                "0.00000000",
			OrigQuantity:             "2",
			ExecutedQuantity:         "0.00000000",
			CummulativeQuoteQuantity: "0.00000000",
			OrderListId:              -1,
		})

		require.Equal(t, 900.0, order.Price)
		require.Equal(t, 2.0, order.Quantity)
		require.Nil(t, order.Stop)
		require.Nil(t, order.GroupID)
	})
}
