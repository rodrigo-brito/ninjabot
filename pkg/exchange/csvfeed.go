package exchange

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/rodrigo-brito/ninjabot/pkg/model"
)

var ErrInsufficientData = errors.New("insufficient data")

type PairFeed struct {
	Pair string
	File string
}

type CSVFeed struct {
	Timeframes          []string
	Feeds               map[string]PairFeed
	CandlePairTimeFrame map[string][]model.Candle
}

func NewCSVFeed(timeframe string, feeds ...PairFeed) (*CSVFeed, error) {
	csvFeed := &CSVFeed{
		Feeds:               make(map[string]PairFeed),
		CandlePairTimeFrame: make(map[string][]model.Candle),
	}

	for _, feed := range feeds {
		csvFeed.Timeframes = append(csvFeed.Timeframes, timeframe)
		csvFeed.Feeds[feed.Pair] = feed

		csvFile, err := os.Open(feed.File)
		if err != nil {
			return nil, err
		}

		csvLines, err := csv.NewReader(csvFile).ReadAll()
		if err != nil {
			fmt.Println(err)
		}

		var candles []model.Candle
		for _, line := range csvLines {
			timestamp, err := strconv.Atoi(line[0])
			if err != nil {
				return nil, err
			}

			candle := model.Candle{
				Time:     time.Unix(int64(timestamp), 0),
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

		csvFeed.CandlePairTimeFrame[csvFeed.feedTimeframeKey(feed.Pair, timeframe)] = candles
	}

	return csvFeed, nil
}

func (c CSVFeed) feedTimeframeKey(pair, timeframe string) string {
	return fmt.Sprintf("%s--%s", pair, timeframe)
}

func (c *CSVFeed) Resample(timeframes ...string) error {
	c.Timeframes = timeframes
	for _, feed := range c.Feeds {
		for _, timeframe := range timeframes {
			key := c.feedTimeframeKey(feed.Pair, timeframe)
			c.CandlePairTimeFrame[key] = make([]model.Candle, 0)
			// TODO: resample candles
		}
	}

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
