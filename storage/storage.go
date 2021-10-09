package storage

import (
	"encoding/json"
	"log"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/rodrigo-brito/ninjabot/model"

	"github.com/tidwall/buntdb"
)

type OrderFilter func(model.Order) bool

type Storage interface {
	CreateOrder(order *model.Order) error
	UpdateOrder(order *model.Order) error
	Orders(filters ...OrderFilter) ([]*model.Order, error)
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

func (b Bunt) UpdateOrder(order *model.Order) error {
	return b.db.Update(func(tx *buntdb.Tx) error {
		id := strconv.FormatInt(order.ID, 10)

		content, err := json.Marshal(order)
		if err != nil {
			return err
		}

		_, _, err = tx.Set(id, string(content), nil)
		return err
	})
}

func (b Bunt) Orders(filters ...OrderFilter) ([]*model.Order, error) {
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
		return order.Symbol == pair
	}
}

func WithUpdateAtBeforeOrEqual(time time.Time) OrderFilter {
	return func(order model.Order) bool {
		return !order.UpdatedAt.After(time)
	}
}
