package storage

import (
	"encoding/json"
	"log"
	"sync/atomic"
	"time"

	"github.com/rodrigo-brito/ninjabot/model"
	"gorm.io/gorm"
)

type SQL struct {
	lastID int64
	db     *gorm.DB
}

type sqlData struct {
	Key   int64 `gorm:"primaryKey"`
	Value string
}

func (sqlData) TableName() string {
	return "ninjabot_storage"
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

	err = db.AutoMigrate(&sqlData{})
	if err != nil {
		return nil, err
	}

	return &SQL{
		lastID: 0,
		db:     db,
	}, nil
}

func (s *SQL) getID() int64 {
	return atomic.AddInt64(&s.lastID, 1)
}

func (s *SQL) CreateOrder(order *model.Order) error {
	order.ID = s.getID()
	content, err := json.Marshal(order)
	if err != nil {
		return err
	}

	data := sqlData{
		Key:   order.ID,
		Value: string(content),
	}
	result := s.db.Create(&data) // pass pointer of data to Create
	return result.Error
}

func (s *SQL) UpdateOrder(order *model.Order) error {
	content, err := json.Marshal(order)
	if err != nil {
		return err
	}

	result := s.db.Model(&sqlData{}).Where("key = ?", order.ID).Update("value", string(content))
	if result.Error != nil {
		return err
	}
	return result.Error
}

func (s *SQL) Orders(filters ...OrderFilter) ([]*model.Order, error) {
	orders := make([]*model.Order, 0)
	var sqlDatas []sqlData

	result := s.db.Find(&sqlDatas)
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		return orders, nil
	}

	for i := range sqlDatas {
		var order model.Order
		err := json.Unmarshal([]byte(sqlDatas[i].Value), &order)
		if err != nil {
			log.Println(err)
			continue
		}
		isFiltered := true
		for _, filter := range filters {
			if ok := filter(order); !ok {
				isFiltered = false
				break
			} else {
				isFiltered = true
			}
		}
		if isFiltered {
			orders = append(orders, &order)
		}
	}
	return orders, nil
}
