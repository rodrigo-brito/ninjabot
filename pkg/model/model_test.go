package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCandle_ToSlice(t *testing.T) {
	candle := Candle{
		Symbol:   "BTCUSDT",
		Time:     time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		Open:     10000.1,
		Close:    10000.1,
		Low:      10000.1,
		High:     10000.1,
		Volume:   10000.1,
		Trades:   10000,
		Complete: true,
	}
	require.Equal(t, []string{"1609459200", "10000.100000", "10000.100000", "10000.100000", "10000.100000",
		"10000.1", "10000"}, candle.ToSlice())
}
