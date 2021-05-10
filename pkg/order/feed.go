package order

import (
	"github.com/rodrigo-brito/ninjabot/pkg/model"
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

func (d *Feed) Subscribe(symbol string, consumer FeedConsumer, onlyNewOrder bool) {
	if _, ok := d.OrderFeeds[symbol]; !ok {
		d.OrderFeeds[symbol] = &DataFeed{
			Data: make(chan model.Order),
			Err:  make(chan error),
		}
	}

	d.SubscriptionsBySymbol[symbol] = append(d.SubscriptionsBySymbol[symbol], Subscription{
		onlyNewOrder: onlyNewOrder,
		consumer:     consumer,
	})
}

func (d *Feed) Publish(order model.Order, newOrder bool) {
	if _, ok := d.OrderFeeds[order.Symbol]; ok {
		d.OrderFeeds[order.Symbol].Data <- order
	}
}

func (d *Feed) Start() {
	for symbol := range d.OrderFeeds {
		go func(symbol string, feed *DataFeed) {
			for order := range feed.Data {
				for _, subscription := range d.SubscriptionsBySymbol[symbol] {
					subscription.consumer(order)
				}
			}
		}(symbol, d.OrderFeeds[symbol])
	}
}
