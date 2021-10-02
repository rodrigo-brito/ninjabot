package download

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloader_candlesCount(t *testing.T) {
	tt := []struct {
		start     time.Time
		end       time.Time
		timeframe string
		interval  time.Duration
		total     int
	}{
		{time.Now(), time.Now().AddDate(0, 0, 10), "1d", time.Hour * 24, 10},
		{time.Now(), time.Now().Add(60 * time.Minute), "1m", time.Minute, 60},
		{time.Now(), time.Now().Add(60 * time.Minute), "15m", 15 * time.Minute, 4},
	}

	t.Run("failed attempt", func(t *testing.T) {
		_, _, err := candlesCount(tt[0].start, tt[0].end, "batata")
		require.Error(t, err)
	})

	t.Run("Success candlesCount", func(t *testing.T) {
		for _, tc := range tt {
			total, interval, err := candlesCount(tc.start, tc.end, tc.timeframe)
			require.NoError(t, err)
			assert.Equal(t, tc.total, total)
			assert.Equal(t, tc.interval, interval)
		}
	})

}

func TestDownloader_withInterval(t *testing.T) {
	startingParams := []Parameters{
		{Start: time.Now(), End: time.Now().AddDate(0, 0, 10)},
		{Start: time.Now().AddDate(0, 0, 15), End: time.Now().AddDate(0, 0, 25)},
	}

	WithInterval(startingParams[0].Start, startingParams[0].End)(&startingParams[1])

	assert.Equal(t, startingParams[0], startingParams[1])
}

// func TestDownloader_withDays(t *testing.T) {
// 	startingParam := []Parameters{
// 		{},
// 		{Start: time.Now().AddDate(0, 0, -10), End: time.Now()},
// 	}

// }

func TestDownloader_download(t *testing.T) {
	tmpFile, err := os.CreateTemp(os.TempDir(), "*.csv")
	require.NoError(t, err)

	time, err := time.Parse("2006-01-02", "2021-04-26")
	require.NoError(t, err)

	param := Parameters{
		Start: time,
		End:   time.AddDate(0, 0, 20),
	}

	csvFeed, _ := exchange.NewCSVFeed(
		"1d",
		exchange.PairFeed{
			Pair:      "BTCUSDT",
			File:      "../testdata/btc-1d.csv",
			Timeframe: "1d",
		})

	fakeExchange := struct {
		service.Broker
		service.Feeder
	}{
		Feeder: csvFeed,
	}

	downloader := Downloader{fakeExchange}

	t.Run("Success case", func(t *testing.T) {
		err = downloader.Download(context.Background(), "BTCUSDT", "1d", tmpFile.Name(), WithInterval(param.Start, param.End))
		require.NoError(t, err)

		bytesRead, err := tmpFile.Read([]byte{1})
		require.NoError(t, err)

		assert.Equal(t, bytesRead, 1)
	})
	t.Run("Empty file", func(t *testing.T) {
		err = downloader.Download(context.Background(), "BTCUSDT", "1d", tmpFile.Name(), WithDays(4))
		require.NoError(t, err)

		_, err := tmpFile.Read([]byte{1})
		require.Error(t, err, io.EOF)
	})
}
