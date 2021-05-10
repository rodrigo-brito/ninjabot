package exchange

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewCSVFeed(t *testing.T) {
	feed, err := NewCSVFeed(PairFeed{
		Pair:      "BTCUSDT",
		File:      "../../testdata/btc-1d.csv",
		Timeframe: "1d",
	})
	candle := feed.CandlePairTimeFrame["BTCUSDT--1d"][0]
	require.NoError(t, err)
	require.Len(t, feed.Timeframes, 1)
	require.Contains(t, feed.Timeframes, "1d")
	require.Len(t, feed.CandlePairTimeFrame["BTCUSDT--1d"], 14)
	require.Equal(t, "2021-04-25 21:00:00", candle.Time.Format("2006-01-02 15:04:05"))
	require.Equal(t, 49066.76, candle.Open)
	require.Equal(t, 54001.39, candle.Close)
	require.Equal(t, 48753.44, candle.Low)
	require.Equal(t, 54356.62, candle.High)
	require.Equal(t, 86310.8, candle.Volume)
}

func TestCSVFeed_CandlesByLimit(t *testing.T) {
	feed, err := NewCSVFeed(PairFeed{
		Pair:      "BTCUSDT",
		File:      "../../testdata/btc-1d.csv",
		Timeframe: "1d",
	})
	require.NoError(t, err)
	canldes, err := feed.CandlesByLimit(context.Background(), "BTCUSDT", "1d", 5)
	require.Nil(t, err)
	require.Len(t, canldes, 5)
}
