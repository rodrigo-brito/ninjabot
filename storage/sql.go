package storage

import (
	"time"

	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/rodrigo-brito/ninjabot/model"
)

type SQL struct {
	db *gorm.DB
}

// FromSQL creates a new SQL connections for orders storage. Example of usage:
//
//	import "github.com/glebarez/sqlite"
//	storage, err := storage.FromSQL(sqlite.Open("sqlite.db"), &gorm.Config{})
//	if err != nil {
//		log.Fatal(err)
//	}
func FromSQL(dialect gorm.Dialector, opts ...gorm.Option) (Storage, error) {
	db, err := gorm.Open(dialect, opts...)
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	err = db.AutoMigrate(&model.Order{})
	if err != nil {
		return nil, err
	}

	return &SQL{
		db: db,
	}, nil
}

// CreateOrder creates a new order in a SQL database
func (s *SQL) CreateOrder(order *model.Order) error {
	result := s.db.Create(order) // pass pointer of data to Create
	return result.Error
}

// UpdateOrder updates a given order
func (s *SQL) UpdateOrder(order *model.Order) error {
	o := model.Order{ID: order.ID}
	s.db.First(&o)
	o = *order
	result := s.db.Save(&o)
	return result.Error
}

// Orders filter a list of orders given a filter
func (s *SQL) Orders(filters ...OrderFilter) ([]*model.Order, error) {
	orders := make([]*model.Order, 0)

	result := s.db.Find(&orders)
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		return orders, nil
	}

	return lo.Filter(orders, func(order *model.Order, _ int) bool {
		for _, filter := range filters {
			if !filter(*order) {
				return false
			}
		}
		return true
	}), nil
}
