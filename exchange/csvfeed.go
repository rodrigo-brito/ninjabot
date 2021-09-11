package exchange

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/rodrigo-brito/ninjabot/model"

	"github.com/xhit/go-str2duration/v2"
)

var ErrInsufficientData = errors.New("insufficient data")

type PairFeed struct {
	Pair      string
	File      string
	Timeframe string
}

type CSVFeed struct {
	Feeds               map[string]PairFeed
	CandlePairTimeFrame map[string][]model.Candle
}

func NewCSVFeed(targetTimeframe string, feeds ...PairFeed) (*CSVFeed, error) {
	csvFeed := &CSVFeed{
		Feeds:               make(map[string]PairFeed),
		CandlePairTimeFrame: make(map[string][]model.Candle),
	}

	for _, feed := range feeds {
		csvFeed.Feeds[feed.Pair] = feed

		csvFile, err := os.Open(feed.File)
		if err != nil {
			return nil, err
		}

		csvLines, err := csv.NewReader(csvFile).ReadAll()
		if err != nil {
			return nil, err
		}

		var candles []model.Candle
		for _, line := range csvLines {
			timestamp, err := strconv.Atoi(line[0])
			if err != nil {
				return nil, err
			}

			candle := model.Candle{
				Time:     time.Unix(int64(timestamp), 0).UTC(),
				Symbol:   feed.Pair,
				Complete: true,
			}

			candle.Open, err = strconv.ParseFloat(line[1], 64)
			if err != nil {
				return nil, err
			}

			candle.Close, err = strconv.ParseFloat(line[2], 64)
			if err != nil {
				return nil, err
			}

			candle.Low, err = strconv.ParseFloat(line[3], 64)
			if err != nil {
				return nil, err
			}

			candle.High, err = strconv.ParseFloat(line[4], 64)
			if err != nil {
				return nil, err
			}

			candle.Volume, err = strconv.ParseFloat(line[5], 64)
			if err != nil {
				return nil, err
			}

			candles = append(candles, candle)
		}

		csvFeed.CandlePairTimeFrame[csvFeed.feedTimeframeKey(feed.Pair, feed.Timeframe)] = candles

		err = csvFeed.resample(feed.Pair, feed.Timeframe, targetTimeframe)
		if err != nil {
			return nil, err
		}
	}

	return csvFeed, nil
}

func (c CSVFeed) feedTimeframeKey(pair, timeframe string) string {
	return fmt.Sprintf("%s--%s", pair, timeframe)
}

func isLastCandlePeriod(t time.Time, fromTimeframe, targetTimeframe string) (bool, error) {
	if fromTimeframe == targetTimeframe {
		return true, nil
	}

	fromDuration, err := str2duration.ParseDuration(fromTimeframe)
	if err != nil {
		return false, err
	}

	next := t.Add(fromDuration).UTC()

	switch targetTimeframe {
	case "1m":
		return next.Second()%60 == 0, nil
	case "5m":
		return next.Minute()%5 == 0, nil
	case "10m":
		return next.Minute()%10 == 0, nil
	case "15m":
		return next.Minute()%15 == 0, nil
	case "30m":
		return next.Minute()%30 == 0, nil
	case "1h":
		return next.Minute()%60 == 0, nil
	case "2h":
		return next.Minute() == 0 && next.Hour()%2 == 0, nil
	case "4h":
		return next.Minute() == 0 && next.Hour()%4 == 0, nil
	case "12h":
		return next.Minute() == 0 && next.Hour()%12 == 0, nil
	case "1d":
		return next.Minute() == 0 && next.Hour()%24 == 0, nil
	case "1w":
		return next.Minute() == 0 && next.Hour()%24 == 0 && next.Weekday() == time.Sunday, nil
	}

	return false, fmt.Errorf("invalid timeframe: 1y")
}

func (c *CSVFeed) resample(pair, sourceTimeframe, targetTimeframe string) error {
	sourceKey := c.feedTimeframeKey(pair, sourceTimeframe)
	targetKey := c.feedTimeframeKey(pair, targetTimeframe)

	candles := make([]model.Candle, 0)
	for i, candle := range c.CandlePairTimeFrame[sourceKey] {
		if last, err := isLastCandlePeriod(candle.Time, sourceTimeframe, targetTimeframe); err != nil {
			return err
		} else if last {
			candle.Complete = true
		} else {
			candle.Complete = false
		}

		if i > 0 && !candles[i-1].Complete {
			candle.Time = candles[i-1].Time
			candle.Open = candles[i-1].Open
			candle.High = math.Max(candles[i-1].High, candle.High)
			candle.Low = math.Min(candles[i-1].Low, candle.Low)
			candle.Volume += candles[i-1].Volume
			candle.Trades += candles[i-1].Trades
		}
		candles = append(candles, candle)
	}

	c.CandlePairTimeFrame[targetKey] = candles

	return nil
}

func (c CSVFeed) CandlesByPeriod(_ context.Context, pair, timeframe string,
	start, end time.Time) ([]model.Candle, error) {

	key := c.feedTimeframeKey(pair, timeframe)
	candles := make([]model.Candle, 0)
	for _, candle := range c.CandlePairTimeFrame[key] {
		if candle.Time.Before(start) || candle.Time.After(end) {
			continue
		}
		candles = append(candles, candle)
	}
	return candles, nil
}

func (c *CSVFeed) CandlesByLimit(_ context.Context, pair, timeframe string, limit int) ([]model.Candle, error) {
	var result []model.Candle
	key := c.feedTimeframeKey(pair, timeframe)
	if len(c.CandlePairTimeFrame[key]) < limit {
		return nil, fmt.Errorf("%w: %s", ErrInsufficientData, pair)
	}
	result, c.CandlePairTimeFrame[key] = c.CandlePairTimeFrame[key][:limit], c.CandlePairTimeFrame[key][limit:]
	return result, nil
}

func (c CSVFeed) CandlesSubscription(pair, timeframe string) (chan model.Candle, chan error) {
	ccandle := make(chan model.Candle)
	cerr := make(chan error)
	key := c.feedTimeframeKey(pair, timeframe)
	go func() {
		for _, candle := range c.CandlePairTimeFrame[key] {
			ccandle <- candle
		}
		close(ccandle)
	}()
	return ccandle, cerr
}
