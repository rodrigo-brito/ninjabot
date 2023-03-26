package exchange

import (
	"testing"

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

func TestUpdatePairFile(t *testing.T) {
	t.Skip() // it is not a test, just an utilitary to update paris list
	err := updateParisFile()
	require.NoError(t, err)
}
