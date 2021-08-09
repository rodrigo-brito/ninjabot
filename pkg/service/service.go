package service

import (
	"context"
	"time"

	"github.com/rodrigo-brito/ninjabot/pkg/model"
)

type Exchange interface {
	Broker
	Feeder
}

type Feeder interface {
	CandlesByPeriod(ctx context.Context, pair, period string, start, end time.Time) ([]model.Candle, error)
	CandlesByLimit(ctx context.Context, pair, period string, limit int) ([]model.Candle, error)
	CandlesSubscription(pair, timeframe string) (chan model.Candle, chan error)
}

type Broker interface {
	Account() (model.Account, error)
	Position(symbol string) (asset, quote float64, err error)
	Order(symbol string, id int64) (model.Order, error)
	OrderOCO(side model.SideType, symbol string, size, price, stop, stopLimit float64) ([]model.Order, error)
	OrderLimit(side model.SideType, symbol string, size float64, limit float64) (model.Order, error)
	OrderMarket(side model.SideType, symbol string, size float64) (model.Order, error)
	Cancel(model.Order) error
}

type Notifier interface {
	Notify(string)
	OnOrder(order model.Order)
	OrError(err error)
}

type Telegram interface {
	Notify(text string)
	Start()
}
