package main

import (
	"fmt"
	"log"
)

type DataFeed struct {
	Data <-chan Candle
	Err  <-chan error
}

type Exchange interface {
	Broker
	Account() (Account, error)
	SubscribeCandles(pair, timeframe string) (<-chan Candle, <-chan error)
}

type Broker interface {
	Buy(tick string, size float64) Order
	Sell(tick string, size float64) Order
	Cancel(Order)
}

type DataFeedSubscription struct {
	exchange                Exchange
	DataFeeds               map[string]*DataFeed
	SubscriptionsByDataFeed map[string][]DataFeedConsumer
}

type DataFeedConsumer func(Candle)

func NewDataFeed(exchange Exchange) DataFeedSubscription {
	return DataFeedSubscription{
		exchange:                exchange,
		DataFeeds:               make(map[string]*DataFeed),
		SubscriptionsByDataFeed: make(map[string][]DataFeedConsumer),
	}
}

func (d *DataFeedSubscription) feedKey(pair, timeframe string) string {
	return fmt.Sprintf("%s-%s", pair, timeframe)
}

func (d *DataFeedSubscription) Register(pair, timeframe string, consumer DataFeedConsumer) {
	key := d.feedKey(pair, timeframe)
	if _, ok := d.DataFeeds[key]; !ok {
		ccandle, cerr := d.exchange.SubscribeCandles(pair, timeframe)
		fmt.Println("new feed -> ", key)
		d.DataFeeds[key] = &DataFeed{
			Data: ccandle,
			Err:  cerr,
		}
	}
	fmt.Println("new consumer -> ", key)
	d.SubscriptionsByDataFeed[key] = append(d.SubscriptionsByDataFeed[key], consumer)
}

func (d *DataFeedSubscription) Start() <-chan struct{} {
	done := make(chan struct{}, 1)

	for key, feed := range d.DataFeeds {
		go func(key string, feed *DataFeed) {
			for {
				select {
				case candle := <-feed.Data:
					fmt.Println("data -> ", candle)
					for _, consumer := range d.SubscriptionsByDataFeed[key] {
						consumer(candle)
					}
				case err := <-feed.Err:
					log.Println("dataFeedSubscription/start: ", err)
				}
			}
		}(key, feed)
	}

	return done
}
