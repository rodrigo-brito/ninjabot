package exchange

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/stretchr/testify/require"
)

// websocketGUID is the constant defined by RFC 6455 for the handshake.
const websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// startWebsocketServer serves a websocket endpoint that pushes the given
// frames to every client and then holds the connection open until the test
// ends. It is enough to drive the kline subscriptions without pulling in a
// websocket library.
func startWebsocketServer(t *testing.T, frames ...string) string {
	t.Helper()

	shutdown := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := acceptWebsocket(w, r)
		if err != nil {
			return
		}
		defer conn.Close()

		for _, frame := range frames {
			if err := writeTextFrame(conn, frame); err != nil {
				return
			}
		}

		// Closing here would make the client reconnect and race with the
		// channels the subscription closes on shutdown.
		<-shutdown
	}))
	t.Cleanup(func() {
		close(shutdown)
		server.Close()
	})

	endpoint, err := url.Parse(server.URL)
	require.NoError(t, err)
	endpoint.Scheme = "ws"

	return endpoint.String()
}

func acceptWebsocket(w http.ResponseWriter, r *http.Request) (net.Conn, error) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijack unsupported", http.StatusInternalServerError)
		return nil, http.ErrNotSupported
	}

	conn, buffer, err := hijacker.Hijack()
	if err != nil {
		return nil, err
	}

	sum := sha1.Sum([]byte(r.Header.Get("Sec-WebSocket-Key") + websocketGUID)) //nolint:gosec // required by RFC 6455
	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + base64.StdEncoding.EncodeToString(sum[:]) + "\r\n\r\n"

	if _, err := buffer.WriteString(response); err != nil {
		conn.Close()
		return nil, err
	}
	if err := buffer.Flush(); err != nil {
		conn.Close()
		return nil, err
	}

	// Drain whatever the client sends (pings, close frames) so its writes
	// never block on a full buffer.
	go func() {
		_, _ = bufio.NewReader(conn).WriteTo(discard{})
	}()

	return conn, nil
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

// writeTextFrame writes an unmasked text frame, as a server must.
func writeTextFrame(conn net.Conn, payload string) error {
	header := []byte{0x81} // FIN + text opcode

	switch size := len(payload); {
	case size < 126:
		header = append(header, byte(size))
	default:
		header = append(header, 126, byte(size>>8), byte(size))
	}

	if _, err := conn.Write(append(header, payload...)); err != nil {
		return err
	}

	return nil
}

const klineEvent = `{
	"e": "kline", "E": 1600000000000, "s": "BTCUSDT",
	"k": {
		"t": 1600000000000, "T": 1600003599999, "s": "BTCUSDT", "i": "1h",
		"o": "100.0", "c": "105.0", "h": "110.0", "l": "90.0", "v": "10.0", "x": true
	}
}`

// useWebsocketEndpoints points both the spot and futures streams at url.
func useWebsocketEndpoints(t *testing.T, url string) {
	t.Helper()

	spot, future := binance.BaseWsMainURL, futures.BaseWsMarketMainUrl
	binance.BaseWsMainURL, futures.BaseWsMarketMainUrl = url, url

	t.Cleanup(func() {
		binance.BaseWsMainURL, futures.BaseWsMarketMainUrl = spot, future
	})
}

func TestBinanceCandlesSubscription(t *testing.T) {
	t.Run("streams the candles of a pair", func(t *testing.T) {
		useWebsocketEndpoints(t, startWebsocketServer(t, klineEvent))

		exchange := &Binance{
			MetadataFetchers: []MetadataFetchers{
				func(string, time.Time) (string, float64) { return "funding", 0.01 },
			},
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		candles, errs := exchange.CandlesSubscription(ctx, "BTCUSDT", "1h")

		select {
		case candle := <-candles:
			require.Equal(t, "BTCUSDT", candle.Pair)
			require.Equal(t, 105.0, candle.Close)
			require.True(t, candle.Complete)
			require.Equal(t, 0.01, candle.Metadata["funding"], "the metadata fetchers run on closed candles")
		case err := <-errs:
			t.Fatalf("unexpected error: %v", err)
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for a candle")
		}
	})

	t.Run("converts to Heikin Ashi when enabled", func(t *testing.T) {
		useWebsocketEndpoints(t, startWebsocketServer(t, klineEvent))

		exchange := &Binance{HeikinAshi: true}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		candles, _ := exchange.CandlesSubscription(ctx, "BTCUSDT", "1h")

		select {
		case candle := <-candles:
			require.NotEqual(t, 105.0, candle.Close)
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for a candle")
		}
	})

	t.Run("reports connection failures", func(t *testing.T) {
		useWebsocketEndpoints(t, "ws://127.0.0.1:1")

		exchange := &Binance{}
		candles, errs := exchange.CandlesSubscription(context.Background(), "BTCUSDT", "1h")

		select {
		case err := <-errs:
			require.Error(t, err)
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for the connection error")
		}

		_, open := <-candles
		require.False(t, open, "the candle channel is closed after a fatal error")
	})

	t.Run("stops when the context is canceled", func(t *testing.T) {
		useWebsocketEndpoints(t, startWebsocketServer(t))

		exchange := &Binance{}
		ctx, cancel := context.WithCancel(context.Background())

		candles, _ := exchange.CandlesSubscription(ctx, "BTCUSDT", "1h")
		cancel()

		select {
		case _, open := <-candles:
			require.False(t, open, "the candle channel is closed on shutdown")
		case <-time.After(3 * time.Second):
			t.Fatal("the subscription did not stop")
		}
	})
}

func TestBinanceFutureCandlesSubscription(t *testing.T) {
	t.Run("streams the candles of a pair", func(t *testing.T) {
		useWebsocketEndpoints(t, startWebsocketServer(t, klineEvent))

		exchange := &BinanceFuture{
			MetadataFetchers: []MetadataFetchers{
				func(string, time.Time) (string, float64) { return "funding", 0.02 },
			},
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		candles, errs := exchange.CandlesSubscription(ctx, "BTCUSDT", "1h")

		select {
		case candle := <-candles:
			require.Equal(t, 105.0, candle.Close)
			require.Equal(t, 0.02, candle.Metadata["funding"])
		case err := <-errs:
			t.Fatalf("unexpected error: %v", err)
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for a candle")
		}
	})

	t.Run("converts to Heikin Ashi when enabled", func(t *testing.T) {
		useWebsocketEndpoints(t, startWebsocketServer(t, klineEvent))

		exchange := &BinanceFuture{HeikinAshi: true}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		candles, _ := exchange.CandlesSubscription(ctx, "BTCUSDT", "1h")

		select {
		case candle := <-candles:
			require.NotEqual(t, 105.0, candle.Close)
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for a candle")
		}
	})

	t.Run("reports connection failures", func(t *testing.T) {
		useWebsocketEndpoints(t, "ws://127.0.0.1:1")

		exchange := &BinanceFuture{}
		candles, errs := exchange.CandlesSubscription(context.Background(), "BTCUSDT", "1h")

		select {
		case err := <-errs:
			require.Error(t, err)
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for the connection error")
		}

		_, open := <-candles
		require.False(t, open)
	})

	t.Run("stops when the context is canceled", func(t *testing.T) {
		useWebsocketEndpoints(t, startWebsocketServer(t))

		exchange := &BinanceFuture{}
		ctx, cancel := context.WithCancel(context.Background())

		candles, _ := exchange.CandlesSubscription(ctx, "BTCUSDT", "1h")
		cancel()

		select {
		case _, open := <-candles:
			require.False(t, open)
		case <-time.After(3 * time.Second):
			t.Fatal("the subscription did not stop")
		}
	})
}
