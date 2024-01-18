package storage

import (
	"time"

	"github.com/rodrigo-brito/ninjabot/model"
)

type OrderFilter func(model.Order) bool
type PositionFilter func(model.Position) bool

type Storage interface {
	CreateOrder(order *model.Order) error
	UpdateOrder(order *model.Order) error
	Orders(filters ...OrderFilter) ([]*model.Order, error)
	CreatePosition(position *model.Position) error
	Positions(filters ...PositionFilter) ([]*model.Position, error)
}

func WithStatusIn(status ...model.OrderStatusType) OrderFilter {
	return func(order model.Order) bool {
		for _, s := range status {
			if s == order.Status {
				return true
			}
		}
		return false
	}
}

func WithStatus(status model.OrderStatusType) OrderFilter {
	return func(order model.Order) bool {
		return order.Status == status
	}
}

func WithPair(pair string) OrderFilter {
	return func(order model.Order) bool {
		return order.Pair == pair
	}
}

func WithUpdateAtBeforeOrEqual(time time.Time) OrderFilter {
	return func(order model.Order) bool {
		return !order.UpdatedAt.After(time)
	}
}
