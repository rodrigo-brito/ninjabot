package exchange

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2/common"
	log "github.com/sirupsen/logrus"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/service"
)

type assetInfo struct {
	Free float64
	Lock float64
}

type AssetValue struct {
	Time  time.Time
	Value float64
}

type PaperWallet struct {
	sync.Mutex
	ctx          context.Context
	baseCoin     string
	counter      int64
	takerFee     float64
	makerFee     float64
	initialValue float64
	feeder       service.Feeder
	orders       []model.Order
	assets       map[string]*assetInfo
	avgPrice     map[string]float64
	volume       map[string]float64
	lastCandle   map[string]model.Candle
	fistCandle   map[string]model.Candle
	assetValues  map[string][]AssetValue
	equityValues []AssetValue
}

func (p *PaperWallet) AssetsInfo(pair string) model.AssetInfo {
	asset, quote := SplitAssetQuote(pair)
	return model.AssetInfo{
		BaseAsset:          asset,
		QuoteAsset:         quote,
		MaxPrice:           math.MaxFloat64,
		MaxQuantity:        math.MaxFloat64,
		StepSize:           0.00000001,
		TickSize:           0.00000001,
		QuotePrecision:     8,
		BaseAssetPrecision: 8,
	}
}

type PaperWalletOption func(*PaperWallet)

func WithPaperAsset(pair string, amount float64) PaperWalletOption {
	return func(wallet *PaperWallet) {
		wallet.assets[pair] = &assetInfo{
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

func WithDataFeed(feeder service.Feeder) PaperWalletOption {
	return func(wallet *PaperWallet) {
		wallet.feeder = feeder
	}
}

func NewPaperWallet(ctx context.Context, baseCoin string, options ...PaperWalletOption) *PaperWallet {
	wallet := PaperWallet{
		ctx:          ctx,
		baseCoin:     baseCoin,
		orders:       make([]model.Order, 0),
		assets:       make(map[string]*assetInfo),
		fistCandle:   make(map[string]model.Candle),
		lastCandle:   make(map[string]model.Candle),
		avgPrice:     make(map[string]float64),
		volume:       make(map[string]float64),
		assetValues:  make(map[string][]AssetValue),
		equityValues: make([]AssetValue, 0),
	}

	for _, option := range options {
		option(&wallet)
	}

	wallet.initialValue = wallet.assets[wallet.baseCoin].Free
	log.Info("[SETUP] Using paper wallet")
	log.Infof("[SETUP] Initial Portfolio = %f %s", wallet.initialValue, wallet.baseCoin)

	return &wallet
}

func (p *PaperWallet) ID() int64 {
	p.counter++
	return p.counter
}

func (p *PaperWallet) Pairs() []string {
	pairs := make([]string, 0)
	for pair := range p.assets {
		pairs = append(pairs, pair)
	}
	return pairs
}

func (p *PaperWallet) LastQuote(ctx context.Context, pair string) (float64, error) {
	return p.feeder.LastQuote(ctx, pair)
}

func (p *PaperWallet) AssetValues(pair string) []AssetValue {
	return p.assetValues[pair]
}

func (p *PaperWallet) EquityValues() []AssetValue {
	return p.equityValues
}

func (p *PaperWallet) MaxDrawdown() (float64, time.Time, time.Time) {
	if len(p.equityValues) < 1 {
		return 0, time.Time{}, time.Time{}
	}

	localMin := math.MaxFloat64
	localMinBase := p.equityValues[0].Value
	localMinStart := p.equityValues[0].Time
	localMinEnd := p.equityValues[0].Time

	globalMin := localMin
	globalMinBase := localMinBase
	globalMinStart := localMinStart
	globalMinEnd := localMinEnd

	for i := 1; i < len(p.equityValues); i++ {
		diff := p.equityValues[i].Value - p.equityValues[i-1].Value

		if localMin > 0 {
			localMin = diff
			localMinBase = p.equityValues[i-1].Value
			localMinStart = p.equityValues[i-1].Time
			localMinEnd = p.equityValues[i].Time
		} else {
			localMin += diff
			localMinEnd = p.equityValues[i].Time
		}

		if localMin < globalMin {
			globalMin = localMin
			globalMinBase = localMinBase
			globalMinStart = localMinStart
			globalMinEnd = localMinEnd
		}
	}

	return globalMin / globalMinBase, globalMinStart, globalMinEnd
}

func (p *PaperWallet) Summary() {
	var (
		total        float64
		marketChange float64
		volume       float64
	)

	fmt.Println("-- FINAL WALLET --")
	for pair, price := range p.avgPrice {
		asset, quote := SplitAssetQuote(pair)
		quantity := p.assets[asset].Free + p.assets[asset].Lock
		total += quantity * price
		marketChange += (p.lastCandle[pair].Close - p.fistCandle[pair].Close) / p.fistCandle[pair].Close
		fmt.Printf("%.4f %s = %.4f %s\n", quantity, asset, total, quote)
	}

	avgMarketChange := marketChange / float64(len(p.avgPrice))
	baseCoinValue := p.assets[p.baseCoin].Free + p.assets[p.baseCoin].Lock
	profit := total + baseCoinValue - p.initialValue
	fmt.Printf("%.4f %s\n", baseCoinValue, p.baseCoin)
	fmt.Println()
	maxDrawDown, _, _ := p.MaxDrawdown()
	fmt.Println("----- RETURNS -----")
	fmt.Printf("START PORTFOLIO     = %.2f %s\n", p.initialValue, p.baseCoin)
	fmt.Printf("FINAL PORTFOLIO     = %.2f %s\n", total+baseCoinValue, p.baseCoin)
	fmt.Printf("GROSS PROFIT        =  %f %s (%.2f%%)\n", profit, p.baseCoin, profit/p.initialValue*100)
	fmt.Printf("MARKET CHANGE (B&H) =  %.2f%%\n", avgMarketChange*100)
	fmt.Println()
	fmt.Println("------ RISK -------")
	fmt.Printf("MAX DRAWDOWN = %.2f %%\n", maxDrawDown*100)
	fmt.Println()
	fmt.Println("------ VOLUME -----")
	for pair, vol := range p.volume {
		volume += vol
		fmt.Printf("%s         = %.2f %s\n", pair, vol, p.baseCoin)
	}
	fmt.Printf("TOTAL           = %.2f %s\n", volume, p.baseCoin)
	fmt.Printf("COSTS (0.001*V) = %.2f %s (ESTIMATION) \n", volume*0.001, p.baseCoin)
	fmt.Println("-------------------")
}

func (p *PaperWallet) lockFunds(asset string, amount float64) error {
	if value, ok := p.assets[asset]; !ok || value.Free < amount {
		return &OrderError{
			Err:      ErrInsufficientFunds,
			Pair:     asset,
			Quantity: amount,
		}
	}
	p.assets[asset].Free = p.assets[asset].Free - amount
	p.assets[asset].Lock = p.assets[asset].Lock + amount
	log.Infof("%s -> LOCK = %f / FREE %f", asset, p.assets[asset].Lock, p.assets[asset].Free)
	return nil
}

func (p *PaperWallet) OnCandle(candle model.Candle) {
	p.Lock()
	defer p.Unlock()

	p.lastCandle[candle.Pair] = candle
	if _, ok := p.fistCandle[candle.Pair]; !ok {
		p.fistCandle[candle.Pair] = candle
	}

	for i, order := range p.orders {
		if order.Pair != candle.Pair || order.Status != model.OrderStatusTypeNew {
			continue
		}

		if _, ok := p.volume[candle.Pair]; !ok {
			p.volume[candle.Pair] = 0
		}

		asset, quote := SplitAssetQuote(order.Pair)
		if order.Side == model.SideTypeBuy && order.Price >= candle.Close {
			if _, ok := p.assets[asset]; !ok {
				p.assets[asset] = &assetInfo{}
			}

			actualQty := p.assets[asset].Free + p.assets[asset].Lock
			orderVolume := order.Price * order.Quantity
			walletValue := p.avgPrice[candle.Pair] * actualQty

			p.volume[candle.Pair] += orderVolume
			p.orders[i].UpdatedAt = candle.Time
			p.orders[i].Status = model.OrderStatusTypeFilled
			p.avgPrice[candle.Pair] = (walletValue + orderVolume) / (actualQty + order.Quantity)
			p.assets[asset].Free = p.assets[asset].Free + order.Quantity
			p.assets[quote].Lock = p.assets[quote].Lock - orderVolume
		}

		if order.Side == model.SideTypeSell {
			var orderPrice float64
			if (order.Type == model.OrderTypeLimit ||
				order.Type == model.OrderTypeLimitMaker ||
				order.Type == model.OrderTypeTakeProfit ||
				order.Type == model.OrderTypeTakeProfitLimit) &&
				candle.High >= order.Price {
				orderPrice = order.Price
			} else if (order.Type == model.OrderTypeStopLossLimit ||
				order.Type == model.OrderTypeStopLoss) &&
				candle.Low <= *order.Stop {
				orderPrice = *order.Stop
			} else {
				continue
			}

			// Cancel other orders from same group
			if order.GroupID != nil {
				for j, groupOrder := range p.orders {
					if groupOrder.GroupID != nil && *groupOrder.GroupID == *order.GroupID &&
						groupOrder.ExchangeID != order.ExchangeID {
						p.orders[j].Status = model.OrderStatusTypeCanceled
						p.orders[j].UpdatedAt = candle.Time
						break
					}
				}
			}

			if _, ok := p.assets[quote]; !ok {
				p.assets[quote] = &assetInfo{}
			}

			orderVolume := order.Quantity * orderPrice
			profitValue := order.Quantity*orderPrice - order.Quantity*p.avgPrice[candle.Pair]
			percentage := profitValue / (order.Quantity * p.avgPrice[candle.Pair])
			log.Infof("PROFIT = %.4f %s (%.2f %%)", profitValue, quote, percentage*100)

			p.volume[candle.Pair] += orderVolume
			p.orders[i].UpdatedAt = candle.Time
			p.orders[i].Status = model.OrderStatusTypeFilled
			p.assets[asset].Lock = p.assets[asset].Lock - order.Quantity
			p.assets[quote].Free = p.assets[quote].Free + order.Quantity*orderPrice
		}
	}

	if candle.Complete {
		var total float64
		for asset, info := range p.assets {
			amount := info.Free + info.Lock
			pair := strings.ToUpper(asset + p.baseCoin)
			total += amount * p.lastCandle[pair].Close
			p.assetValues[asset] = append(p.assetValues[asset], AssetValue{
				Time:  candle.Time,
				Value: amount * p.lastCandle[pair].Close,
			})
		}

		baseCoinInfo := p.assets[p.baseCoin]
		p.equityValues = append(p.equityValues, AssetValue{
			Time:  candle.Time,
			Value: total + baseCoinInfo.Lock + baseCoinInfo.Free,
		})
	}
}

func (p *PaperWallet) Account() (model.Account, error) {
	balances := make([]model.Balance, 0)
	for pair, info := range p.assets {
		balances = append(balances, model.Balance{
			Tick: pair,
			Free: info.Free,
			Lock: info.Lock,
		})
	}

	return model.Account{
		Balances: balances,
	}, nil
}

func (p *PaperWallet) Position(pair string) (asset, quote float64, err error) {
	p.Lock()
	defer p.Unlock()

	assetTick, quoteTick := SplitAssetQuote(pair)
	acc, err := p.Account()
	if err != nil {
		return 0, 0, err
	}

	assetBalance, quoteBalance := acc.Balance(assetTick, quoteTick)

	return assetBalance.Free + assetBalance.Lock, quoteBalance.Free + quoteBalance.Lock, nil
}

func (p *PaperWallet) CreateOrderOCO(side model.SideType, pair string,
	size, price, stop, stopLimit float64) ([]model.Order, error) {
	p.Lock()
	defer p.Unlock()

	asset, _ := SplitAssetQuote(pair)

	err := p.lockFunds(asset, size)
	if err != nil {
		return nil, err
	}

	groupID := p.ID()
	limitMaker := model.Order{
		ExchangeID: p.ID(),
		CreatedAt:  p.lastCandle[pair].Time,
		UpdatedAt:  p.lastCandle[pair].Time,
		Pair:       pair,
		Side:       side,
		Type:       model.OrderTypeLimitMaker,
		Status:     model.OrderStatusTypeNew,
		Price:      price,
		Quantity:   size,
		GroupID:    &groupID,
		RefPrice:   p.lastCandle[pair].Close,
	}

	stopOrder := model.Order{
		ExchangeID: p.ID(),
		CreatedAt:  p.lastCandle[pair].Time,
		UpdatedAt:  p.lastCandle[pair].Time,
		Pair:       pair,
		Side:       side,
		Type:       model.OrderTypeStopLoss,
		Status:     model.OrderStatusTypeNew,
		Price:      stopLimit,
		Stop:       &stop,
		Quantity:   size,
		GroupID:    &groupID,
		RefPrice:   p.lastCandle[pair].Close,
	}
	p.orders = append(p.orders, limitMaker, stopOrder)

	return []model.Order{limitMaker, stopOrder}, nil
}

func (p *PaperWallet) CreateOrderLimit(side model.SideType, pair string,
	size float64, limit float64) (model.Order, error) {

	p.Lock()
	defer p.Unlock()

	asset, quote := SplitAssetQuote(pair)
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
		ExchangeID: p.ID(),
		CreatedAt:  p.lastCandle[pair].Time,
		UpdatedAt:  p.lastCandle[pair].Time,
		Pair:       pair,
		Side:       side,
		Type:       model.OrderTypeLimit,
		Status:     model.OrderStatusTypeNew,
		Price:      limit,
		Quantity:   size,
	}
	p.orders = append(p.orders, order)
	return order, nil
}

func (p *PaperWallet) CreateOrderMarket(side model.SideType, pair string, size float64) (model.Order, error) {
	p.Lock()
	defer p.Unlock()

	return p.createOrderMarket(side, pair, size)
}

func (p *PaperWallet) CreateOrderStop(pair string, size float64, limit float64) (model.Order, error) {
	p.Lock()
	defer p.Unlock()

	asset, _ := SplitAssetQuote(pair)
	err := p.lockFunds(asset, size)
	if err != nil {
		return model.Order{}, err
	}

	order := model.Order{
		ExchangeID: p.ID(),
		CreatedAt:  p.lastCandle[pair].Time,
		UpdatedAt:  p.lastCandle[pair].Time,
		Pair:       pair,
		Side:       model.SideTypeSell,
		Type:       model.OrderTypeStopLossLimit,
		Status:     model.OrderStatusTypeNew,
		Price:      limit,
		Stop:       &limit,
		Quantity:   size,
	}
	p.orders = append(p.orders, order)
	return order, nil
}

func (p *PaperWallet) createOrderMarket(side model.SideType, pair string, size float64) (model.Order, error) {
	asset, quote := SplitAssetQuote(pair)
	if side == model.SideTypeSell {
		if value, ok := p.assets[asset]; !ok || value.Free < size {
			return model.Order{}, &OrderError{
				Err:      ErrInsufficientFunds,
				Pair:     pair,
				Quantity: size,
			}
		}
		if _, ok := p.assets[quote]; !ok {
			p.assets[quote] = &assetInfo{}
		}
		p.assets[asset].Free = p.assets[asset].Free - size
		p.assets[quote].Free = p.assets[quote].Free + p.lastCandle[pair].Close*size
	} else {
		if value, ok := p.assets[quote]; !ok || value.Free < size*p.lastCandle[pair].Close {
			return model.Order{}, &OrderError{
				Err:      ErrInsufficientFunds,
				Pair:     pair,
				Quantity: size,
			}
		}
		if _, ok := p.assets[asset]; !ok {
			p.assets[asset] = &assetInfo{}
		}
		actualQty := p.assets[asset].Free + p.assets[asset].Lock
		p.avgPrice[pair] = (p.avgPrice[pair]*actualQty + p.lastCandle[pair].Close*size) / (actualQty + size)
		p.assets[quote].Free = p.assets[quote].Free - (size * p.lastCandle[pair].Close)
		p.assets[asset].Free = p.assets[asset].Free + size
	}

	if _, ok := p.volume[pair]; !ok {
		p.volume[pair] = 0
	}

	p.volume[pair] += p.lastCandle[pair].Close * size

	order := model.Order{
		ExchangeID: p.ID(),
		CreatedAt:  p.lastCandle[pair].Time,
		UpdatedAt:  p.lastCandle[pair].Time,
		Pair:       pair,
		Side:       side,
		Type:       model.OrderTypeMarket,
		Status:     model.OrderStatusTypeFilled,
		Price:      p.lastCandle[pair].Close,
		Quantity:   size,
	}
	p.orders = append(p.orders, order)
	return order, nil
}

func (p *PaperWallet) CreateOrderMarketQuote(side model.SideType, pair string,
	quoteQuantity float64) (model.Order, error) {
	p.Lock()
	defer p.Unlock()

	info := p.AssetsInfo(pair)
	quantity := common.AmountToLotSize(info.StepSize, info.BaseAssetPrecision, quoteQuantity/p.lastCandle[pair].Close)
	return p.createOrderMarket(side, pair, quantity)
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

func (p *PaperWallet) Order(pair string, id int64) (model.Order, error) {
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

func (p *PaperWallet) CandlesSubscription(ctx context.Context, pair, timeframe string) (chan model.Candle, chan error) {
	return p.feeder.CandlesSubscription(ctx, pair, timeframe)
}
