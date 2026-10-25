package exchange

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/adshao/go-binance/v2"
	"github.com/stretchr/testify/require"
)

func TestSplitAssetQuote(t *testing.T) {
	tt := []struct {
		Pair  string
		Asset string
		Quote string
	}{
		{"BTCUSDT", "BTC", "USDT"},
		{"ETHBTC", "ETH", "BTC"},
		{"BTCBUSD", "BTC", "BUSD"},
		{"1000SHIBBUSD", "1000SHIB", "BUSD"},
	}

	for _, tc := range tt {
		t.Run(tc.Pair, func(t *testing.T) {
			asset, quote := SplitAssetQuote(tc.Pair)
			require.Equal(t, tc.Asset, asset)
			require.Equal(t, tc.Quote, quote)
		})
	}
}

// useTempWorkdir runs the test in a scratch directory, so that updatePairsFile
// never overwrites the pairs.json of the repository.
func useTempWorkdir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	previous, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(previous)) })

	return dir
}

// preservePairs restores the embedded pair map after the test.
func preservePairs(t *testing.T) {
	t.Helper()

	previous := maps.Clone(pairAssetQuoteMap)
	t.Cleanup(func() { pairAssetQuoteMap = previous })
}

func TestUpdatePairsFile(t *testing.T) {
	const spotInfo = `{
		"timezone": "UTC", "serverTime": 1600000000000,
		"symbols": [{"symbol": "AAABBB", "status": "TRADING", "baseAsset": "AAA",
			"baseAssetPrecision": 8, "quoteAsset": "BBB", "quotePrecision": 8,
			"quoteAssetPrecision": 8, "filters": []}]
	}`
	const futuresInfo = `{
		"timezone": "UTC", "serverTime": 1600000000000,
		"symbols": [{"symbol": "CCCDDD", "status": "TRADING", "baseAsset": "CCC",
			"baseAssetPrecision": 8, "quoteAsset": "DDD", "quotePrecision": 8,
			"quotePrecisionInt": 8, "filters": []}]
	}`

	t.Run("merges the spot and futures pairs into pairs.json", func(t *testing.T) {
		preservePairs(t)
		dir := useTempWorkdir(t)

		spot := newBinanceServer(t, map[route]binanceHandler{"GET /api/v3/exchangeInfo": ok(spotInfo)})
		restoreBinanceEndpoints(t)
		binance.BaseAPIMainURL = spot.URL

		futuresServer := newFuturesServer(t, map[route]binanceHandler{
			"GET /fapi/v1/exchangeInfo": ok(futuresInfo),
		})
		useFuturesServer(t, futuresServer.URL)

		require.NoError(t, updatePairsFile())

		content, err := os.ReadFile(filepath.Join(dir, "pairs.json"))
		require.NoError(t, err)

		var pairs map[string]AssetQuote
		require.NoError(t, json.Unmarshal(content, &pairs))
		require.Equal(t, AssetQuote{Asset: "AAA", Quote: "BBB"}, pairs["AAABBB"])
		require.Equal(t, AssetQuote{Asset: "CCC", Quote: "DDD"}, pairs["CCCDDD"])
		require.Contains(t, pairs, "BTCUSDT", "the known pairs are kept")
	})

	t.Run("fails when the spot API is unreachable", func(t *testing.T) {
		preservePairs(t)
		useTempWorkdir(t)

		restoreBinanceEndpoints(t)
		binance.BaseAPIMainURL = "http://127.0.0.1:1"

		require.ErrorContains(t, updatePairsFile(), "failed to get exchange info")
	})

	t.Run("fails when the futures API is unreachable", func(t *testing.T) {
		preservePairs(t)
		useTempWorkdir(t)

		spot := newBinanceServer(t, map[route]binanceHandler{"GET /api/v3/exchangeInfo": ok(spotInfo)})
		restoreBinanceEndpoints(t)
		binance.BaseAPIMainURL = spot.URL
		useFuturesServer(t, "http://127.0.0.1:1")

		require.ErrorContains(t, updatePairsFile(), "failed to get exchange info")
	})

	t.Run("fails when the file cannot be written", func(t *testing.T) {
		preservePairs(t)
		dir := useTempWorkdir(t)

		spot := newBinanceServer(t, map[route]binanceHandler{"GET /api/v3/exchangeInfo": ok(spotInfo)})
		restoreBinanceEndpoints(t)
		binance.BaseAPIMainURL = spot.URL

		futuresServer := newFuturesServer(t, map[route]binanceHandler{
			"GET /fapi/v1/exchangeInfo": ok(futuresInfo),
		})
		useFuturesServer(t, futuresServer.URL)

		// A directory named pairs.json makes the write fail.
		require.NoError(t, os.Mkdir(filepath.Join(dir, "pairs.json"), 0o755))

		require.ErrorContains(t, updatePairsFile(), "failed to write to file")
	})
}

// The embedded pairs.json must stay parseable: init panics otherwise.
func TestEmbeddedPairs(t *testing.T) {
	var parsed map[string]AssetQuote
	require.NoError(t, json.Unmarshal(pairs, &parsed))
	require.NotEmpty(t, parsed)

	asset, quote := SplitAssetQuote("BTCUSDT")
	require.Equal(t, "BTC", asset)
	require.Equal(t, "USDT", quote)
}
