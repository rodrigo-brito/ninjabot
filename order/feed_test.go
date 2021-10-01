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
	feed, symbol, called := NewOrderFeed(), "blaus", false

	feed.Subscribe(symbol, func(order model.Order) {
		called = true
	}, false)

	feed.Start()
	feed.Publish(model.Order{Symbol: symbol}, false)

	require.True(t, called)
}
