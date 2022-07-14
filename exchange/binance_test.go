package exchange

import (
	"fmt"
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/stretchr/testify/require"
	"testing"
)

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
		{"BTCUSDT", 1.1111111111, "1.11111"},
		{"BTCUSDT", 1111111.1111111111, "1111111.11111"},
		{"BATUSDT", 111.111, "111.11"},
		{"BATUSDT", 9.99999, "9.99"},
	}

	for _, tc := range tt {
		t.Run(fmt.Sprintf("given %f %s", tc.quantity, tc.pair), func(t *testing.T) {
			require.Equal(t, tc.expected, binance.formatQuantity(tc.pair, tc.quantity))
			require.Equal(t, tc.expected, binance.formatPrice(tc.pair, tc.quantity))
		})
	}
}
