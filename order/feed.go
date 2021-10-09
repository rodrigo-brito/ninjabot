package order

import (
	"github.com/rodrigo-brito/ninjabot/model"
)

type DataFeed struct {
	Data chan model.Order
	Err  chan error
}

type FeedConsumer func(order model.Order)

type Feed struct {
	OrderFeeds            map[string]*DataFeed
	SubscriptionsBySymbol map[string][]Subscription
}

type Subscription struct {
	onlyNewOrder bool
	consumer     FeedConsumer
}

func NewOrderFeed() *Feed {
	return &Feed{
		OrderFeeds:            make(map[string]*DataFeed),
		SubscriptionsBySymbol: make(map[string][]Subscription),
	}
}

func (d *Feed) Subscribe(pair string, consumer FeedConsumer, onlyNewOrder bool) {
	if _, ok := d.OrderFeeds[pair]; !ok {
		d.OrderFeeds[pair] = &DataFeed{
			Data: make(chan model.Order),
			Err:  make(chan error),
		}
	}

	d.SubscriptionsBySymbol[pair] = append(d.SubscriptionsBySymbol[pair], Subscription{
		onlyNewOrder: onlyNewOrder,
		consumer:     consumer,
	})
}

func (d *Feed) Publish(order model.Order, newOrder bool) {
	if _, ok := d.OrderFeeds[order.Pair]; ok {
		d.OrderFeeds[order.Pair].Data <- order
	}
}

func (d *Feed) Start() {
	for pair := range d.OrderFeeds {
		go func(pair string, feed *DataFeed) {
			for order := range feed.Data {
				for _, subscription := range d.SubscriptionsBySymbol[pair] {
					subscription.consumer(order)
				}
			}
		}(pair, d.OrderFeeds[pair])
	}
}
