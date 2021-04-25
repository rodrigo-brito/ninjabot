package ninjabot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type DataFeed struct {
	Data chan Candle
	Err  chan error
}

type Exchange interface {
	Broker
	Account() (Account, error)
	LoadCandlesByPeriod(ctx context.Context, pair, period string, start, end time.Time) ([]Candle, error)
	LoadCandlesByLimit(ctx context.Context, pair, period string, limit int) ([]Candle, error)
	SubscribeCandles(pair, timeframe string) (chan Candle, chan error)
}

type OrderKind string

var (
	SellOrder OrderKind = "sell"
	BuyOrder  OrderKind = "buy"
)

type Broker interface {
	OrderLimit(kind OrderKind, tick string, size float64, limit float64) Order
	OrderMarket(kind OrderKind, tick string, size float64) Order
	Stop(tick string, size float64) Order
	Cancel(Order)
}

type DataFeedSubscription struct {
	exchange                Exchange
	Feeds                   []string
	DataFeeds               map[string]*DataFeed
	SubscriptionsByDataFeed map[string][]Subscription
}

type Subscription struct {
	onCandleClose bool
	consumer      DataFeedConsumer
}

type DataFeedConsumer func(Candle)

func NewDataFeed(exchange Exchange) DataFeedSubscription {
	return DataFeedSubscription{
		exchange:                exchange,
		DataFeeds:               make(map[string]*DataFeed),
		SubscriptionsByDataFeed: make(map[string][]Subscription),
	}
}

func (d *DataFeedSubscription) feedKey(pair, timeframe string) string {
	return fmt.Sprintf("%s--%s", pair, timeframe)
}

func (d *DataFeedSubscription) PairTimeframeFromKey(key string) (pair, timeframe string) {
	parts := strings.Split(key, "--")
	return parts[0], parts[1]
}

func (d *DataFeedSubscription) Register(pair, timeframe string, consumer DataFeedConsumer, onCandleClose bool) {
	key := d.feedKey(pair, timeframe)
	d.Feeds = append(d.Feeds, key)
	d.SubscriptionsByDataFeed[key] = append(d.SubscriptionsByDataFeed[key], Subscription{
		onCandleClose: onCandleClose,
		consumer:      consumer,
	})
}

func (d *DataFeedSubscription) Preload(pair, timeframe string, candles []Candle) {
	fmt.Printf("[SETUP] preloading %d candles for %s--%s\n", len(candles), pair, timeframe)
	key := d.feedKey(pair, timeframe)
	for _, candle := range candles {
		for _, subscription := range d.SubscriptionsByDataFeed[key] {
			subscription.consumer(candle)
		}
	}
}

func (d *DataFeedSubscription) Connect() {
	for _, feed := range d.Feeds {
		fmt.Println("[SETUP] connecting to datafeed:", feed)
		pair, timeframe := d.PairTimeframeFromKey(feed)
		ccandle, cerr := d.exchange.SubscribeCandles(pair, timeframe)
		d.DataFeeds[feed] = &DataFeed{
			Data: ccandle,
			Err:  cerr,
		}
	}
}

func (d *DataFeedSubscription) Start(ctx context.Context) <-chan struct{} {
	d.Connect()
	done := make(chan struct{}, 1)
	for key, feed := range d.DataFeeds {
		go func(key string, feed *DataFeed) {
			for {
				select {
				case candle := <-feed.Data:
					for _, subscription := range d.SubscriptionsByDataFeed[key] {
						if subscription.onCandleClose && !candle.Complete {
							continue
						}
						subscription.consumer(candle)
					}
				case err := <-feed.Err:
					log.Println("dataFeedSubscription/start: ", err)
					if errors.Is(err, &websocket.CloseError{}) {
						close(done)
						return
					}
				}
			}
		}(key, feed)
	}

	fmt.Println("Bot started.")

	return done
}
