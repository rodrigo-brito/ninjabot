package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPriorityQueue(t *testing.T) {
	now := time.Now()
	pq := NewPriorityQueue(nil)
	pq.Push(Candle{Time: now, Close: 2})
	pq.Push(Candle{Time: now.Add(time.Minute), Close: 6, Symbol: "D"})
	pq.Push(Candle{Time: now.Add(time.Minute), Close: 5, Symbol: "C"})
	pq.Push(Candle{Time: now.Add(time.Minute), Close: 4, Symbol: "B"})
	pq.Push(Candle{Time: now.Add(time.Minute), Close: 3, Symbol: "A"})
	pq.Push(Candle{Time: now.Add(-time.Minute), Close: 1})

	require.Equal(t, 1.0, pq.Pop().(Candle).Close)
	require.Equal(t, 2.0, pq.Pop().(Candle).Close)
	require.Equal(t, 3.0, pq.Pop().(Candle).Close)
	require.Equal(t, 4.0, pq.Pop().(Candle).Close)
	require.Equal(t, 5.0, pq.Pop().(Candle).Close)
	require.Equal(t, 6.0, pq.Pop().(Candle).Close)
}
