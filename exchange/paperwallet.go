package exchange

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2/common"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/service"
	"github.com/rodrigo-brito/ninjabot/tools/log"
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
	ctx           context.Context
	baseCoin      string
	counter       int64
	takerFee      float64
	makerFee      float64
	feesPaid      float64
	initialValue  float64
	feeder        service.Feeder
	orders        []model.Order
	pendingOrders map[string][]int // pair -> order indices for fast lookup
	assets        map[string]*assetInfo
	avgShortPrice map[string]float64
	avgLongPrice  map[string]float64
	volume        map[string]float64
	lastCandle    map[string]model.Candle
	fistCandle    map[string]model.Candle
	assetValues   map[string][]AssetValue
	equityValues  []AssetValue
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
		ctx:           ctx,
		baseCoin:      baseCoin,
		orders:        make([]model.Order, 0, 100), // Pre-allocate with capacity
		pendingOrders: make(map[string][]int),
		assets:        make(map[string]*assetInfo),
		fistCandle:    make(map[string]model.Candle),
		lastCandle:    make(map[string]model.Candle),
		avgShortPrice: make(map[string]float64),
		avgLongPrice:  make(map[string]float64),
		volume:        make(map[string]float64),
		assetValues:   make(map[string][]AssetValue),
		equityValues:  make([]AssetValue, 0, 1000), // Pre-allocate for equity tracking
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

	fmt.Println("----- FINAL WALLET -----")
	for pair := range p.lastCandle {
		asset, quote := SplitAssetQuote(pair)
		assetInfo, ok := p.assets[asset]
		if !ok {
			continue
		}

		quantity := assetInfo.Free + assetInfo.Lock
		value := quantity * p.lastCandle[pair].Close
		if quantity < 0 {
			totalShort := 2.0*p.avgShortPrice[pair]*quantity - p.lastCandle[pair].Close*quantity
			value = math.Abs(totalShort)
		}
		total += value
		marketChange += (p.lastCandle[pair].Close - p.fistCandle[pair].Close) / p.fistCandle[pair].Close
		fmt.Printf("%.4f %s = %.4f %s\n", quantity, asset, total, quote)
	}

	avgMarketChange := marketChange / float64(len(p.lastCandle))
	baseCoinValue := p.assets[p.baseCoin].Free + p.assets[p.baseCoin].Lock
	profit := total + baseCoinValue - p.initialValue
	fmt.Printf("%.4f %s\n", baseCoinValue, p.baseCoin)
	fmt.Println()
	maxDrawDown, _, _ := p.MaxDrawdown()
	fmt.Println("----- RETURNS -----")
	fmt.Printf("START PORTFOLIO     = %.2f %s\n", p.initialValue, p.baseCoin)
	fmt.Printf("FINAL PORTFOLIO     = %.2f %s\n", total+baseCoinValue, p.baseCoin)
	fmt.Printf("GROSS PROFIT        =  %f %s (%.2f%%)\n", profit+p.feesPaid, p.baseCoin,
		(profit+p.feesPaid)/p.initialValue*100)
	fmt.Printf("TRADING FEES        =  %f %s\n", p.feesPaid, p.baseCoin)
	fmt.Printf("NET PROFIT          =  %f %s (%.2f%%)\n", profit, p.baseCoin, profit/p.initialValue*100)
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
	fmt.Println("-------------------")
}

// feeRate returns the fee charged for an order type. Orders that rest on the
// book and fill at their own price add liquidity and pay the maker fee, the
// ones that cross the spread - market and triggered stop orders - pay the taker
// fee.
func (p *PaperWallet) feeRate(orderType model.OrderType) float64 {
	switch orderType {
	case model.OrderTypeLimit, model.OrderTypeLimitMaker,
		model.OrderTypeTakeProfit, model.OrderTypeTakeProfitLimit:
		return p.makerFee
	default:
		return p.takerFee
	}
}

// chargeFee deducts a fee, in quote currency, from the wallet, registers it in
// the total paid so far and returns it.
func (p *PaperWallet) chargeFee(quote string, fee float64) float64 {
	if fee == 0 {
		return 0
	}

	if _, ok := p.assets[quote]; !ok {
		p.assets[quote] = &assetInfo{}
	}

	p.assets[quote].Free -= fee
	p.feesPaid += fee

	return fee
}

// validateFunds checks whether the wallet can afford the order and, when fill
// is true, moves the balances. Fees are always charged in quote currency: they
// come out of the proceeds when a position is closed, and out of the free
// balance when one is opened.
//
// It returns the traded amount and its fee. The amount may be slightly smaller
// than requested: an order sized with the whole available balance leaves no
// room for its fee, so instead of rejecting it we trim it by the fee. The trim
// is capped at the fee itself, so a genuinely underfunded order still fails.
func (p *PaperWallet) validateFunds(side model.SideType, pair string, amount, value, feeRate float64,
	fill bool) (filled, fee float64, err error) {

	asset, quote := SplitAssetQuote(pair)
	if _, ok := p.assets[asset]; !ok {
		p.assets[asset] = &assetInfo{}
	}

	if _, ok := p.assets[quote]; !ok {
		p.assets[quote] = &assetInfo{}
	}

	funds := p.assets[quote].Free
	if side == model.SideTypeSell {
		if p.assets[asset].Free > 0 {
			funds += p.assets[asset].Free * value
		}

		// A sell that liquidates a long position pays the fee out of the sale
		// proceeds, but a short entry receives nothing to pay it with, so the
		// wallet must hold enough quote to cover it.
		upfrontRate := 0.0
		if amount > math.Max(p.assets[asset].Free, 0) {
			upfrontRate = feeRate
		}

		if funds < amount*value*(1+upfrontRate) {
			trimmed := 0.0
			if fill {
				trimmed = trimByFee(amount, funds/(value*(1+upfrontRate)), feeRate)
			}

			if trimmed == 0 {
				return 0, 0, &OrderError{
					Err:      ErrInsufficientFunds,
					Pair:     pair,
					Quantity: amount,
				}
			}
			amount = trimmed
		}

		lockedAsset := math.Min(math.Max(p.assets[asset].Free, 0), amount) // ignore negative asset amount to lock
		lockedQuote := (amount - lockedAsset) * value

		p.assets[asset].Free -= lockedAsset
		p.assets[quote].Free -= lockedQuote
		if fill {
			p.updateAveragePrice(side, pair, amount, value)
			if lockedQuote > 0 { // entering in short position
				p.assets[asset].Free -= amount
			} else { // liquidating long position
				p.assets[quote].Free += amount * value

			}
			fee = p.chargeFee(quote, amount*value*feeRate)
		} else {
			p.assets[asset].Lock += lockedAsset
			p.assets[quote].Lock += lockedQuote
		}

		log.Debugf("%s -> LOCK = %f / FREE %f", asset, p.assets[asset].Lock, p.assets[asset].Free)
	} else { // SideTypeBuy
		shortPosition := math.Min(p.assets[asset].Free, 0)

		var liquidShortValue float64
		if shortPosition < 0 {
			v := math.Abs(shortPosition)
			liquidShortValue = 2*v*p.avgShortPrice[pair] - v*value // liquid price of short position
			funds += liquidShortValue
		}

		// the quantity covering an open short is paid with its liquidation
		// value, only the remainder consumes quote balance
		if funds < (amount+shortPosition)*value+amount*value*feeRate {
			trimmed := 0.0
			if fill {
				maxAmount := (funds - shortPosition*value) / (value * (1 + feeRate))
				trimmed = trimByFee(amount, maxAmount, feeRate)
			}

			if trimmed == 0 {
				return 0, 0, &OrderError{
					Err:      ErrInsufficientFunds,
					Pair:     pair,
					Quantity: amount,
				}
			}
			amount = trimmed
		}

		lockedAsset := math.Min(-shortPosition, amount) // ignore positive amount to lock
		lockedQuote := (amount-lockedAsset)*value - liquidShortValue

		p.assets[asset].Free += lockedAsset
		p.assets[quote].Free -= lockedQuote

		if fill {
			p.updateAveragePrice(side, pair, amount, value)
			p.assets[asset].Free += amount - lockedAsset
			fee = p.chargeFee(quote, amount*value*feeRate)
		} else {
			p.assets[asset].Lock += lockedAsset
			p.assets[quote].Lock += lockedQuote
		}
		log.Debugf("%s -> LOCK = %f / FREE %f", asset, p.assets[asset].Lock, p.assets[asset].Free)
	}

	return amount, fee, nil
}

// trimByFee reduces amount to maxAmount when the shortfall is no bigger than
// the fee charged over the order, and returns 0 otherwise - the wallet is then
// underfunded for reasons other than the fee, and the order must be rejected.
func trimByFee(amount, maxAmount, feeRate float64) float64 {
	if feeRate <= 0 {
		return 0
	}

	// the epsilon absorbs the rounding noise of a size derived from the exact
	// available balance, which lands within an ulp of the limit
	if maxAmount < amount/(1+feeRate)*(1-1e-9) {
		return 0
	}

	return math.Min(amount, maxAmount)
}

func (p *PaperWallet) updateAveragePrice(side model.SideType, pair string, amount, value float64) {
	actualQty := 0.0
	asset, quote := SplitAssetQuote(pair)

	if p.assets[asset] != nil {
		actualQty = p.assets[asset].Free
	}

	// without previous position
	if actualQty == 0 {
		if side == model.SideTypeBuy {
			p.avgLongPrice[pair] = value
		} else {
			p.avgShortPrice[pair] = value
		}
		return
	}

	// actual long + order buy
	if actualQty > 0 && side == model.SideTypeBuy {
		positionValue := p.avgLongPrice[pair] * actualQty
		p.avgLongPrice[pair] = (positionValue + amount*value) / (actualQty + amount)
		return
	}

	// actual long + order sell
	if actualQty > 0 && side == model.SideTypeSell {
		profitValue := amount*value - math.Min(amount, actualQty)*p.avgLongPrice[pair]
		percentage := profitValue / (amount * p.avgLongPrice[pair])
		log.Infof("PROFIT = %.4f %s (%.2f %%)", profitValue, quote, percentage*100.0) // TODO: store profits

		if amount <= actualQty { // not enough quantity to close the position
			return
		}

		p.avgShortPrice[pair] = value

		return
	}

	// actual short + order sell
	if actualQty < 0 && side == model.SideTypeSell {
		positionValue := p.avgShortPrice[pair] * -actualQty
		p.avgShortPrice[pair] = (positionValue + amount*value) / (-actualQty + amount)

		return
	}

	// actual short + order buy
	if actualQty < 0 && side == model.SideTypeBuy {
		profitValue := math.Min(amount, -actualQty)*p.avgShortPrice[pair] - amount*value
		percentage := profitValue / (amount * p.avgShortPrice[pair])
		log.Infof("PROFIT = %.4f %s (%.2f %%)", profitValue, quote, percentage*100.0) // TODO: store profits

		if amount <= -actualQty { // not enough quantity to close the position
			return
		}

		p.avgLongPrice[pair] = value
	}
}

func (p *PaperWallet) OnCandle(candle model.Candle) {
	p.Lock()
	defer p.Unlock()

	p.lastCandle[candle.Pair] = candle
	if _, ok := p.fistCandle[candle.Pair]; !ok {
		p.fistCandle[candle.Pair] = candle
	}

	// Use indexed lookup for pending orders - O(1) instead of O(n)
	pendingIndices, hasPending := p.pendingOrders[candle.Pair]
	if !hasPending {
		// Fast path: no pending orders for this pair
		p.updateEquityValues(candle)
		return
	}

	if _, ok := p.volume[candle.Pair]; !ok {
		p.volume[candle.Pair] = 0
	}

	// Process only pending orders for this pair
	completedIndices := make([]int, 0, len(pendingIndices))
	for _, i := range pendingIndices {
		order := p.orders[i]
		if order.Status != model.OrderStatusTypeNew {
			completedIndices = append(completedIndices, i)
			continue
		}

		asset, quote := SplitAssetQuote(order.Pair)
		if order.Side == model.SideTypeBuy && order.Price >= candle.Close {
			if _, ok := p.assets[asset]; !ok {
				p.assets[asset] = &assetInfo{}
			}

			fee := order.Quantity * order.Price * p.feeRate(order.Type)

			p.volume[candle.Pair] += order.Price * order.Quantity
			p.orders[i].UpdatedAt = candle.Time
			p.orders[i].Status = model.OrderStatusTypeFilled
			p.orders[i].Fee = fee
			completedIndices = append(completedIndices, i)

			// update assets size
			p.updateAveragePrice(order.Side, order.Pair, order.Quantity, order.Price)
			p.assets[asset].Free = p.assets[asset].Free + order.Quantity
			p.assets[quote].Lock = p.assets[quote].Lock - order.Price*order.Quantity
			p.chargeFee(quote, fee)
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
			fee := orderVolume * p.feeRate(order.Type)

			p.volume[candle.Pair] += orderVolume
			p.orders[i].UpdatedAt = candle.Time
			p.orders[i].Status = model.OrderStatusTypeFilled
			p.orders[i].Fee = fee
			completedIndices = append(completedIndices, i)

			// update assets size
			p.updateAveragePrice(order.Side, order.Pair, order.Quantity, orderPrice)
			p.assets[asset].Lock = p.assets[asset].Lock - order.Quantity
			p.assets[quote].Free = p.assets[quote].Free + orderVolume
			p.chargeFee(quote, fee)
		}
	}

	// Remove completed orders from pending index
	if len(completedIndices) > 0 {
		p.removeCompletedOrders(candle.Pair, completedIndices)
	}

	p.updateEquityValues(candle)
}

// removeCompletedOrders removes completed order indices from the pending map
func (p *PaperWallet) removeCompletedOrders(pair string, completedIndices []int) {
	if len(completedIndices) == 0 {
		return
	}

	pending := p.pendingOrders[pair]
	newPending := make([]int, 0, len(pending)-len(completedIndices))
	completedMap := make(map[int]bool, len(completedIndices))
	for _, idx := range completedIndices {
		completedMap[idx] = true
	}

	for _, idx := range pending {
		if !completedMap[idx] {
			newPending = append(newPending, idx)
		}
	}

	if len(newPending) == 0 {
		delete(p.pendingOrders, pair)
	} else {
		p.pendingOrders[pair] = newPending
	}
}

// updateEquityValues calculates and stores equity values for complete candles
func (p *PaperWallet) updateEquityValues(candle model.Candle) {
	if !candle.Complete {
		return
	}

	if candle.Complete {
		var total float64
		for asset, info := range p.assets {
			amount := info.Free + info.Lock
			pair := strings.ToUpper(asset + p.baseCoin)
			if amount < 0 {
				v := math.Abs(amount)
				liquid := 2*v*p.avgShortPrice[pair] - v*p.lastCandle[pair].Close
				total += liquid
			} else {
				total += amount * p.lastCandle[pair].Close
			}

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
			Asset: pair,
			Free:  info.Free,
			Lock:  info.Lock,
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

	if size == 0 {
		return nil, ErrInvalidQuantity
	}

	_, _, err := p.validateFunds(side, pair, size, price, p.feeRate(model.OrderTypeLimitMaker), false)
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
	// Add orders and update pending index
	orderIdx := len(p.orders)
	p.orders = append(p.orders, limitMaker, stopOrder)
	p.pendingOrders[pair] = append(p.pendingOrders[pair], orderIdx, orderIdx+1)

	return []model.Order{limitMaker, stopOrder}, nil
}

func (p *PaperWallet) CreateOrderLimit(side model.SideType, pair string,
	size float64, limit float64) (model.Order, error) {

	p.Lock()
	defer p.Unlock()

	if size == 0 {
		return model.Order{}, ErrInvalidQuantity
	}

	_, _, err := p.validateFunds(side, pair, size, limit, p.feeRate(model.OrderTypeLimit), false)
	if err != nil {
		return model.Order{}, err
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
	// Add order and update pending index
	orderIdx := len(p.orders)
	p.orders = append(p.orders, order)
	p.pendingOrders[pair] = append(p.pendingOrders[pair], orderIdx)
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

	if size == 0 {
		return model.Order{}, ErrInvalidQuantity
	}

	_, _, err := p.validateFunds(model.SideTypeSell, pair, size, limit,
		p.feeRate(model.OrderTypeStopLossLimit), false)
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
	// Add order and update pending index
	orderIdx := len(p.orders)
	p.orders = append(p.orders, order)
	p.pendingOrders[pair] = append(p.pendingOrders[pair], orderIdx)
	return order, nil
}

func (p *PaperWallet) createOrderMarket(side model.SideType, pair string, size float64) (model.Order, error) {
	if size == 0 {
		return model.Order{}, ErrInvalidQuantity
	}

	price := p.lastCandle[pair].Close

	// the wallet may fill less than requested to make room for the fee
	filled, fee, err := p.validateFunds(side, pair, size, price, p.feeRate(model.OrderTypeMarket), true)
	if err != nil {
		return model.Order{}, err
	}

	if _, ok := p.volume[pair]; !ok {
		p.volume[pair] = 0
	}

	p.volume[pair] += price * filled

	order := model.Order{
		ExchangeID: p.ID(),
		CreatedAt:  p.lastCandle[pair].Time,
		UpdatedAt:  p.lastCandle[pair].Time,
		Pair:       pair,
		Side:       side,
		Type:       model.OrderTypeMarket,
		Status:     model.OrderStatusTypeFilled,
		Price:      price,
		Quantity:   filled,
		Fee:        fee,
	}

	p.orders = append(p.orders, order)

	return order, nil
}

func (p *PaperWallet) CreateOrderMarketQuote(side model.SideType, pair string,
	quoteQuantity float64) (model.Order, error) {
	p.Lock()
	defer p.Unlock()

	info := p.AssetsInfo(pair)
	amountStr := strconv.FormatFloat(quoteQuantity/p.lastCandle[pair].Close, 'f', -1, 64)
	minQuantityStr := strconv.FormatFloat(info.MinQuantity, 'f', -1, 64)
	stepSizeStr := strconv.FormatFloat(info.StepSize, 'f', -1, 64)
	quantity := common.AmountToLotSize(amountStr, minQuantityStr, stepSizeStr, info.BaseAssetPrecision)
	quantityFloat, err := strconv.ParseFloat(quantity, 64)
	if err != nil {
		return model.Order{}, err
	}
	return p.createOrderMarket(side, pair, quantityFloat)
}

func (p *PaperWallet) Cancel(order model.Order) error {
	p.Lock()
	defer p.Unlock()

	for i, o := range p.orders {
		if o.ExchangeID == order.ExchangeID {
			p.orders[i].Status = model.OrderStatusTypeCanceled

			// unlock funds
			assset, quote := SplitAssetQuote(o.Pair)
			// we have open long position
			if p.assets[assset].Lock > 0 && o.Side == model.SideTypeSell {
				p.assets[assset].Free += o.Quantity
				p.assets[assset].Lock -= o.Quantity
			} else {
				// we don't have open long position
				if p.assets[assset].Lock == 0 {
					amount := order.Price * order.Quantity
					p.assets[quote].Free += amount
					p.assets[quote].Lock -= amount
				}
			}
		}
	}
	return nil
}

func (p *PaperWallet) Order(_ string, id int64) (model.Order, error) {
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
