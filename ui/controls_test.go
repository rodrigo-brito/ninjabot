package ui

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/order"
)

// fakeController records calls and returns canned balances / orders.
type fakeController struct {
	status       order.Status
	asset, quote float64
	positionErr  error
	orderErr     error

	started, stopped bool
	lastSide         model.SideType
	lastPair         string
	lastAmount       float64
	marketCalls      int
	quoteCalls       int
}

func (f *fakeController) Status() order.Status { return f.status }
func (f *fakeController) Start()               { f.started = true; f.status = order.StatusRunning }
func (f *fakeController) Stop()                { f.stopped = true; f.status = order.StatusStopped }

func (f *fakeController) Position(string) (float64, float64, error) {
	return f.asset, f.quote, f.positionErr
}

func (f *fakeController) CreateOrderMarket(side model.SideType, pair string, size float64) (model.Order, error) {
	f.marketCalls++
	f.lastSide, f.lastPair, f.lastAmount = side, pair, size
	if f.orderErr != nil {
		return model.Order{}, f.orderErr
	}
	return model.Order{ID: 1, Pair: pair, Side: side, Quantity: size}, nil
}

func (f *fakeController) CreateOrderMarketQuote(side model.SideType, pair string, amount float64) (model.Order, error) {
	f.quoteCalls++
	f.lastSide, f.lastPair, f.lastAmount = side, pair, amount
	if f.orderErr != nil {
		return model.Order{}, f.orderErr
	}
	return model.Order{ID: 2, Pair: pair, Side: side, Quantity: amount}, nil
}

func postJSON(t *testing.T, url, body string) (*http.Response, string) {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, string(data)
}

func decodeControls(t *testing.T, body string) ControlsResponse {
	t.Helper()
	var response ControlsResponse
	require.NoError(t, json.Unmarshal([]byte(body), &response))
	return response
}

func TestControls_Disabled(t *testing.T) {
	c := newTestChart(t)
	srv := newTestServer(t, c, nil)

	resp, body := get(t, srv.URL+"/api/controls")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, ControlsResponse{Enabled: false, Status: ""}, decodeControls(t, body))

	for _, path := range []string{"/controls/start", "/controls/stop", "/controls/order"} {
		resp, body := postJSON(t, srv.URL+"/api"+path, "{}")
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, path)
		assert.Contains(t, body, "not enabled")
	}
}

func TestControls_StatusStartStop(t *testing.T) {
	c := newTestChart(t)
	controller := &fakeController{status: order.StatusRunning}
	c.SetOrderController(controller)
	srv := newTestServer(t, c, nil)

	resp, body := get(t, srv.URL+"/api/controls")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, ControlsResponse{Enabled: true, Status: "running"}, decodeControls(t, body))

	resp, body = postJSON(t, srv.URL+"/api/controls/stop", "")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.True(t, controller.stopped)
	assert.Equal(t, ControlsResponse{Enabled: true, Status: "stopped"}, decodeControls(t, body))

	resp, body = postJSON(t, srv.URL+"/api/controls/start", "")
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.True(t, controller.started)
	assert.Equal(t, ControlsResponse{Enabled: true, Status: "running"}, decodeControls(t, body))
}

func TestControls_OrderValidation(t *testing.T) {
	c := newTestChart(t)
	c.SetOrderController(&fakeController{})
	c.OnCandle(candleAt("BTCUSDT", time.Date(2021, 9, 26, 20, 0, 0, 0, time.UTC), 40000))
	srv := newTestServer(t, c, nil)

	testCases := []struct {
		name, body, wantError string
		wantStatus            int
	}{
		{"invalid json", "{", "invalid JSON", http.StatusBadRequest},
		{"unknown pair", `{"pair":"DOGEUSDT","side":"buy","amount":10}`, "unknown pair", http.StatusNotFound},
		{"zero amount", `{"pair":"BTCUSDT","side":"buy","amount":0}`, "amount must be positive", http.StatusBadRequest},
		{"percent above 100", `{"pair":"BTCUSDT","side":"buy","amount":150,"percent":true}`,
			"percentage cannot exceed 100", http.StatusBadRequest},
		{"invalid side", `{"pair":"BTCUSDT","side":"hold","amount":10}`, "side must be", http.StatusBadRequest},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, body := postJSON(t, srv.URL+"/api/controls/order", tc.body)
			assert.Equal(t, tc.wantStatus, resp.StatusCode)
			assert.Contains(t, body, tc.wantError)
		})
	}
}

func TestControls_OrderSemantics(t *testing.T) {
	base := time.Date(2021, 9, 26, 20, 0, 0, 0, time.UTC)

	testCases := []struct {
		name            string
		body            string
		wantSide        model.SideType
		wantAmount      float64
		wantMarketCalls int
		wantQuoteCalls  int
	}{
		{"buy quote amount", `{"pair":"btcusdt","side":"buy","amount":100}`,
			model.SideTypeBuy, 100, 0, 1},
		{"buy percent of quote balance", `{"pair":"BTCUSDT","side":"BUY","amount":50,"percent":true}`,
			model.SideTypeBuy, 500, 0, 1},
		{"sell quote amount", `{"pair":"BTCUSDT","side":"sell","amount":25}`,
			model.SideTypeSell, 25, 0, 1},
		{"sell percent of asset balance", `{"pair":"BTCUSDT","side":"sell","amount":50,"percent":true}`,
			model.SideTypeSell, 1, 1, 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestChart(t)
			controller := &fakeController{asset: 2, quote: 1000}
			c.SetOrderController(controller)
			c.OnCandle(candleAt("BTCUSDT", base, 40000))
			srv := newTestServer(t, c, nil)

			resp, body := postJSON(t, srv.URL+"/api/controls/order", tc.body)
			require.Equal(t, http.StatusOK, resp.StatusCode, body)

			assert.Equal(t, tc.wantSide, controller.lastSide)
			assert.Equal(t, "BTCUSDT", controller.lastPair)
			assert.InDelta(t, tc.wantAmount, controller.lastAmount, 1e-9)
			assert.Equal(t, tc.wantMarketCalls, controller.marketCalls)
			assert.Equal(t, tc.wantQuoteCalls, controller.quoteCalls)

			var created Order
			require.NoError(t, json.Unmarshal([]byte(body), &created))
			assert.Equal(t, "BTCUSDT", created.Pair)
		})
	}
}

func TestControls_OrderErrors(t *testing.T) {
	base := time.Date(2021, 9, 26, 20, 0, 0, 0, time.UTC)

	c := newTestChart(t)
	controller := &fakeController{orderErr: order.ErrBotStopped}
	c.SetOrderController(controller)
	c.OnCandle(candleAt("BTCUSDT", base, 40000))
	srv := newTestServer(t, c, nil)

	resp, body := postJSON(t, srv.URL+"/api/controls/order", `{"pair":"BTCUSDT","side":"buy","amount":10}`)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	assert.Contains(t, body, "stopped")

	controller.orderErr = assert.AnError
	resp, body = postJSON(t, srv.URL+"/api/controls/order", `{"pair":"BTCUSDT","side":"buy","amount":10}`)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Contains(t, body, assert.AnError.Error())
}
