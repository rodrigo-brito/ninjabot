package service

import (
	"context"
	"time"

	"github.com/rodrigo-brito/ninjabot/model"
)

type Exchange interface {
	Broker
	Feeder
}

type Feeder interface {
	AssetsInfo(pair string) model.AssetInfo
	LastQuote(ctx context.Context, pair string) (float64, error)
	CandlesByPeriod(ctx context.Context, pair, period string, start, end time.Time) ([]model.Candle, error)
	CandlesByLimit(ctx context.Context, pair, period string, limit int) ([]model.Candle, error)
	CandlesSubscription(ctx context.Context, pair, timeframe string) (chan model.Candle, chan error)
}

type Broker interface {
	Account() (model.Account, error)
	Position(pair string) (asset, quote float64, err error)
	Order(pair string, id int64) (model.Order, error)
	CreateOrderOCO(side model.SideType, pair string, size, price, stop, stopLimit float64) ([]model.Order, error)
	CreateOrderLimit(side model.SideType, pair string, size float64, limit float64) (model.Order, error)
	CreateOrderMarket(side model.SideType, pair string, size float64) (model.Order, error)
	CreateOrderMarketQuote(side model.SideType, pair string, quote float64) (model.Order, error)
	CreateOrderStop(pair string, quantity float64, limit float64) (model.Order, error)
	Cancel(model.Order) error
}

type Notifier interface {
	Notify(string)
	OnOrder(order model.Order)
	OnError(err error)
}

type Telegram interface {
	Notifier
	Start()
}
