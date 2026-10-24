// Package backtest runs strategies over historical candles with a
// deterministic, single-threaded engine. Data is loaded once into a Dataset
// that many runs can share concurrently.
package backtest

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/model"
)

// ErrUnknownPair is returned when a dataset has no candles for a pair.
var ErrUnknownPair = errors.New("backtest: unknown pair")

// source is the raw candles of a pair, in the timeframe they were loaded.
type source struct {
	timeframe string
	candles   []model.Candle
}

// store is the immutable data shared by a dataset and its windows, plus the
// cache of resampled series.
type store struct {
	sources map[string]source

	mu        sync.RWMutex
	resampled map[string][]model.Candle // pair--timeframe
}

// Dataset holds the candles of a set of pairs, loaded once. Series in other
// timeframes are resampled on first use and cached, so every run that shares
// the dataset pays for each resample only once. A Dataset is safe for
// concurrent use; the slices it returns are shared and must not be modified.
type Dataset struct {
	store      *store
	start, end time.Time // zero values mean unbounded
}

// LoadDataset parses the CSV files of the feeds, in parallel.
func LoadDataset(feeds ...exchange.PairFeed) (*Dataset, error) {
	candles := make([][]model.Candle, len(feeds))
	group := new(errgroup.Group)
	for i, feed := range feeds {
		group.Go(func() error {
			parsed, err := exchange.ParseCSV(feed.File, feed.Pair, feed.HeikinAshi)
			if err != nil {
				return fmt.Errorf("backtest: load %s: %w", feed.File, err)
			}
			candles[i] = parsed
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}

	ds := NewDataset()
	for i, feed := range feeds {
		if err := ds.add(feed.Pair, feed.Timeframe, candles[i]); err != nil {
			return nil, err
		}
	}
	return ds, nil
}

// NewDataset creates an empty dataset. Use Add to fill it before sharing it.
func NewDataset() *Dataset {
	return &Dataset{store: &store{
		sources:   make(map[string]source),
		resampled: make(map[string][]model.Candle),
	}}
}

// Add registers the candles of a pair, in the given timeframe. Candles must
// be complete and sorted by time. Repeated timestamps are accepted, as the
// downloader may produce them at batch boundaries; the strategy controller
// treats a repeated candle as an update of the last one. Add is not safe for concurrent use: call it
// before the dataset is shared.
func (d *Dataset) Add(pair, timeframe string, candles []model.Candle) error {
	return d.add(pair, timeframe, candles)
}

func (d *Dataset) add(pair, timeframe string, candles []model.Candle) error {
	if _, ok := d.store.sources[pair]; ok {
		return fmt.Errorf("backtest: pair %s loaded twice", pair)
	}
	if len(candles) == 0 {
		return fmt.Errorf("%w: no candles for %s", exchange.ErrInsufficientData, pair)
	}
	for i := 1; i < len(candles); i++ {
		if candles[i].Time.Before(candles[i-1].Time) {
			return fmt.Errorf("backtest: %s candles are not sorted at %s", pair, candles[i].Time)
		}
	}
	d.store.sources[pair] = source{timeframe: timeframe, candles: candles}
	return nil
}

// Pairs returns the pairs of the dataset, sorted.
func (d *Dataset) Pairs() []string {
	pairs := make([]string, 0, len(d.store.sources))
	for pair := range d.store.sources {
		pairs = append(pairs, pair)
	}
	sort.Strings(pairs)
	return pairs
}

// Timeframe returns the timeframe a pair was loaded in.
func (d *Dataset) Timeframe(pair string) (string, error) {
	src, ok := d.store.sources[pair]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrUnknownPair, pair)
	}
	return src.timeframe, nil
}

// Candles returns the candles of a pair in a timeframe, inside the window of
// the dataset. When the timeframe differs from the loaded one, the series is
// resampled (see exchange.Resample): partial candles precede each complete
// one. The returned slice is shared and read-only.
func (d *Dataset) Candles(pair, timeframe string) ([]model.Candle, error) {
	candles, err := d.store.series(pair, timeframe)
	if err != nil {
		return nil, err
	}
	return d.clip(candles), nil
}

// Window returns a view of the dataset restricted to candles whose time is
// within [start, end]. A zero start or end leaves that side unbounded. The
// view shares the data and the resample cache: nothing is copied.
func (d *Dataset) Window(start, end time.Time) *Dataset {
	return &Dataset{store: d.store, start: start, end: end}
}

// Bounds returns the window of the dataset; zero values mean unbounded.
func (d *Dataset) Bounds() (start, end time.Time) {
	return d.start, d.end
}

func (d *Dataset) clip(candles []model.Candle) []model.Candle {
	from, to := 0, len(candles)
	if !d.start.IsZero() {
		from = sort.Search(len(candles), func(i int) bool { return !candles[i].Time.Before(d.start) })
	}
	if !d.end.IsZero() {
		to = sort.Search(len(candles), func(i int) bool { return candles[i].Time.After(d.end) })
	}
	if from > to {
		return nil
	}
	return candles[from:to:to]
}

func (s *store) series(pair, timeframe string) ([]model.Candle, error) {
	src, ok := s.sources[pair]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownPair, pair)
	}
	if timeframe == "" || timeframe == src.timeframe {
		return src.candles, nil
	}

	key := pair + "--" + timeframe
	s.mu.RLock()
	candles, ok := s.resampled[key]
	s.mu.RUnlock()
	if ok {
		return candles, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if candles, ok := s.resampled[key]; ok {
		return candles, nil
	}
	candles, err := exchange.Resample(src.candles, src.timeframe, timeframe)
	if err != nil {
		return nil, fmt.Errorf("backtest: resample %s to %s: %w", pair, timeframe, err)
	}
	s.resampled[key] = candles
	return candles, nil
}

// feeder exposes a dataset as a service.Feeder for one run. CandlesByLimit
// consumes candles from a cursor of its own, so the dataset is never
// modified. It is safe for concurrent use.
type feeder struct {
	ds *Dataset

	mu     sync.Mutex
	cursor map[string]int
	last   map[string]float64
}

func newFeeder(ds *Dataset) *feeder {
	return &feeder{ds: ds, cursor: make(map[string]int), last: make(map[string]float64)}
}

func (f *feeder) observe(candle model.Candle) {
	f.mu.Lock()
	f.last[candle.Pair] = candle.Close
	f.mu.Unlock()
}

func (f *feeder) AssetsInfo(pair string) model.AssetInfo {
	asset, quote := exchange.SplitAssetQuote(pair)
	return model.AssetInfo{
		BaseAsset:          asset,
		QuoteAsset:         quote,
		MaxPrice:           math.MaxFloat64,
		MaxQuantity:        math.MaxFloat64,
		StepSize:           0.00000001,
		TickSize:           0.00000001,
		QuotePrecision:     8,
		BaseAssetPrecision: 8,
	}
}

// LastQuote returns the close of the last candle the run processed.
func (f *feeder) LastQuote(_ context.Context, pair string) (float64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	price, ok := f.last[pair]
	if !ok {
		return 0, fmt.Errorf("%w: no quote for %s yet", exchange.ErrInsufficientData, pair)
	}
	return price, nil
}

func (f *feeder) CandlesByPeriod(_ context.Context, pair, timeframe string,
	start, end time.Time) ([]model.Candle, error) {
	return f.ds.Window(start, end).Candles(pair, timeframe)
}

func (f *feeder) CandlesByLimit(_ context.Context, pair, timeframe string, limit int) ([]model.Candle, error) {
	candles, err := f.ds.Candles(pair, timeframe)
	if err != nil {
		return nil, err
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	key := pair + "--" + timeframe
	from := f.cursor[key]
	if len(candles)-from < limit {
		return nil, fmt.Errorf("%w: %s", exchange.ErrInsufficientData, pair)
	}
	f.cursor[key] = from + limit
	return candles[from : from+limit : from+limit], nil
}

func (f *feeder) CandlesSubscription(_ context.Context, pair, timeframe string) (chan model.Candle, chan error) {
	ccandle := make(chan model.Candle)
	cerr := make(chan error, 1)
	go func() {
		defer close(ccandle)
		defer close(cerr)
		candles, err := f.ds.Candles(pair, timeframe)
		if err != nil {
			cerr <- err
			return
		}
		for _, candle := range candles {
			ccandle <- candle
		}
	}()
	return ccandle, cerr
}
