package exchange

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitAssetQuote(t *testing.T) {
	tt := []struct {
		Symbol string
		Asset  string
		Quote  string
	}{
		{"BTCUSDT", "BTC", "USDT"},
		{"ETHBTC", "ETC", "BTC"},
		{"BTCBUSD", "BTC", "BUSD"},
	}

	for _, tc := range tt {
		asset, quote := SplitAssetQuote(tc.Symbol)
		require.Equal(t, tc.Asset, asset)
		require.Equal(t, tc.Quote, quote)
	}
}
