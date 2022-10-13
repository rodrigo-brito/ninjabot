package exchange

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCSVFeed(t *testing.T) {
	t.Run("no header", func(t *testing.T) {
		feed, err := NewCSVFeed("1d", PairFeed{
			Timeframe: "1d",
			Pair:      "BTCUSDT",
			File:      "../testdata/btc-1d.csv",
		})

		candle := feed.CandlePairTimeFrame["BTCUSDT--1d"][0]
		require.NoError(t, err)
		require.Len(t, feed.CandlePairTimeFrame["BTCUSDT--1d"], 14)
		require.Equal(t, "2021-04-26 00:00:00", candle.Time.UTC().Format("2006-01-02 15:04:05"))
		require.Equal(t, 49066.76, candle.Open)
		require.Equal(t, 54001.39, candle.Close)
		require.Equal(t, 48753.44, candle.Low)
		require.Equal(t, 54356.62, candle.High)
		require.Equal(t, 86310.8, candle.Volume)
	})

	t.Run("with header and custom data", func(t *testing.T) {
		feed, err := NewCSVFeed("1d", PairFeed{
			Timeframe: "1d",
			Pair:      "BTCUSDT",
			File:      "../testdata/btc-1d-header.csv",
		})
		require.NoError(t, err)

		candle := feed.CandlePairTimeFrame["BTCUSDT--1d"][0]
		require.Len(t, feed.CandlePairTimeFrame["BTCUSDT--1d"], 14)
		require.Equal(t, "2021-04-26 00:00:00", candle.Time.UTC().Format("2006-01-02 15:04:05"))
		require.Equal(t, 49066.76, candle.Open)
		require.Equal(t, 54001.39, candle.Close)
		require.Equal(t, 48753.44, candle.Low)
		require.Equal(t, 54356.62, candle.High)
		require.Equal(t, 86310.8, candle.Volume)
		require.Equal(t, 1.1, candle.Metadata["lsr"])
	})
}

func TestCSVFeed_CandlesByLimit(t *testing.T) {
	feed, err := NewCSVFeed("1d", PairFeed{
		Timeframe: "1d",
		Pair:      "BTCUSDT",
		File:      "../testdata/btc-1d.csv",
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

func TestCSVFeed_resample(t *testing.T) {
	t.Run("1h to 1d", func(t *testing.T) {
		feed, err := NewCSVFeed(
			"1d",
			PairFeed{
				Timeframe: "1h",
				Pair:      "BTCUSDT",
				File:      "../testdata/btc-1h-2021-05-13.csv",
			})
		require.NoError(t, err)
		require.Len(t, feed.CandlePairTimeFrame["BTCUSDT--1d"], 24)
		require.Len(t, feed.CandlePairTimeFrame["BTCUSDT--1h"], 24)

		for _, candle := range feed.CandlePairTimeFrame["BTCUSDT--1d"][:23] {
			require.False(t, candle.Complete)
		}

		last := feed.CandlePairTimeFrame["BTCUSDT--1d"][23]
		require.Equal(t, int64(1620864000), last.Time.UTC().Unix()) // 13 May 2021 00:00:00

		assert.Equal(t, 49537.15, last.Open)
		assert.Equal(t, 49670.97, last.Close)
		assert.Equal(t, 46000.00, last.Low)
		assert.Equal(t, 51367.19, last.High)
		assert.Equal(t, 147332.0, last.Volume)
		assert.True(t, last.Complete)

		// load feed with 180 days witch candles of 1h
		feed, err = NewCSVFeed(
			"1d",
			PairFeed{
				Timeframe: "1h",
				Pair:      "BTCUSDT",
				File:      "../testdata/btc-1h.csv",
			})
		require.NoError(t, err)

		totalComplete := 0
		for _, candle := range feed.CandlePairTimeFrame["BTCUSDT--1d"] {
			if candle.Time.Hour() == 23 {
				require.True(t, true)
			}
			if candle.Complete {
				totalComplete++
			}
		}
		require.Equal(t, 180, totalComplete)
	})

	t.Run("invalid timeframe", func(t *testing.T) {
		feed, err := NewCSVFeed(
			"1d",
			PairFeed{
				Timeframe: "invalid",
				Pair:      "BTCUSDT",
				File:      "../testdata/btc-1h-2021-05-13.csv",
			})
		require.Error(t, err)
		require.Nil(t, feed)
	})
}

func TestIsLastCandlePeriod(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		tt := []struct {
			sourceTimeFrame string
			targetTimeFrame string
			time            time.Time
			last            bool
		}{
			{"1s", "1m", time.Date(2021, 1, 1, 23, 59, 59, 0, time.UTC), true},
			{"1h", "1h", time.Date(2021, 1, 1, 23, 59, 0, 0, time.UTC), true},
			{"1m", "1d", time.Date(2021, 1, 1, 23, 59, 0, 0, time.UTC), true},
			{"1m", "1d", time.Date(2021, 1, 1, 23, 58, 0, 0, time.UTC), false},
			{"1h", "1d", time.Date(2021, 1, 1, 23, 00, 0, 0, time.UTC), true},
			{"1h", "1d", time.Date(2021, 1, 1, 22, 00, 0, 0, time.UTC), false},
			{"1m", "5m", time.Date(2021, 1, 1, 0, 4, 0, 0, time.UTC), true},
			{"1m", "5m", time.Date(2021, 1, 1, 0, 1, 0, 0, time.UTC), false},
			{"1m", "10m", time.Date(2021, 1, 1, 0, 9, 0, 0, time.UTC), true},
			{"1m", "15m", time.Date(2021, 1, 1, 0, 14, 0, 0, time.UTC), true},
			{"1m", "15m", time.Date(2021, 1, 1, 0, 13, 0, 0, time.UTC), false},
			{"1h", "1w", time.Date(2021, 1, 2, 23, 00, 0, 0, time.UTC), true},
			{"1m", "30m", time.Date(2021, 1, 2, 0, 29, 0, 0, time.UTC), true},
			{"1m", "1h", time.Date(2021, 1, 2, 0, 59, 0, 0, time.UTC), true},
			{"1m", "2h", time.Date(2021, 1, 2, 1, 59, 0, 0, time.UTC), true},
			{"1m", "4h", time.Date(2021, 1, 2, 3, 59, 0, 0, time.UTC), true},
			{"1m", "12h", time.Date(2021, 1, 2, 23, 59, 0, 0, time.UTC), true},
			{"1d", "1w", time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC), true},
		}

		for _, tc := range tt {
			t.Run(fmt.Sprintf("%s to %s", tc.sourceTimeFrame, tc.targetTimeFrame), func(t *testing.T) {
				last, err := isLastCandlePeriod(tc.time, tc.sourceTimeFrame, tc.targetTimeFrame)
				require.NoError(t, err)
				require.Equal(t, tc.last, last)
			})
		}
	})

	t.Run("invalid source", func(t *testing.T) {
		last, err := isLastCandlePeriod(time.Now(), "invalid", "1h")
		require.Error(t, err)
		require.False(t, last)
	})

	t.Run("not supported interval", func(t *testing.T) {
		last, err := isLastCandlePeriod(time.Now(), "1d", "1y")
		require.EqualError(t, err, "invalid timeframe: 1y")
		require.False(t, last)
	})
}

func TestIsFistCandlePeriod(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		tt := []struct {
			sourceTimeFrame string
			targetTimeFrame string
			time            time.Time
			last            bool
		}{
			{"1d", "1w", time.Date(2021, 11, 6, 0, 0, 0, 0, time.UTC), false}, // sunday
			{"1d", "1w", time.Date(2021, 11, 7, 0, 0, 0, 0, time.UTC), true},  // monday
			{"1d", "1w", time.Date(2021, 11, 8, 0, 0, 0, 0, time.UTC), false}, // monday
			{"1h", "1d", time.Date(2021, 11, 8, 0, 0, 0, 0, time.UTC), true},  // monday
			{"1h", "1d", time.Date(2021, 11, 8, 1, 0, 0, 0, time.UTC), false}, // monday
		}

		for _, tc := range tt {
			t.Run(fmt.Sprintf("%s to %s", tc.sourceTimeFrame, tc.targetTimeFrame), func(t *testing.T) {
				first, err := isFistCandlePeriod(tc.time, tc.sourceTimeFrame, tc.targetTimeFrame)
				require.NoError(t, err)
				require.Equal(t, tc.last, first)
			})
		}
	})

	t.Run("invalid source", func(t *testing.T) {
		last, err := isFistCandlePeriod(time.Now(), "invalid", "1h")
		require.Error(t, err)
		require.False(t, last)
	})

	t.Run("not supported interval", func(t *testing.T) {
		last, err := isFistCandlePeriod(time.Now(), "1d", "1y")
		require.EqualError(t, err, "invalid timeframe: 1y")
		require.False(t, last)
	})
}
