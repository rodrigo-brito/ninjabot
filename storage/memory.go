package storage

import (
	"fmt"
	"sort"
	"sync"

	"github.com/rodrigo-brito/ninjabot/model"
)

// Memory is an in-process order storage backed by a slice. It keeps orders as
// plain structs, with no serialization, which makes it the fastest backend
// for backtests. Nothing is persisted.
type Memory struct {
	mu     sync.RWMutex
	lastID int64
	orders []model.Order
	index  map[int64]int // order ID -> position in orders
}

// NewMemory creates an empty in-memory storage.
func NewMemory() *Memory {
	return &Memory{index: make(map[int64]int)}
}

// CreateOrder stores a copy of the order and assigns it a sequential ID.
func (m *Memory) CreateOrder(order *model.Order) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastID++
	order.ID = m.lastID
	m.index[order.ID] = len(m.orders)
	m.orders = append(m.orders, *order)
	return nil
}

// UpdateOrder replaces the stored copy of an order.
func (m *Memory) UpdateOrder(order *model.Order) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	i, ok := m.index[order.ID]
	if !ok {
		return fmt.Errorf("storage: order %d not found", order.ID)
	}
	m.orders[i] = *order
	return nil
}

// Orders returns copies of the orders accepted by every filter, ordered by
// update time and then by ID, like the update index of the BuntDB storage.
func (m *Memory) Orders(filters ...OrderFilter) ([]*model.Order, error) {
	m.mu.RLock()
	result := make([]*model.Order, 0)
	for i := range m.orders {
		if accept(m.orders[i], filters) {
			order := m.orders[i]
			result = append(result, &order)
		}
	}
	m.mu.RUnlock()

	sort.SliceStable(result, func(i, j int) bool {
		if !result[i].UpdatedAt.Equal(result[j].UpdatedAt) {
			return result[i].UpdatedAt.Before(result[j].UpdatedAt)
		}
		return result[i].ID < result[j].ID
	})
	return result, nil
}

func accept(order model.Order, filters []OrderFilter) bool {
	for _, filter := range filters {
		if !filter(order) {
			return false
		}
	}
	return true
}
