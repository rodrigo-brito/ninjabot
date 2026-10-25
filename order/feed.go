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

	// synchronous feeds deliver orders to the consumers on the caller's
	// goroutine, in the order they are published. Backtests use them to keep
	// orders and candles in step.
	synchronous bool
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

// NewSyncFeed creates a feed that calls the consumers directly from Publish,
// on the caller's goroutine. Start is a no-op for it. Consumers must not
// publish orders themselves.
func NewSyncFeed() *Feed {
	feed := NewOrderFeed()
	feed.synchronous = true
	return feed
}

// SetSynchronous switches the delivery mode of the feed. It must be called
// before Start and before any order is published.
func (d *Feed) SetSynchronous(synchronous bool) {
	d.synchronous = synchronous
}

// Synchronous reports whether the feed delivers orders from Publish itself.
func (d *Feed) Synchronous() bool {
	return d.synchronous
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

func (d *Feed) Publish(order model.Order, _ bool) {
	if d.synchronous {
		for _, subscription := range d.SubscriptionsBySymbol[order.Pair] {
			subscription.consumer(order)
		}
		return
	}

	if _, ok := d.OrderFeeds[order.Pair]; ok {
		d.OrderFeeds[order.Pair].Data <- order
	}
}

func (d *Feed) Start() {
	if d.synchronous {
		return
	}
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
