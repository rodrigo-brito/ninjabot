package exchange

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/rodrigo-brito/ninjabot/pkg/service"

	"github.com/rodrigo-brito/ninjabot/pkg/model"

	log "github.com/sirupsen/logrus"
)

var (
	ErrInvalidQuantity   = errors.New("invalid quantity")
	ErrInsufficientFunds = errors.New("insufficient funds or locked")
	ErrInvalidAsset      = errors.New("invalid asset")
)

type DataFeed struct {
	Data chan model.Candle
	Err  chan error
}

type DataFeedSubscription struct {
	exchange                service.Exchange
	Feeds                   []string
	DataFeeds               map[string]*DataFeed
	SubscriptionsByDataFeed map[string][]Subscription
}

type Subscription struct {
	onCandleClose bool
	consumer      DataFeedConsumer
}

type DataFeedConsumer func(model.Candle)

func NewDataFeed(exchange service.Exchange) *DataFeedSubscription {
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
	log.Infof("Connecting to the exchange.")
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

	log.Infof("Data feed connected.")
	wg.Wait()
}
