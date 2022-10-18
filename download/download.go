package download

import (
	"context"
	"encoding/csv"
	"os"
	"time"

	"github.com/rodrigo-brito/ninjabot/service"

	"github.com/schollz/progressbar/v3"
	log "github.com/sirupsen/logrus"
	"github.com/xhit/go-str2duration/v2"
)

const batchSize = 500

type Downloader struct {
	exchange service.Exchange
}

func NewDownloader(exchange service.Exchange) Downloader {
	return Downloader{
		exchange: exchange,
	}
}

type Parameters struct {
	Start time.Time
	End   time.Time
}

type Option func(*Parameters)

func WithInterval(start, end time.Time) Option {
	return func(parameters *Parameters) {
		parameters.Start = start
		parameters.End = end
	}
}

func WithDays(days int) Option {
	return func(parameters *Parameters) {
		parameters.Start = time.Now().AddDate(0, 0, -days)
		parameters.End = time.Now()
	}
}

func candlesCount(start, end time.Time, timeframe string) (int, time.Duration, error) {
	totalDuration := end.Sub(start)
	interval, err := str2duration.ParseDuration(timeframe)
	if err != nil {
		return 0, 0, err
	}
	return int(totalDuration / interval), interval, nil
}

func (d Downloader) Download(ctx context.Context, pair, timeframe string, output string, options ...Option) error {
	recordFile, err := os.Create(output)
	if err != nil {
		return err
	}

	now := time.Now()
	parameters := &Parameters{
		Start: now.AddDate(0, -1, 0),
		End:   now,
	}

	for _, option := range options {
		option(parameters)
	}

	parameters.Start = time.Date(parameters.Start.Year(), parameters.Start.Month(), parameters.Start.Day(),
		0, 0, 0, 0, time.UTC)

	if now.Sub(parameters.End) > 0 {
		parameters.End = time.Date(parameters.End.Year(), parameters.End.Month(), parameters.End.Day(),
			0, 0, 0, 0, time.UTC)
	} else {
		parameters.End = now
	}

	candlesCount, interval, err := candlesCount(parameters.Start, parameters.End, timeframe)
	if err != nil {
		return err
	}
	candlesCount++

	log.Infof("Downloading %d candles of %s for %s", candlesCount, timeframe, pair)
	info := d.exchange.AssetsInfo(pair)
	writer := csv.NewWriter(recordFile)

	progressBar := progressbar.Default(int64(candlesCount))
	lostData := 0
	isLastLoop := false

	// write headers
	err = writer.Write([]string{
		"time", "open", "close", "low", "high", "volume",
	})
	if err != nil {
		return err
	}

	for begin := parameters.Start; begin.Before(parameters.End); begin = begin.Add(interval * batchSize) {
		end := begin.Add(interval * batchSize)
		if end.Before(parameters.End) {
			end = end.Add(-1 * time.Second)
		} else {
			end = parameters.End
			isLastLoop = true
		}

		candles, err := d.exchange.CandlesByPeriod(ctx, pair, timeframe, begin, end)
		if err != nil {
			return err
		}

		for _, candle := range candles {
			err := writer.Write(candle.ToSlice(info.QuotePrecision))
			if err != nil {
				return err
			}
		}

		countCandles := len(candles)
		if !isLastLoop {
			lostData += batchSize - countCandles
		}

		if err = progressBar.Add(countCandles); err != nil {
			log.Warningf("update progresbar fail: %s", err.Error())
		}
	}

	if err = progressBar.Close(); err != nil {
		log.Warningf("close progresbar fail: %s", err.Error())
	}

	if lostData > 0 {
		log.Warningf("%d missing candles", lostData)
	}

	writer.Flush()
	log.Info("Done!")
	return writer.Error()
}
