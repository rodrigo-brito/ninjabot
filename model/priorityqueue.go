package model

import "sync"

type PriorityQueue struct {
	sync.Mutex
	length          int
	data            []Item
	notifyCallbacks []func(Item)
}

type Item interface {
	Less(Item) bool
}

func NewPriorityQueue(data []Item) *PriorityQueue {
	q := &PriorityQueue{}
	q.data = data
	q.length = len(data)
	if q.length > 0 {
		i := q.length >> 1
		for ; i >= 0; i-- {
			q.down(i)
		}
	}
	return q
}

func (q *PriorityQueue) Push(item Item) {
	q.Lock()
	defer q.Unlock()

	q.data = append(q.data, item)
	q.length++
	q.up(q.length - 1)

	for _, notify := range q.notifyCallbacks {
		go notify(item)
	}
}

func (q *PriorityQueue) PopLock() <-chan Item {
	ch := make(chan Item)
	q.notifyCallbacks = append(q.notifyCallbacks, func(_ Item) {
		ch <- q.Pop()
	})
	return ch
}

func (q *PriorityQueue) Pop() Item {
	q.Lock()
	defer q.Unlock()

	if q.length == 0 {
		return nil
	}
	top := q.data[0]
	q.length--
	if q.length > 0 {
		q.data[0] = q.data[q.length]
		q.down(0)
	}
	q.data = q.data[:len(q.data)-1]
	return top
}

func (q *PriorityQueue) Peek() Item {
	q.Lock()
	defer q.Unlock()

	if q.length == 0 {
		return nil
	}
	return q.data[0]
}

func (q *PriorityQueue) Len() int {
	q.Lock()
	defer q.Unlock()

	return q.length
}
func (q *PriorityQueue) down(pos int) {
	data := q.data
	halfLength := q.length >> 1
	item := data[pos]
	for pos < halfLength {
		left := (pos << 1) + 1
		right := left + 1
		best := data[left]
		if right < q.length && data[right].Less(best) {
			left = right
			best = data[right]
		}
		if !best.Less(item) {
			break
		}
		data[pos] = best
		pos = left
	}
	data[pos] = item
}

func (q *PriorityQueue) up(pos int) {
	data := q.data
	item := data[pos]
	for pos > 0 {
		parent := (pos - 1) >> 1
		current := data[parent]
		if !item.Less(current) {
			break
		}
		data[pos] = current
		pos = parent
	}
	data[pos] = item
}
