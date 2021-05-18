package exchange

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rodrigo-brito/ninjabot/pkg/model"

	log "github.com/sirupsen/logrus"
)

var (
	ErrInvalidQuantity   = errors.New("invalid quantity")
	ErrInsufficientFunds = errors.New("insufficient funds or locked")
	ErrInvalidAsset      = errors.New("invalid asset")
)

type Exchange interface {
	Broker
	Feeder
}

type Feeder interface {
	CandlesByPeriod(ctx context.Context, pair, period string, start, end time.Time) ([]model.Candle, error)
	CandlesByLimit(ctx context.Context, pair, period string, limit int) ([]model.Candle, error)
	CandlesSubscription(pair, timeframe string) (chan model.Candle, chan error)
}

type Broker interface {
	Account() (model.Account, error)
	Position(symbol string) (asset, quote float64, err error)
	Order(symbol string, id int64) (model.Order, error)
	OrderOCO(side model.SideType, symbol string, size, price, stop, stopLimit float64) ([]model.Order, error)
	OrderLimit(side model.SideType, symbol string, size float64, limit float64) (model.Order, error)
	OrderMarket(side model.SideType, symbol string, size float64) (model.Order, error)
	Cancel(model.Order) error
}

type DataFeed struct {
	Data chan model.Candle
	Err  chan error
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

type DataFeedConsumer func(model.Candle)

func NewDataFeed(exchange Exchange) *DataFeedSubscription {
	return &DataFeedSubscription{
		exchange:                exchange,
		DataFeeds:               make(map[string]*DataFeed),
		SubscriptionsByDataFeed: make(map[string][]Subscription),
	}
}

func (d *DataFeedSubscription) feedKey(pair, timeframe string) string {
	return fmt.Sprintf("%s--%s", pair, timeframe)
}

func (d *DataFeedSubscription) pairTimeframeFromKey(key string) (pair, timeframe string) {
	parts := strings.Split(key, "--")
	return parts[0], parts[1]
}

func (d *DataFeedSubscription) Subscribe(pair, timeframe string, consumer DataFeedConsumer, onCandleClose bool) {
	key := d.feedKey(pair, timeframe)
	d.Feeds = append(d.Feeds, key)
	d.SubscriptionsByDataFeed[key] = append(d.SubscriptionsByDataFeed[key], Subscription{
		onCandleClose: onCandleClose,
		consumer:      consumer,
	})
}

func (d *DataFeedSubscription) Preload(pair, timeframe string, candles []model.Candle) {
	log.Infof("[SETUP] preloading %d candles for %s-%s", len(candles), pair, timeframe)
	key := d.feedKey(pair, timeframe)
	for _, candle := range candles {
		for _, subscription := range d.SubscriptionsByDataFeed[key] {
			subscription.consumer(candle)
		}
	}
}

func (d *DataFeedSubscription) Connect() {
	log.Infof("[SETUP] connecting to exchange")
	for _, feed := range d.Feeds {
		pair, timeframe := d.pairTimeframeFromKey(feed)
		ccandle, cerr := d.exchange.CandlesSubscription(pair, timeframe)
		d.DataFeeds[feed] = &DataFeed{
			Data: ccandle,
			Err:  cerr,
		}
	}
}

func (d *DataFeedSubscription) Start() {
	d.Connect()
	wg := new(sync.WaitGroup)
	for key, feed := range d.DataFeeds {
		wg.Add(1)
		go func(key string, feed *DataFeed) {
			for {
				select {
				case candle, ok := <-feed.Data:
					if !ok {
						wg.Done()
						return
					}
					for _, subscription := range d.SubscriptionsByDataFeed[key] {
						if subscription.onCandleClose && !candle.Complete {
							continue
						}
						subscription.consumer(candle)
					}
				case err := <-feed.Err:
					log.Error("dataFeedSubscription/start: ", err)
				}
			}
		}(key, feed)
	}

	log.Infof("Bot started.")
	wg.Wait()
}
