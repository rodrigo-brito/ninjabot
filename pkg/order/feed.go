package order

import (
	"github.com/rodrigo-brito/ninjabot/pkg/model"
)

type Feed struct {
	Data chan model.Order
	Err  chan error
}

type FeedConsumer func(order model.Order)

type FeedSubscription struct {
	OrderFeeds            map[string]*Feed
	SubscriptionsBySymbol map[string][]Subscription
}

type Subscription struct {
	onlyNewOrder bool
	consumer     FeedConsumer
}

func NewOrderFeed() FeedSubscription {
	return FeedSubscription{
		OrderFeeds:            make(map[string]*Feed),
		SubscriptionsBySymbol: make(map[string][]Subscription),
	}
}

func (d *FeedSubscription) Subscribe(symbol string, consumer FeedConsumer, onlyNewOrder bool) {
	if _, ok := d.OrderFeeds[symbol]; !ok {
		d.OrderFeeds[symbol] = &Feed{
			Data: make(chan model.Order),
			Err:  make(chan error),
		}
	}

	d.SubscriptionsBySymbol[symbol] = append(d.SubscriptionsBySymbol[symbol], Subscription{
		onlyNewOrder: onlyNewOrder,
		consumer:     consumer,
	})
}

func (d *FeedSubscription) Publish(order model.Order, newOrder bool) {
	if _, ok := d.OrderFeeds[order.Symbol]; ok {
		d.OrderFeeds[order.Symbol].Data <- order
	}
}

func (d *FeedSubscription) Start() {
	for symbol, feed := range d.OrderFeeds {
		go func(symbol string, feed *Feed) {
			for order := range feed.Data {
				for _, subscription := range d.SubscriptionsBySymbol[symbol] {
					subscription.consumer(order)
				}
			}
		}(symbol, feed)
	}
}
