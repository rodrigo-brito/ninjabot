package backtest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/model"
)

func sampleDataset(t testing.TB) *Dataset {
	t.Helper()
	ds, err := LoadDataset(
		exchange.PairFeed{Pair: "BTCUSDT", File: "../testdata/btc-1h.csv", Timeframe: "1h"},
		exchange.PairFeed{Pair: "ETHUSDT", File: "../testdata/eth-1h.csv", Timeframe: "1h"},
	)
	require.NoError(t, err)
	return ds
}

func TestLoadDataset(t *testing.T) {
	ds := sampleDataset(t)
	require.Equal(t, []string{"BTCUSDT", "ETHUSDT"}, ds.Pairs())

	tf, err := ds.Timeframe("BTCUSDT")
	require.NoError(t, err)
	require.Equal(t, "1h", tf)

	candles, err := ds.Candles("BTCUSDT", "1h")
	require.NoError(t, err)
	require.Len(t, candles, 4314)

	_, err = ds.Candles("XRPUSDT", "1h")
	require.True(t, errors.Is(err, ErrUnknownPair))

	_, err = LoadDataset(exchange.PairFeed{Pair: "BTCUSDT", File: "../testdata/missing.csv", Timeframe: "1h"})
	require.Error(t, err)

	_, err = LoadDataset(
		exchange.PairFeed{Pair: "BTCUSDT", File: "../testdata/btc-1h.csv", Timeframe: "1h"},
		exchange.PairFeed{Pair: "BTCUSDT", File: "../testdata/btc-1h.csv", Timeframe: "1h"},
	)
	require.Error(t, err, "a pair loaded twice is rejected")
}

func TestDataset_ResampleMatchesCSVFeed(t *testing.T) {
	ds := sampleDataset(t)
	feed, err := exchange.NewCSVFeed("4h", exchange.PairFeed{Pair: "BTCUSDT", File: "../testdata/btc-1h.csv", Timeframe: "1h"})
	require.NoError(t, err)

	candles, err := ds.Candles("BTCUSDT", "4h")
	require.NoError(t, err)
	require.Equal(t, feed.CandlePairTimeFrame["BTCUSDT--4h"], candles)

	again, err := ds.Candles("BTCUSDT", "4h")
	require.NoError(t, err)
	require.Same(t, &candles[0], &again[0], "the resampled series is cached")
}

func TestDataset_Window(t *testing.T) {
	ds := sampleDataset(t)
	all, err := ds.Candles("BTCUSDT", "1h")
	require.NoError(t, err)

	start, end := all[10].Time, all[19].Time
	window := ds.Window(start, end)
	candles, err := window.Candles("BTCUSDT", "1h")
	require.NoError(t, err)
	require.Len(t, candles, 10)
	require.Equal(t, start, candles[0].Time)
	require.Equal(t, end, candles[9].Time)
	require.Same(t, &all[10], &candles[0], "a window shares the data")

	open, err := ds.Window(all[len(all)-3].Time, time.Time{}).Candles("BTCUSDT", "1h")
	require.NoError(t, err)
	require.Len(t, open, 3)

	empty, err := ds.Window(end, start).Candles("BTCUSDT", "1h")
	require.NoError(t, err)
	require.Empty(t, empty)

	// appending to a window never writes into the shared data
	_ = append(candles, model.Candle{})
	require.Equal(t, all[20], func() model.Candle { c, _ := ds.Candles("BTCUSDT", "1h"); return c[20] }())
}

func TestFeeder(t *testing.T) {
	ds := sampleDataset(t)
	f := newFeeder(ds)
	ctx := context.Background()

	first, err := f.CandlesByLimit(ctx, "BTCUSDT", "1h", 5)
	require.NoError(t, err)
	second, err := f.CandlesByLimit(ctx, "BTCUSDT", "1h", 5)
	require.NoError(t, err)
	require.True(t, second[0].Time.After(first[4].Time), "the cursor advances")

	all, err := ds.Candles("BTCUSDT", "1h")
	require.NoError(t, err)
	require.Len(t, all, 4314, "the dataset is not consumed")

	_, err = f.CandlesByLimit(ctx, "BTCUSDT", "1h", 1e6)
	require.True(t, errors.Is(err, exchange.ErrInsufficientData))

	_, err = f.LastQuote(ctx, "BTCUSDT")
	require.Error(t, err)
	f.observe(all[0])
	quote, err := f.LastQuote(ctx, "BTCUSDT")
	require.NoError(t, err)
	require.Equal(t, all[0].Close, quote)

	period, err := f.CandlesByPeriod(ctx, "BTCUSDT", "1h", all[0].Time, all[2].Time)
	require.NoError(t, err)
	require.Len(t, period, 3)

	ch, cerr := f.CandlesSubscription(ctx, "ETHUSDT", "4h")
	count := 0
	for range ch {
		count++
	}
	require.NoError(t, <-cerr)
	require.Positive(t, count)
}

func TestDataset_Concurrent(t *testing.T) {
	ds := sampleDataset(t)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, tf := range []string{"1h", "4h", "1d"} {
				for _, pair := range ds.Pairs() {
					candles, err := ds.Window(time.Time{}, time.Now()).Candles(pair, tf)
					require.NoError(t, err)
					require.NotEmpty(t, candles)
				}
			}
		}()
	}
	wg.Wait()
}
