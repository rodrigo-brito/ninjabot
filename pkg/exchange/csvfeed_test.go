package exchange

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewCSVFeed(t *testing.T) {
	feed, err := NewCSVFeed("1d", PairFeed{
		Pair: "BTCUSDT",
		File: "../../testdata/btc-1d.csv",
	})
	candle := feed.CandlePairTimeFrame["BTCUSDT--1d"][0]
	require.NoError(t, err)
	require.Len(t, feed.Timeframes, 1)
	require.Contains(t, feed.Timeframes, "1d")
	require.Len(t, feed.CandlePairTimeFrame["BTCUSDT--1d"], 14)
	require.Equal(t, "2021-04-26 00:00:00", candle.Time.UTC().Format("2006-01-02 15:04:05"))
	require.Equal(t, 49066.76, candle.Open)
	require.Equal(t, 54001.39, candle.Close)
	require.Equal(t, 48753.44, candle.Low)
	require.Equal(t, 54356.62, candle.High)
	require.Equal(t, 86310.8, candle.Volume)
}

func TestCSVFeed_CandlesByLimit(t *testing.T) {
	feed, err := NewCSVFeed("1d", PairFeed{
		Pair: "BTCUSDT",
		File: "../../testdata/btc-1d.csv",
	})
	require.NoError(t, err)
	candles, err := feed.CandlesByLimit(context.Background(), "BTCUSDT", "1d", 1)
	require.Nil(t, err)
	require.Len(t, candles, 1)
	require.Equal(t, "2021-04-26 00:00:00", candles[0].Time.UTC().Format("2006-01-02 15:04:05"))

	// should remove the candle from array
	require.Len(t, feed.CandlePairTimeFrame["BTCUSDT--1d"], 13)
	candle := feed.CandlePairTimeFrame["BTCUSDT--1d"][0]
	require.Equal(t, "2021-04-27 00:00:00", candle.Time.UTC().Format("2006-01-02 15:04:05"))
}
