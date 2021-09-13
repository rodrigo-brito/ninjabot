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
		{"ETHBTC", "ETH", "BTC"},
		{"BTCBUSD", "BTC", "BUSD"},
	}

	for _, tc := range tt {
		t.Run(tc.Symbol, func(t *testing.T) {
			asset, quote := SplitAssetQuote(tc.Symbol)
			require.Equal(t, tc.Asset, asset)
			require.Equal(t, tc.Quote, quote)
		})
	}
}
