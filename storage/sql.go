package storage

import (
	"time"

	"github.com/rodrigo-brito/ninjabot/model"
	"gorm.io/gorm"
)

type SQL struct {
	db *gorm.DB
}

func FromSQL(dialector gorm.Dialector, opts ...gorm.Option) (Storage, error) {
	db, err := gorm.Open(dialector, opts...)
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

func (s *SQL) CreateOrder(order *model.Order) error {
	result := s.db.Create(order) // pass pointer of data to Create
	return result.Error
}

func (s *SQL) UpdateOrder(order *model.Order) error {
	o := model.Order{ID: order.ID}
	s.db.First(&o)
	o = *order
	result := s.db.Save(&o)
	return result.Error
}

func (s *SQL) Orders(filters ...OrderFilter) ([]*model.Order, error) {
	orders := make([]*model.Order, 0)
	var os []model.Order

	result := s.db.Find(&os)
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		return orders, nil
	}

	for i := range os {
		isFiltered := true
		for _, filter := range filters {
			if ok := filter(os[i]); !ok {
				isFiltered = false
				break
			} else {
				isFiltered = true
			}
		}
		if isFiltered {
			orders = append(orders, &os[i])
		}
	}
	return orders, nil
}
