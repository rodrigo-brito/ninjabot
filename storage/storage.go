package storage

import (
	"fmt"
	"time"

	"github.com/rodrigo-brito/ninjabot/model"

	"github.com/jmoiron/sqlx"
	//nolint
	_ "github.com/mattn/go-sqlite3"
)

type Storage interface {
	CreateOrder(order *model.Order) error
	UpdateOrderStatus(id int64, status model.OrderStatusType) error
	UpdateOrder(id int64, updatedAt time.Time, status model.OrderStatusType, quantity, price float64) error
	GetPendingOrders() ([]*model.Order, error)
	FilterOrders(updatedBefore time.Time, status model.OrderStatusType, symbol string, id int64) ([]*model.Order, error)
}

func FromMemory() (Storage, error) {
	return New("file::memory:?mode=memory&cache=shared&_fk=1&_mutex=full")
}

func FromFile(file string) (Storage, error) {
	return New(fmt.Sprintf("file:%s?cache=shared&_fk=1&_mutex=full", file))
}

func New(storageType string) (Storage, error) {
	db, err := sqlx.Connect("sqlite3", storageType)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS orders(
		id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
		exchange_id INTEGER NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		symbol VARCHAR (255) NOT NULL,
		side VARCHAR (255) NOT NULL,
		type VARCHAR (255) NOT NULL,
		status VARCHAR (255) NOT NULL,
		price REAL NOT NULL,
		quantity REAL NOT NULL,
		group_id INTEGER,
		stop REAL
	);`)
	if err != nil {
		return nil, err
	}

	return storage{db: db}, nil
}

type storage struct {
	db *sqlx.DB
}

func (s storage) CreateOrder(order *model.Order) error {
	res, err := s.db.Exec(`INSERT INTO orders (exchange_id, symbol, side, type, status, price, 
                    quantity, created_at, updated_at, stop, group_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`,
		order.ExchangeID, order.Symbol, order.Side, order.Type, order.Status, order.Price,
		order.Quantity, order.CreatedAt, order.UpdatedAt, order.Stop, order.GroupID)
	if err != nil {
		return fmt.Errorf("error on save order: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return err
	}

	order.ID = id
	return nil
}

func (s storage) UpdateOrderStatus(id int64, status model.OrderStatusType) error {
	_, err := s.db.Exec("UPDATE orders SET status = ? WHERE id = ?;", status, id)
	return err
}

func (s storage) UpdateOrder(id int64, updatedAt time.Time, status model.OrderStatusType,
	quantity, price float64) error {

	_, err := s.db.Exec(`UPDATE orders SET updated_at = ?, status = ?, quantity = ?, price = ? 
		WHERE id = ?;`, updatedAt, status, quantity, price, id)
	return err
}

func (s storage) GetPendingOrders() ([]*model.Order, error) {
	orders := make([]*model.Order, 0)
	err := s.db.Select(&orders, `SELECT id, exchange_id, symbol, side, type, status, price, quantity, created_at,
       		updated_at, stop, group_id 
		FROM orders WHERE status IN(?, ?, ?) ORDER BY id ASC;`,
		model.OrderStatusTypeNew, model.OrderStatusTypePartiallyFilled, model.OrderStatusTypePendingCancel)
	if err != nil {
		return nil, err
	}

	return orders, nil
}

func (s storage) FilterOrders(updatedAt time.Time, status model.OrderStatusType, symbol string, id int64) (
	[]*model.Order, error) {

	orders := make([]*model.Order, 0)
	err := s.db.Select(&orders, `SELECT id, exchange_id, symbol, side, type, status, price, quantity, created_at, 
       		updated_at, stop, group_id 
		FROM orders where updated_at <= ? AND status = ? AND symbol = ? AND id != ? 
		ORDER BY updated_at ASC;`, updatedAt, status, symbol, id)
	if err != nil {
		return nil, err
	}

	return orders, nil
}
