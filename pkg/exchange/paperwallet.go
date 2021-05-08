package exchange

import (
	"context"
	"errors"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/rodrigo-brito/ninjabot/pkg/model"
)

type assetInfo struct {
	Free float64
	Lock float64
}

type PaperWallet struct {
	ctx        context.Context
	counter    int64
	takerFee   float64
	makerFee   float64
	dataSource Feeder
	orders     []model.Order
	assets     map[string]*assetInfo
	avgPrice   map[string]float64
	lastCandle map[string]float64
}

type PaperWalletOption func(*PaperWallet)

func WithPaperAsset(symbol string, amount float64) PaperWalletOption {
	return func(wallet *PaperWallet) {
		wallet.assets[symbol] = &assetInfo{
			Free: amount,
			Lock: 0,
		}
	}
}

func WithPaperFee(maker, taker float64) PaperWalletOption {
	return func(wallet *PaperWallet) {
		wallet.makerFee = maker
		wallet.takerFee = taker
	}
}

func WithDataSource(feeder Feeder) PaperWalletOption {
	return func(wallet *PaperWallet) {
		wallet.dataSource = feeder
	}
}

func NewPaperWallet(ctx context.Context, options ...PaperWalletOption) *PaperWallet {
	wallet := PaperWallet{
		ctx:        ctx,
		orders:     make([]model.Order, 0),
		assets:     make(map[string]*assetInfo),
		lastCandle: make(map[string]float64),
		avgPrice:   make(map[string]float64),
	}

	for _, option := range options {
		option(&wallet)
	}

	log.Info("[SETUP] Using paper wallet")

	return &wallet
}

func (p *PaperWallet) lockFunds(asset string, amount float64) error {
	if value, ok := p.assets[asset]; !ok || value.Free < amount {
		return ErrInsufficientFunds
	}
	p.assets[asset].Free = p.assets[asset].Free - amount
	p.assets[asset].Lock = p.assets[asset].Lock + amount
	log.Infof("%s -> LOCK = %f / FREE %f", asset, p.assets[asset].Lock, p.assets[asset].Free)
	return nil
}

func (p *PaperWallet) OnCandle(candle model.Candle) {
	p.lastCandle[candle.Symbol] = candle.Close

	for i, order := range p.orders {
		if order.Status != model.OrderStatusTypeNew {
			continue
		}

		asset, quote := SplitAssetQuote(order.Symbol)
		if order.Side == model.SideTypeBuy && order.Price <= candle.Close {
			if _, ok := p.assets[asset]; !ok {
				p.assets[asset] = &assetInfo{}
			}

			actualQty := p.assets[asset].Free + p.assets[asset].Lock
			p.orders[i].Status = model.OrderStatusTypeFilled
			p.avgPrice[candle.Symbol] = (p.avgPrice[candle.Symbol]*actualQty + order.Price*order.Quantity) / (actualQty + order.Quantity)
			p.assets[asset].Free = p.assets[asset].Free + order.Quantity
			p.assets[quote].Lock = p.assets[quote].Lock - order.Quantity*order.Price

			log.Infof("%s -> LOCK = %f / FREE %f", asset, p.assets[asset].Lock, p.assets[asset].Free)
			log.Infof("%s -> LOCK = %f / FREE %f", quote, p.assets[quote].Lock, p.assets[quote].Free)
		}

		if order.Side == model.SideTypeSell && order.Price >= candle.Close {
			if _, ok := p.assets[quote]; !ok {
				p.assets[quote] = &assetInfo{}
			}

			profit := order.Quantity*order.Price - order.Quantity*p.avgPrice[candle.Symbol]
			percentage := profit / (order.Quantity * p.avgPrice[candle.Symbol])
			log.Infof("PROFIT = %.4f %s (%.2f %%)", profit, quote, percentage*100)

			p.orders[i].Status = model.OrderStatusTypeFilled
			p.assets[asset].Lock = p.assets[asset].Lock - order.Quantity
			p.assets[quote].Free = p.assets[quote].Free + order.Quantity*order.Price

			log.Infof("%s -> LOCK = %f / FREE %f", asset, p.assets[asset].Lock, p.assets[asset].Free)
			log.Infof("%s -> LOCK = %f / FREE %f", quote, p.assets[quote].Lock, p.assets[quote].Free)
		}
	}
}

func (p PaperWallet) Account() (model.Account, error) {
	balances := make([]model.Balance, 0)
	for symbol, info := range p.assets {
		balances = append(balances, model.Balance{
			Tick: symbol,
			Free: info.Free,
			Lock: info.Lock,
		})
	}

	return model.Account{
		Balances: balances,
	}, nil
}

func (p *PaperWallet) OrderOCO(side model.SideType, symbol string, size, price, stop, stopLimit float64) ([]model.Order, error) {
	panic("implement me")
}

func (p *PaperWallet) OrderLimit(side model.SideType, symbol string, size float64, limit float64) (model.Order, error) {
	p.counter = p.counter + 1
	asset, quote := SplitAssetQuote(symbol)
	if side == model.SideTypeSell {
		err := p.lockFunds(asset, size)
		if err != nil {
			return model.Order{}, err
		}
	} else {
		err := p.lockFunds(quote, size*limit)
		if err != nil {
			return model.Order{}, err
		}
	}
	order := model.Order{
		ExchangeID: p.counter,
		Date:       time.Now(),
		Symbol:     symbol,
		Side:       side,
		Type:       model.OrderTypeLimit,
		Status:     model.OrderStatusTypeNew,
		Price:      limit,
		Quantity:   size,
	}
	p.orders = append(p.orders, order)
	return order, nil
}

func (p *PaperWallet) OrderMarket(side model.SideType, symbol string, size float64) (model.Order, error) {
	asset, quote := SplitAssetQuote(symbol)
	if side == model.SideTypeSell {
		if value, ok := p.assets[asset]; !ok || value.Free < size {
			return model.Order{}, ErrInsufficientFunds
		}
		if _, ok := p.assets[quote]; !ok {
			p.assets[quote] = &assetInfo{}
		}
		p.assets[asset].Free = p.assets[asset].Free - size
		p.assets[quote].Free = p.assets[quote].Free + p.lastCandle[symbol]*size
	} else {
		if value, ok := p.assets[quote]; !ok || value.Free < size*p.lastCandle[symbol] {
			return model.Order{}, ErrInsufficientFunds
		}
		if _, ok := p.assets[asset]; !ok {
			p.assets[asset] = &assetInfo{}
		}
		actualQty := p.assets[asset].Free + p.assets[asset].Lock
		p.avgPrice[symbol] = (p.avgPrice[symbol]*actualQty + p.lastCandle[symbol]*size) / (actualQty + size)
		p.assets[quote].Free = p.assets[quote].Free - (size * p.lastCandle[symbol])
		p.assets[asset].Free = p.assets[asset].Free + size
	}

	p.counter = p.counter + 1
	order := model.Order{
		ExchangeID: p.counter,
		Date:       time.Now(),
		Symbol:     symbol,
		Side:       side,
		Type:       model.OrderTypeMarket,
		Status:     model.OrderStatusTypeFilled,
		Price:      p.lastCandle[symbol],
		Quantity:   size,
	}
	p.orders = append(p.orders, order)
	return order, nil
}

func (p *PaperWallet) Cancel(order model.Order) error {
	for i, o := range p.orders {
		if o.ExchangeID == order.ExchangeID {
			p.orders[i].Status = model.OrderStatusTypeCanceled
		}
	}
	return nil
}

func (p *PaperWallet) Order(symbol string, id int64) (model.Order, error) {
	for _, order := range p.orders {
		if order.ExchangeID == id {
			return order, nil
		}
	}
	return model.Order{}, errors.New("order not found")
}

func (p *PaperWallet) CandlesByPeriod(ctx context.Context, pair, period string, start, end time.Time) ([]model.Candle, error) {
	return p.dataSource.CandlesByPeriod(ctx, pair, period, start, end)
}

func (p *PaperWallet) CandlesByLimit(ctx context.Context, pair, period string, limit int) ([]model.Candle, error) {
	return p.dataSource.CandlesByLimit(ctx, pair, period, limit)
}

func (p *PaperWallet) CandlesSubscription(pair, timeframe string) (chan model.Candle, chan error) {
	return p.dataSource.CandlesSubscription(pair, timeframe)
}
