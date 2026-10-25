package download

import (
	"context"
	"encoding/csv"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/service"
	"github.com/rodrigo-brito/ninjabot/testdata/mocks"
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

func TestDownloader_download(t *testing.T) {
	ctx := context.Background()
	tmpFile, err := os.CreateTemp(os.TempDir(), "*.csv")
	require.NoError(t, err)

	time, err := time.Parse("2006-01-02", "2021-04-26")
	require.NoError(t, err)

	param := Parameters{
		Start: time,
		End:   time.AddDate(0, 0, 20),
	}

	csvFeed, err := exchange.NewCSVFeed(
		"1d",
		exchange.PairFeed{
			Pair:      "BTCUSDT",
			File:      "../testdata/btc-1d.csv",
			Timeframe: "1d",
		})
	require.NoError(t, err)

	fakeExchange := struct {
		service.Feeder
	}{
		Feeder: csvFeed,
	}

	downloader := Downloader{fakeExchange}

	t.Run("success", func(t *testing.T) {
		err = downloader.Download(ctx, "BTCUSDT", "1d", tmpFile.Name(), WithInterval(param.Start, param.End))
		require.NoError(t, err)

		csvFeed, err := exchange.NewCSVFeed(
			"1d",
			exchange.PairFeed{
				Pair:      "BTCUSDT",
				File:      "../testdata/btc-1d.csv",
				Timeframe: "1d",
			})
		require.NoError(t, err)
		require.Len(t, csvFeed.CandlePairTimeFrame["BTCUSDT--1d"], 14)
	})
}

func TestNewDownloader(t *testing.T) {
	feeder := &mocks.Feeder{}

	downloader := NewDownloader(feeder)

	require.Equal(t, feeder, downloader.exchange)
}

func TestWithDays(t *testing.T) {
	parameters := &Parameters{}

	WithDays(10)(parameters)

	require.WithinDuration(t, time.Now(), parameters.End, time.Minute)
	require.WithinDuration(t, time.Now().AddDate(0, 0, -10), parameters.Start, time.Minute)
}

func TestDownloadErrors(t *testing.T) {
	ctx := context.Background()
	start := time.Date(2021, 4, 26, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 5)

	t.Run("fails when the output file cannot be created", func(t *testing.T) {
		downloader := NewDownloader(&mocks.Feeder{})

		err := downloader.Download(ctx, "BTCUSDT", "1d",
			filepath.Join(t.TempDir(), "missing", "out.csv"), WithInterval(start, end))

		require.Error(t, err)
	})

	t.Run("fails on an invalid timeframe", func(t *testing.T) {
		downloader := NewDownloader(&mocks.Feeder{})

		err := downloader.Download(ctx, "BTCUSDT", "banana",
			filepath.Join(t.TempDir(), "out.csv"), WithInterval(start, end))

		require.Error(t, err)
	})

	t.Run("fails when the exchange rejects the request", func(t *testing.T) {
		feeder := &mocks.Feeder{}
		feeder.On("AssetsInfo", "BTCUSDT").Return(model.AssetInfo{QuotePrecision: 2})
		feeder.On("CandlesByPeriod", mock.Anything, "BTCUSDT", "1d", mock.Anything, mock.Anything).
			Return([]model.Candle{}, errors.New("exchange down"))

		downloader := NewDownloader(feeder)

		err := downloader.Download(ctx, "BTCUSDT", "1d",
			filepath.Join(t.TempDir(), "out.csv"), WithInterval(start, end))

		require.ErrorContains(t, err, "exchange down")
	})
}

func TestDownloadDefaultsToLastMonth(t *testing.T) {
	candle := func(at time.Time) model.Candle {
		return model.Candle{Pair: "BTCUSDT", Time: at, Open: 1, Close: 2, Low: 0.5, High: 3, Volume: 10}
	}

	feeder := &mocks.Feeder{}
	feeder.On("AssetsInfo", "BTCUSDT").Return(model.AssetInfo{QuotePrecision: 2})
	feeder.On("CandlesByPeriod", mock.Anything, "BTCUSDT", "1d", mock.Anything, mock.Anything).
		Return([]model.Candle{candle(time.Now().AddDate(0, 0, -1))}, nil)

	output := filepath.Join(t.TempDir(), "out.csv")
	require.NoError(t, NewDownloader(feeder).Download(context.Background(), "BTCUSDT", "1d", output))

	file, err := os.Open(output)
	require.NoError(t, err)
	defer file.Close()

	lines, err := csv.NewReader(file).ReadAll()
	require.NoError(t, err)
	require.Equal(t, []string{"time", "open", "close", "low", "high", "volume"}, lines[0])
	require.Len(t, lines, 2, "one header plus the single candle returned by the exchange")
}

// An end date in the future is clamped to now, so the download never asks for
// candles that do not exist yet.
func TestDownloadClampsFutureEnd(t *testing.T) {
	feeder := &mocks.Feeder{}
	feeder.On("AssetsInfo", "BTCUSDT").Return(model.AssetInfo{QuotePrecision: 2})
	feeder.On("CandlesByPeriod", mock.Anything, "BTCUSDT", "1d", mock.Anything, mock.Anything).
		Return([]model.Candle{}, nil)

	output := filepath.Join(t.TempDir(), "out.csv")
	err := NewDownloader(feeder).Download(context.Background(), "BTCUSDT", "1d", output,
		WithInterval(time.Now().AddDate(0, 0, -2), time.Now().AddDate(0, 0, 10)))

	require.NoError(t, err)

	for _, call := range feeder.Calls {
		if call.Method != "CandlesByPeriod" {
			continue
		}
		require.LessOrEqual(t, call.Arguments.Get(4).(time.Time).Unix(), time.Now().Unix())
	}
}
