package exchange

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/rodrigo-brito/ninjabot/pkg/model"
)

type assetInfo struct {
	Free float64
	Lock float64
}

type PaperWallet struct {
	sync.Mutex
	ctx          context.Context
	baseCoin     string
	counter      int64
	takerFee     float64
	makerFee     float64
	initialValue float64
	feeder       Feeder
	orders       []model.Order
	assets       map[string]*assetInfo
	avgPrice     map[string]float64
	lastCandle   map[string]model.Candle
	fistCandle   map[string]model.Candle
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

func WithDataFeed(feeder Feeder) PaperWalletOption {
	return func(wallet *PaperWallet) {
		wallet.feeder = feeder
	}
}

func NewPaperWallet(ctx context.Context, baseCoin string, options ...PaperWalletOption) *PaperWallet {
	wallet := PaperWallet{
		ctx:        ctx,
		baseCoin:   baseCoin,
		orders:     make([]model.Order, 0),
		assets:     make(map[string]*assetInfo),
		fistCandle: make(map[string]model.Candle),
		lastCandle: make(map[string]model.Candle),
		avgPrice:   make(map[string]float64),
	}

	for _, option := range options {
		option(&wallet)
	}

	wallet.initialValue = wallet.assets[wallet.baseCoin].Free
	log.Info("[SETUP] Using paper wallet")
	log.Infof("[SETUP] Initial Portfolio = %f %s", wallet.initialValue, wallet.baseCoin)

	return &wallet
}

func (p *PaperWallet) Summary() {
	var (
		total        float64
		marketChange float64
	)

	fmt.Println("--------------")
	fmt.Println("WALLET SUMMARY")
	fmt.Println("--------------")
	for pair, price := range p.avgPrice {
		asset, _ := SplitAssetQuote(pair)
		quantity := p.assets[asset].Free + p.assets[asset].Lock
		total += quantity * price
		marketChange += (p.lastCandle[pair].Close - p.fistCandle[pair].Close) / p.fistCandle[pair].Close
		fmt.Printf("%f %s\n", quantity, asset)
	}
	avgMarketChange := marketChange / float64(len(p.avgPrice))
	baseCoinValue := p.assets[p.baseCoin].Free + p.assets[p.baseCoin].Lock
	profit := total + baseCoinValue - p.initialValue
	fmt.Printf("%f %s\n", baseCoinValue, p.baseCoin)
	fmt.Println("--------------")
	fmt.Println("START PORTFOLIO = ", p.initialValue, p.baseCoin)
	fmt.Println("FINAL PORTFOLIO = ", total+baseCoinValue, p.baseCoin)
	fmt.Printf("GROSS PROFIT    =  %f %s (%.2f%%)\n", profit, p.baseCoin, profit/p.initialValue*100)
	fmt.Printf("MARKET CHANGE   =  %.2f%%\n", avgMarketChange*100)
	fmt.Println("--------------")
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
	p.Lock()
	defer p.Unlock()

	p.lastCandle[candle.Symbol] = candle
	if _, ok := p.fistCandle[candle.Symbol]; !ok {
		p.fistCandle[candle.Symbol] = candle
	}

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
			orderValue := order.Price * order.Quantity
			walletValue := p.avgPrice[candle.Symbol] * actualQty

			p.orders[i].Status = model.OrderStatusTypeFilled
			p.avgPrice[candle.Symbol] = (walletValue + orderValue) / (actualQty + order.Quantity)
			p.assets[asset].Free = p.assets[asset].Free + order.Quantity
			p.assets[quote].Lock = p.assets[quote].Lock - orderValue
		}

		if order.Side == model.SideTypeSell && order.Price >= candle.Close {
			if _, ok := p.assets[quote]; !ok {
				p.assets[quote] = &assetInfo{}
			}

			profitValue := order.Quantity*order.Price - order.Quantity*p.avgPrice[candle.Symbol]
			percentage := profitValue / (order.Quantity * p.avgPrice[candle.Symbol])
			log.Infof("PROFIT = %.4f %s (%.2f %%)", profitValue, quote, percentage*100)

			p.orders[i].Status = model.OrderStatusTypeFilled
			p.assets[asset].Lock = p.assets[asset].Lock - order.Quantity
			p.assets[quote].Free = p.assets[quote].Free + order.Quantity*order.Price
		}
	}
}

func (p *PaperWallet) Account() (model.Account, error) {
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

func (p *PaperWallet) Position(symbol string) (asset, quote float64, err error) {
	p.Lock()
	defer p.Unlock()

	assetTick, quoteTick := SplitAssetQuote(symbol)
	acc, err := p.Account()
	if err != nil {
		return 0, 0, err
	}
	return acc.Balance(assetTick).Free, acc.Balance(quoteTick).Free, nil
}

func (p *PaperWallet) OrderOCO(side model.SideType, symbol string,
	size, price, stop, stopLimit float64) ([]model.Order, error) {
	p.Lock()
	defer p.Unlock()

	panic("implement me")
}

func (p *PaperWallet) OrderLimit(side model.SideType, symbol string, size float64, limit float64) (model.Order, error) {
	p.Lock()
	defer p.Unlock()

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
		Date:       p.lastCandle[symbol].Time,
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
	p.Lock()
	defer p.Unlock()

	asset, quote := SplitAssetQuote(symbol)
	if side == model.SideTypeSell {
		if value, ok := p.assets[asset]; !ok || value.Free < size {
			return model.Order{}, ErrInsufficientFunds
		}
		if _, ok := p.assets[quote]; !ok {
			p.assets[quote] = &assetInfo{}
		}
		p.assets[asset].Free = p.assets[asset].Free - size
		p.assets[quote].Free = p.assets[quote].Free + p.lastCandle[symbol].Close*size
	} else {
		if value, ok := p.assets[quote]; !ok || value.Free < size*p.lastCandle[symbol].Close {
			return model.Order{}, ErrInsufficientFunds
		}
		if _, ok := p.assets[asset]; !ok {
			p.assets[asset] = &assetInfo{}
		}
		actualQty := p.assets[asset].Free + p.assets[asset].Lock
		p.avgPrice[symbol] = (p.avgPrice[symbol]*actualQty + p.lastCandle[symbol].Close*size) / (actualQty + size)
		p.assets[quote].Free = p.assets[quote].Free - (size * p.lastCandle[symbol].Close)
		p.assets[asset].Free = p.assets[asset].Free + size
	}

	p.counter = p.counter + 1
	order := model.Order{
		ExchangeID: p.counter,
		Date:       p.lastCandle[symbol].Time,
		Symbol:     symbol,
		Side:       side,
		Type:       model.OrderTypeMarket,
		Status:     model.OrderStatusTypeFilled,
		Price:      p.lastCandle[symbol].Close,
		Quantity:   size,
	}
	p.orders = append(p.orders, order)
	return order, nil
}

func (p *PaperWallet) Cancel(order model.Order) error {
	p.Lock()
	defer p.Unlock()

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

func (p *PaperWallet) CandlesByPeriod(ctx context.Context, pair, period string,
	start, end time.Time) ([]model.Candle, error) {
	return p.feeder.CandlesByPeriod(ctx, pair, period, start, end)
}

func (p *PaperWallet) CandlesByLimit(ctx context.Context, pair, period string, limit int) ([]model.Candle, error) {
	return p.feeder.CandlesByLimit(ctx, pair, period, limit)
}

func (p *PaperWallet) CandlesSubscription(pair, timeframe string) (chan model.Candle, chan error) {
	return p.feeder.CandlesSubscription(pair, timeframe)
}
