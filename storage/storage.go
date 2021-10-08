package storage

import (
	"encoding/json"
	"log"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/tidwall/gjson"

	"github.com/rodrigo-brito/ninjabot/model"

	"github.com/tidwall/buntdb"
)

type Filter func(model.Order) bool

type Storage interface {
	CreateOrder(order *model.Order) error
	UpdateOrderStatus(id int64, status model.OrderStatusType) error
	UpdateOrder(id int64, updatedAt time.Time, status model.OrderStatusType, quantity, price float64) error
	GetPendingOrders() ([]*model.Order, error)
	Filter(filters ...Filter) ([]*model.Order, error)
}

func FromMemory() (Storage, error) {
	return new(":memory:")
}

func FromFile(file string) (Storage, error) {
	return new(file)
}

type Bunt struct {
	lastID int64
	db     *buntdb.DB
}

func new(sourceFile string) (Storage, error) {
	db, err := buntdb.Open(sourceFile)
	if err != nil {
		log.Fatal(err)
	}

	err = db.CreateIndex("id_index", "*", buntdb.IndexJSON("id"))
	if err != nil {
		return nil, err
	}

	err = db.CreateIndex("symbol_index", "*", buntdb.IndexJSON("symbol"))
	if err != nil {
		return nil, err
	}

	err = db.CreateIndex("update_index", "*", buntdb.IndexJSON("updated_at"))
	if err != nil {
		return nil, err
	}

	err = db.CreateIndex("status_index", "*", buntdb.IndexJSON("status"))
	if err != nil {
		return nil, err
	}

	return &Bunt{
		db: db,
	}, nil
}

func (b *Bunt) getID() int64 {
	return atomic.AddInt64(&b.lastID, 1)
}

func (b *Bunt) CreateOrder(order *model.Order) error {
	return b.db.Update(func(tx *buntdb.Tx) error {
		order.ID = b.getID()
		content, err := json.Marshal(order)
		if err != nil {
			return err
		}

		_, _, err = tx.Set(strconv.FormatInt(order.ID, 10), string(content), nil)
		return err
	})
}

func (b Bunt) UpdateOrderStatus(id int64, status model.OrderStatusType) error {
	return b.db.View(func(tx *buntdb.Tx) error {
		idStr := strconv.FormatInt(id, 10)
		var order model.Order
		content, err := tx.Get(idStr)
		if err != nil {
			return err
		}

		err = json.Unmarshal([]byte(content), &order)
		if err != nil {
			return err
		}

		order.Status = status

		newContent, err := json.Marshal(order)
		if err != nil {
			return err
		}

		_, _, err = tx.Set(idStr, string(newContent), nil)
		return err
	})
}

func (b Bunt) UpdateOrder(id int64, updatedAt time.Time, status model.OrderStatusType, quantity, price float64) error {
	return b.db.Update(func(tx *buntdb.Tx) error {
		idStr := strconv.FormatInt(id, 10)
		var order model.Order
		content, err := tx.Get(idStr)
		if err != nil {
			return err
		}

		err = json.Unmarshal([]byte(content), &order)
		if err != nil {
			return err
		}

		order.Status = status
		order.UpdatedAt = updatedAt
		order.Quantity = quantity
		order.Price = price

		newContent, err := json.Marshal(order)
		if err != nil {
			return err
		}

		_, _, err = tx.Set(idStr, string(newContent), nil)
		return err
	})
}

func (b Bunt) GetPendingOrders() ([]*model.Order, error) {
	pending := make([]*model.Order, 0)
	err := b.db.View(func(tx *buntdb.Tx) error {
		err := tx.Ascend("status_index", func(key, value string) bool {
			status := gjson.Get(value, "status").String()
			if status == string(model.OrderStatusTypeNew) ||
				status == string(model.OrderStatusTypePartiallyFilled) ||
				status == string(model.OrderStatusTypePendingCancel) {
				var order model.Order
				err := json.Unmarshal([]byte(value), &order)
				if err != nil {
					log.Println(err)
				}
				pending = append(pending, &order)
			}
			return true
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return pending, nil
}

func (b Bunt) Filter(filters ...Filter) ([]*model.Order, error) {
	orders := make([]*model.Order, 0)
	err := b.db.View(func(tx *buntdb.Tx) error {
		err := tx.Ascend("update_index", func(key, value string) bool {
			var order model.Order
			err := json.Unmarshal([]byte(value), &order)
			if err != nil {
				log.Println(err)
				return true
			}

			for _, filter := range filters {
				if ok := filter(order); !ok {
					return true
				}
			}

			orders = append(orders, &order)

			return true
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return orders, nil
}

func WithStatus(status model.OrderStatusType) Filter {
	return func(order model.Order) bool {
		return order.Status == status
	}
}

func WithPair(pair string) Filter {
	return func(order model.Order) bool {
		return order.Symbol == pair
	}
}

func WithUpdateAtBeforeOrEqual(time time.Time) Filter {
	return func(order model.Order) bool {
		return !order.UpdatedAt.After(time)
	}
}
