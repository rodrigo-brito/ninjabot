package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPriorityQueue(t *testing.T) {
	pq := NewPriorityQueue()
	pq.Push(Candle{Time: time.Now(), Close: 2})
	pq.Push(Candle{Time: time.Now().Add(time.Minute), Close: 3})
	pq.Push(Candle{Time: time.Now().Add(-time.Minute), Close: 1})

	require.Equal(t, 1.0, pq.Pop().(Candle).Close)
	require.Equal(t, 2.0, pq.Pop().(Candle).Close)
	require.Equal(t, 3.0, pq.Pop().(Candle).Close)
}
