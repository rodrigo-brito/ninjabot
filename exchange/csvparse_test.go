package exchange

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/model"
)

func TestParseCSV(t *testing.T) {
	t.Run("with header and metadata", func(t *testing.T) {
		candles, err := ParseCSV("../testdata/btc-1d-header.csv", "BTCUSDT", false)
		require.NoError(t, err)
		require.Len(t, candles, 14)

		first := candles[0]
		require.Equal(t, "BTCUSDT", first.Pair)
		require.Equal(t, time.Unix(1619395200, 0).UTC(), first.Time)
		require.Equal(t, 49066.76, first.Open)
		require.Equal(t, 54001.39, first.Close)
		require.Equal(t, 48753.44, first.Low)
		require.Equal(t, 54356.62, first.High)
		require.Equal(t, 86310.8, first.Volume)
		require.True(t, first.Complete)
		require.Equal(t, map[string]float64{"trades": 2174544, "lsr": 1.1}, first.Metadata)
	})

	t.Run("without header", func(t *testing.T) {
		candles, err := ParseCSV("../testdata/btc-1h-2021-05-13.csv", "BTCUSDT", false)
		require.NoError(t, err)
		require.Len(t, candles, 24)
		require.Nil(t, candles[0].Metadata)
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := ParseCSV("../testdata/missing.csv", "BTCUSDT", false)
		require.Error(t, err)
	})

	t.Run("concurrent", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := ParseCSV("../testdata/btc-1h-2021-05-13.csv", "BTCUSDT", true)
				require.NoError(t, err)
			}()
		}
		wg.Wait()
	})
}

func TestResample(t *testing.T) {
	source, err := ParseCSV("../testdata/btc-1h-2021-05-13.csv", "BTCUSDT", false)
	require.NoError(t, err)
	snapshot := append([]model.Candle(nil), source...)

	candles, err := Resample(source, "1h", "4h")
	require.NoError(t, err)
	require.Equal(t, snapshot, source, "the input is not modified")

	// 2021-05-13 00:00 UTC is a 4h boundary: 24 hourly candles -> 24 outputs,
	// six of them complete
	require.Len(t, candles, 24)
	complete := 0
	for i, candle := range candles {
		bucket := source[i].Time.Truncate(4 * time.Hour)
		require.Equal(t, bucket, candle.Time, "candle %d carries the period start", i)
		if candle.Complete {
			complete++
		}
	}
	require.Equal(t, 6, complete)

	// the first complete candle aggregates the first four hours
	first := candles[3]
	require.True(t, first.Complete)
	require.Equal(t, source[0].Open, first.Open)
	require.Equal(t, source[3].Close, first.Close)
	require.InDelta(t, source[0].Volume+source[1].Volume+source[2].Volume+source[3].Volume, first.Volume, 1e-9)

	t.Run("same timeframe", func(t *testing.T) {
		candles, err := Resample(source, "1h", "1h")
		require.NoError(t, err)
		require.Equal(t, source, candles)
	})

	t.Run("no complete candle", func(t *testing.T) {
		_, err := Resample(source[:2], "1h", "1d")
		require.True(t, errors.Is(err, ErrInsufficientData))
	})

	t.Run("invalid timeframe", func(t *testing.T) {
		_, err := Resample(source, "1h", "3h")
		require.Error(t, err)
	})
}
