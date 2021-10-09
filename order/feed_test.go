package order

import (
	"testing"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/stretchr/testify/require"
)

func TestFeed_NewOrderFeed(t *testing.T) {
	feed := NewOrderFeed()
	require.NotEmpty(t, feed)
}

func TestFeed_Subscribe(t *testing.T) {
	feed, pair := NewOrderFeed(), "blaus"
	called := make(chan bool, 1)

	feed.Subscribe(pair, func(order model.Order) {
		called <- true
	}, false)

	feed.Start()
	feed.Publish(model.Order{Pair: pair}, false)
	require.True(t, <-called)
}
