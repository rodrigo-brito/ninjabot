package data

import (
	"context"
	"encoding/csv"
	"os"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/rodrigo-brito/ninjabot/pkg/exchange"
	"github.com/xhit/go-str2duration/v2"
)

const batchSize = 1000

type Downloader struct {
	exchange exchange.Exchange
}

func NewDownloader(exchange exchange.Exchange) Downloader {
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

func (d Downloader) Download(ctx context.Context, symbol, timeframe string, output string, options ...Option) error {
	recordFile, err := os.Create(output)
	if err != nil {
		return err
	}

	parameters := &Parameters{
		Start: time.Now().AddDate(0, -1, 0),
		End:   time.Now(),
	}

	for _, option := range options {
		option(parameters)
	}

	candlesCount, interval, err := candlesCount(parameters.Start, parameters.End, timeframe)
	if err != nil {
		return err
	}

	log.Infof("Downloading %d candles of %s for %s", candlesCount, timeframe, symbol)

	writer := csv.NewWriter(recordFile)
	for begin := parameters.Start; begin.Before(parameters.End); begin = begin.Add(interval * batchSize) {
		end := begin.Add(interval * batchSize)
		if end.After(parameters.End) {
			end = parameters.End
		}

		candles, err := d.exchange.CandlesByPeriod(ctx, symbol, timeframe, begin, end)
		if err != nil {
			return err
		}

		for _, candle := range candles {
			err := writer.Write(candle.ToSlice())
			if err != nil {
				return err
			}
		}
	}
	writer.Flush()
	log.Info("Done!")
	return writer.Error()
}
