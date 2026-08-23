package exchange

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/testdata/mocks"
)

// candleRecorder collects the candles delivered to a subscription.
type candleRecorder struct {
	mu      sync.Mutex
	candles []model.Candle
	done    chan struct{}
	want    int
}

func newCandleRecorder(want int) *candleRecorder {
	return &candleRecorder{done: make(chan struct{}), want: want}
}

func (r *candleRecorder) consume(candle model.Candle) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.candles = append(r.candles, candle)
	if len(r.candles) == r.want {
		close(r.done)
	}
}

func (r *candleRecorder) wait(t *testing.T) []model.Candle {
	t.Helper()

	select {
	case <-r.done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for candles")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]model.Candle(nil), r.candles...)
}

func testCandle(index int, complete bool) model.Candle {
	return model.Candle{
		Pair:     "BTCUSDT",
		Time:     time.Date(2023, 1, 1, index, 0, 0, 0, time.UTC),
		Close:    100 + float64(index),
		Complete: complete,
	}
}

func TestOrderError(t *testing.T) {
	cause := errors.New("boom")
	err := &OrderError{Err: cause, Pair: "BTCUSDT", Quantity: 1.5}

	require.Equal(t, "order error: boom", err.Error())
	require.ErrorIs(t, err, cause, "the cause must stay reachable through errors.Is")
}

func TestDataFeedSubscriptionKeys(t *testing.T) {
	feed := NewDataFeed(&mocks.Exchange{})

	key := feed.feedKey("BTCUSDT", "1h")
	require.Equal(t, "BTCUSDT--1h", key)

	pair, timeframe := feed.pairTimeframeFromKey(key)
	require.Equal(t, "BTCUSDT", pair)
	require.Equal(t, "1h", timeframe)
}

func TestDataFeedSubscribe(t *testing.T) {
	feed := NewDataFeed(&mocks.Exchange{})

	feed.Subscribe("BTCUSDT", "1h", func(model.Candle) {}, true)
	feed.Subscribe("BTCUSDT", "1h", func(model.Candle) {}, false)
	feed.Subscribe("ETHUSDT", "1h", func(model.Candle) {}, true)

	require.Len(t, feed.SubscriptionsByDataFeed["BTCUSDT--1h"], 2)
	require.Len(t, feed.SubscriptionsByDataFeed["ETHUSDT--1h"], 1)
	require.Equal(t, 2, feed.Feeds.Length(), "the same pair/timeframe shares a single feed")
}

func TestDataFeedPreload(t *testing.T) {
	feed := NewDataFeed(&mocks.Exchange{})

	var received []model.Candle
	feed.Subscribe("BTCUSDT", "1h", func(candle model.Candle) {
		received = append(received, candle)
	}, true)

	feed.Preload("BTCUSDT", "1h", []model.Candle{
		testCandle(1, true),
		testCandle(2, false), // partial candles are never preloaded
		testCandle(3, true),
	})

	require.Len(t, received, 2)
	require.Equal(t, 101.0, received[0].Close)
	require.Equal(t, 103.0, received[1].Close)
}

func TestDataFeedConnect(t *testing.T) {
	exchange := &mocks.Exchange{}
	candles := make(chan model.Candle)
	errs := make(chan error)
	exchange.On("CandlesSubscription", mock.Anything, "BTCUSDT", "1h").Return(candles, errs)

	feed := NewDataFeed(exchange)
	feed.Subscribe("BTCUSDT", "1h", func(model.Candle) {}, true)
	feed.Connect()

	require.Len(t, feed.DataFeeds, 1)
	require.NotNil(t, feed.DataFeeds["BTCUSDT--1h"])
	exchange.AssertExpectations(t)
}

func TestDataFeedStart(t *testing.T) {
	t.Run("delivers candles to every subscriber", func(t *testing.T) {
		exchange := &mocks.Exchange{}
		candles := make(chan model.Candle)
		errs := make(chan error)
		exchange.On("CandlesSubscription", mock.Anything, "BTCUSDT", "1h").Return(candles, errs)

		everyCandle := newCandleRecorder(2)
		closedOnly := newCandleRecorder(1)

		feed := NewDataFeed(exchange)
		feed.Subscribe("BTCUSDT", "1h", everyCandle.consume, false)
		feed.Subscribe("BTCUSDT", "1h", closedOnly.consume, true)
		feed.Start(false)

		candles <- testCandle(1, false)
		candles <- testCandle(2, true)

		require.Len(t, everyCandle.wait(t), 2)
		closed := closedOnly.wait(t)
		require.Len(t, closed, 1)
		require.True(t, closed[0].Complete, "onCandleClose subscribers skip partial candles")
	})

	t.Run("keeps running after a feed error", func(t *testing.T) {
		exchange := &mocks.Exchange{}
		candles := make(chan model.Candle)
		errs := make(chan error)
		exchange.On("CandlesSubscription", mock.Anything, "BTCUSDT", "1h").Return(candles, errs)

		recorder := newCandleRecorder(1)
		feed := NewDataFeed(exchange)
		feed.Subscribe("BTCUSDT", "1h", recorder.consume, false)
		feed.Start(false)

		errs <- errors.New("connection reset")
		candles <- testCandle(1, true)

		require.Len(t, recorder.wait(t), 1)
	})

	t.Run("returns when the feed is closed in sync mode", func(t *testing.T) {
		exchange := &mocks.Exchange{}
		candles := make(chan model.Candle)
		errs := make(chan error)
		exchange.On("CandlesSubscription", mock.Anything, "BTCUSDT", "1h").Return(candles, errs)

		feed := NewDataFeed(exchange)
		feed.Subscribe("BTCUSDT", "1h", func(model.Candle) {}, false)

		finished := make(chan struct{})
		go func() {
			feed.Start(true)
			close(finished)
		}()

		candles <- testCandle(1, true)
		close(candles)

		select {
		case <-finished:
		case <-time.After(time.Second):
			t.Fatal("Start(true) did not wait for the feed to close")
		}
	})
}
