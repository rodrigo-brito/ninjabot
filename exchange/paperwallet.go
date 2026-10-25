package exchange

import (
	"context"
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
	"github.com/rodrigo-brito/ninjabot/tools/metrics"
)

type assetInfo struct {
	Free float64
	Lock float64
}

type AssetValue struct {
	Time  time.Time
	Value float64
}

// orderLock records the balances a pending order moved from Free to Lock, so
// that a fill or a cancel releases exactly what was reserved. OCO legs share
// one lock, keyed by their group id.
type orderLock struct {
	asset float64 // base asset backing a sell that closes a long position
	quote float64 // quote paying a buy, or the collateral of a short entry
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
	ordersByID    map[int64]int        // exchange id -> index in orders
	pendingOrders map[string][]int     // pair -> order indices for fast lookup
	locks         map[int64]*orderLock // order (or OCO group) id -> reserved funds
	assets        map[string]*assetInfo
	avgShortPrice map[string]float64
	avgLongPrice  map[string]float64
	volume        map[string]float64
	lastCandle    map[string]model.Candle
	fistCandle    map[string]model.Candle
	assetValues   map[string][]AssetValue
	equityValues  []AssetValue
	fills         []model.Order // orders whose status changed in OnCandle, drained by TakeFills
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
		ordersByID:    make(map[int64]int),
		pendingOrders: make(map[string][]int),
		locks:         make(map[int64]*orderLock),
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

	if _, ok := wallet.assets[wallet.baseCoin]; !ok {
		wallet.assets[wallet.baseCoin] = &assetInfo{}
	}
	wallet.initialValue = wallet.assets[wallet.baseCoin].Free
	log.Debug("[SETUP] Using paper wallet")
	log.Debugf("[SETUP] Initial Portfolio = %f %s", wallet.initialValue, wallet.baseCoin)

	return &wallet
}

// ID returns a new sequential order ID.
func (p *PaperWallet) ID() int64 {
	p.Lock()
	defer p.Unlock()
	return p.id()
}

// id returns a new sequential order ID. The caller holds the lock.
func (p *PaperWallet) id() int64 {
	p.counter++
	return p.counter
}

func (p *PaperWallet) Pairs() []string {
	p.Lock()
	defer p.Unlock()

	pairs := make([]string, 0, len(p.assets))
	for pair := range p.assets {
		pairs = append(pairs, pair)
	}
	return pairs
}

func (p *PaperWallet) LastQuote(ctx context.Context, pair string) (float64, error) {
	return p.feeder.LastQuote(ctx, pair)
}

// AssetValues returns a copy of the value history, in quote currency, of an
// asset held by the wallet. There is one point per candle timestamp.
func (p *PaperWallet) AssetValues(asset string) []AssetValue {
	p.Lock()
	defer p.Unlock()

	values := make([]AssetValue, len(p.assetValues[asset]))
	copy(values, p.assetValues[asset])
	return values
}

// EquityValues returns a copy of the equity curve of the wallet, in quote
// currency. There is one point per candle timestamp, no matter how many pairs
// are traded.
func (p *PaperWallet) EquityValues() []AssetValue {
	p.Lock()
	defer p.Unlock()

	values := make([]AssetValue, len(p.equityValues))
	copy(values, p.equityValues)
	return values
}

// MaxDrawdown returns the largest peak-to-trough decline of the equity curve,
// as a negative fraction of the peak, with the time of the peak and of the
// trough. It returns zero values when the curve never declines.
func (p *PaperWallet) MaxDrawdown() (float64, time.Time, time.Time) {
	p.Lock()
	defer p.Unlock()

	return maxDrawdown(p.equityValues)
}

func maxDrawdown(values []AssetValue) (float64, time.Time, time.Time) {
	if len(values) == 0 {
		return 0, time.Time{}, time.Time{}
	}

	equity := make([]float64, len(values))
	for i, value := range values {
		equity[i] = value.Value
	}

	worst := metrics.MaxDrawdown(equity)
	if worst == 0 {
		return 0, values[0].Time, values[0].Time
	}

	// locate the peak and the trough of the worst decline
	peak := 0
	var start, end time.Time
	for i := range values {
		if values[i].Value > values[peak].Value {
			peak = i
		}
		if values[peak].Value > 0 && (values[peak].Value-values[i].Value)/values[peak].Value == worst {
			start, end = values[peak].Time, values[i].Time
			break
		}
	}

	return -worst, start, end
}

func (p *PaperWallet) Summary() {
	p.Lock()
	defer p.Unlock()

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
		fmt.Printf("%.4f %s = %.4f %s\n", quantity, asset, value, quote)
	}

	avgMarketChange := marketChange / float64(len(p.lastCandle))
	baseCoinValue := p.assets[p.baseCoin].Free + p.assets[p.baseCoin].Lock
	profit := total + baseCoinValue - p.initialValue
	fmt.Printf("%.4f %s\n", baseCoinValue, p.baseCoin)
	fmt.Println()
	maxDrawDown, _, _ := maxDrawdown(p.equityValues)
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

// ensureAssets creates the balance entries of a pair when missing.
func (p *PaperWallet) ensureAssets(pair string) (asset, quote string) {
	asset, quote = SplitAssetQuote(pair)
	if _, ok := p.assets[asset]; !ok {
		p.assets[asset] = &assetInfo{}
	}
	if _, ok := p.assets[quote]; !ok {
		p.assets[quote] = &assetInfo{}
	}
	return asset, quote
}

// lockKey returns the id under which the reserved funds of an order are
// recorded. OCO legs share one reservation, keyed by the group id.
func lockKey(order model.Order) int64 {
	if order.GroupID != nil {
		return *order.GroupID
	}
	return order.ExchangeID
}

// checkFunds verifies the wallet can afford an order of the given side and
// size at the given price and returns the amount it can fill.
//
// Shorts are modelled as a negative asset balance backed by collateral: a
// short entry takes price*quantity out of the quote balance, and its
// liquidation value is 2*avg*qty - price*qty (collateral plus profit). A buy
// that covers an open short is paid with that value; only the part opening a
// long consumes quote balance. Likewise a sell that closes a long pays its fee
// from the proceeds, but a short entry receives nothing, so the wallet must
// hold enough quote for its collateral and its fee.
//
// When trim is true the amount may come back slightly smaller than requested:
// an order sized with the whole available balance leaves no room for its fee,
// so instead of rejecting it we trim it by the fee. The trim is capped at the
// fee itself, so a genuinely underfunded order still fails.
func (p *PaperWallet) checkFunds(side model.SideType, pair string, amount, price, feeRate float64,
	trim bool) (float64, error) {

	asset, quote := p.ensureAssets(pair)
	free := p.assets[quote].Free

	var need, maxAmount float64
	if side == model.SideTypeSell {
		long := math.Max(p.assets[asset].Free, 0)
		short := amount - closing(long, amount)
		if short > 0 {
			need = short*price + amount*price*feeRate
		}
		maxAmount = (free + long*price) / (price * (1 + feeRate))
	} else {
		short := math.Max(-p.assets[asset].Free, 0)
		cover := closing(short, amount)
		free += cover * (2*p.avgShortPrice[pair] - price) // liquidation of the covered short
		need = (amount-cover)*price + amount*price*feeRate
		maxAmount = (free + short*price) / (price * (1 + feeRate))
	}

	if free >= need {
		return amount, nil
	}

	if trim {
		if trimmed := trimByFee(amount, maxAmount, feeRate); trimmed > 0 {
			return trimmed, nil
		}
	}

	return 0, &OrderError{
		Err:      ErrInsufficientFunds,
		Pair:     pair,
		Quantity: amount,
	}
}

// trimByFee reduces amount to maxAmount when the shortfall is no bigger than
// the fee charged over the order, and returns 0 otherwise - the wallet is then
// underfunded for reasons other than the fee, and the order must be rejected.
func trimByFee(amount, maxAmount, feeRate float64) float64 {
	// an order sized with the exact balance can exceed it by float rounding,
	// with or without a fee
	if maxAmount >= amount*(1-1e-9) {
		return math.Min(amount, maxAmount)
	}
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

// dust is the balance below which an amount is float rounding noise. It is
// well below the step size of the paper wallet (1e-8): no order can trade it.
const dust = 1e-9

// closing returns the part of an order of the given amount that closes an
// open position of size held (the long for a sell, the short for a buy). An
// order matching the position give or take float rounding closes all of it:
// it is a liquidation, not an entry on the other side for the residue.
func closing(held, amount float64) float64 {
	closed := math.Min(math.Max(held, 0), amount)
	if amount-closed < dust {
		closed = amount
	}
	return closed
}

// sweepDust zeroes the balances of the given assets that are rounding noise.
// Without it, trimming an order by its fee can leave a residue such as
// 7e-15 ETH and -1e-12 USDT: the strategy sees an open position it cannot
// sell, and every attempt fails for insufficient funds.
func (p *PaperWallet) sweepDust(assets ...string) {
	for _, asset := range assets {
		info, ok := p.assets[asset]
		if !ok {
			continue
		}
		if math.Abs(info.Free) < dust {
			info.Free = 0
		}
		if math.Abs(info.Lock) < dust {
			info.Lock = 0
		}
	}
}

// reserve moves the funds backing a pending order from Free to Lock and
// records them under key: the asset sold out of a long position, and the quote
// paying a buy or collateralizing a short entry. The share of a buy that covers
// an open short is paid by the short liquidation on fill, so nothing is
// reserved for it.
func (p *PaperWallet) reserve(key int64, side model.SideType, pair string, amount, price float64) {
	asset, quote := p.ensureAssets(pair)

	lock := &orderLock{}
	if side == model.SideTypeSell {
		lock.asset = math.Min(math.Max(p.assets[asset].Free, 0), amount)
		lock.quote = (amount - closing(p.assets[asset].Free, amount)) * price
	} else {
		lock.quote = (amount - closing(-p.assets[asset].Free, amount)) * price
	}

	p.assets[asset].Free -= lock.asset
	p.assets[asset].Lock += lock.asset
	p.assets[quote].Free -= lock.quote
	p.assets[quote].Lock += lock.quote
	p.locks[key] = lock

	log.Debugf("%s -> LOCK = %f / FREE %f", asset, p.assets[asset].Lock, p.assets[asset].Free)
}

// release gives the funds reserved under key back to the free balance. It is
// a no-op when nothing is reserved, so cancelling twice is harmless.
func (p *PaperWallet) release(key int64, pair string) {
	lock, ok := p.locks[key]
	if !ok {
		return
	}
	delete(p.locks, key)

	asset, quote := p.ensureAssets(pair)
	p.assets[asset].Lock -= lock.asset
	p.assets[asset].Free += lock.asset
	p.assets[quote].Lock -= lock.quote
	p.assets[quote].Free += lock.quote
	p.sweepDust(asset, quote)
}

// settle applies a fill to the free balances and charges its fee, which is
// returned. The caller must have checked the funds (and released any
// reservation of the order) beforehand.
func (p *PaperWallet) settle(side model.SideType, pair string, amount, price, feeRate float64) float64 {
	asset, quote := p.ensureAssets(pair)

	// the average price must be updated before the balance moves, it reads the
	// current position from it
	p.updateAveragePrice(side, pair, amount, price)

	if side == model.SideTypeSell {
		long := closing(p.assets[asset].Free, amount) // closes a long
		short := amount - long                        // opens a short
		p.assets[asset].Free -= amount
		p.assets[quote].Free += long*price - short*price // proceeds minus collateral
	} else {
		cover := closing(-p.assets[asset].Free, amount) // closes a short
		open := amount - cover                          // opens a long
		p.assets[asset].Free += amount
		p.assets[quote].Free += cover*(2*p.avgShortPrice[pair]-price) - open*price
	}

	log.Debugf("%s -> LOCK = %f / FREE %f", asset, p.assets[asset].Lock, p.assets[asset].Free)

	fee := p.chargeFee(quote, amount*price*feeRate)
	p.sweepDust(asset, quote)
	return fee
}

// lockOrder checks the funds of a pending order and reserves them under key.
func (p *PaperWallet) lockOrder(key int64, side model.SideType, pair string, amount, price, feeRate float64) error {
	if _, err := p.checkFunds(side, pair, amount, price, feeRate, false); err != nil {
		return err
	}
	p.reserve(key, side, pair, amount, price)
	return nil
}

// fillOrder checks the funds of an immediate fill and settles it, returning
// the filled amount (see checkFunds for why it may be trimmed) and the fee.
func (p *PaperWallet) fillOrder(side model.SideType, pair string, amount, price, feeRate float64) (float64, float64, error) {
	filled, err := p.checkFunds(side, pair, amount, price, feeRate, true)
	if err != nil {
		return 0, 0, err
	}
	return filled, p.settle(side, pair, filled, price, feeRate), nil
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
		closed := math.Min(amount, actualQty)
		profitValue := closed*value - closed*p.avgLongPrice[pair]
		percentage := profitValue / (closed * p.avgLongPrice[pair])
		log.Debugf("PROFIT = %.4f %s (%.2f %%)", profitValue, quote, percentage*100.0)

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
		closed := math.Min(amount, -actualQty)
		profitValue := closed*p.avgShortPrice[pair] - closed*value
		percentage := profitValue / (closed * p.avgShortPrice[pair])
		log.Debugf("PROFIT = %.4f %s (%.2f %%)", profitValue, quote, percentage*100.0)

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

		price, ok := fillPrice(order, candle)
		if !ok {
			continue
		}

		// Cancel other orders from same group
		if order.GroupID != nil {
			for j, groupOrder := range p.orders {
				if groupOrder.GroupID != nil && *groupOrder.GroupID == *order.GroupID &&
					groupOrder.ExchangeID != order.ExchangeID &&
					groupOrder.Status == model.OrderStatusTypeNew {
					p.orders[j].Status = model.OrderStatusTypeCanceled
					p.orders[j].UpdatedAt = candle.Time
					p.fills = append(p.fills, p.orders[j])
				}
			}
		}

		// give the reservation back and settle the fill as a trade at the
		// fill price, exactly like a market order
		p.release(lockKey(order), order.Pair)
		fee := p.settle(order.Side, order.Pair, order.Quantity, price, p.feeRate(order.Type))

		p.volume[candle.Pair] += order.Quantity * price
		p.orders[i].UpdatedAt = candle.Time
		p.orders[i].Status = model.OrderStatusTypeFilled
		p.orders[i].Price = price
		p.orders[i].Fee = fee
		completedIndices = append(completedIndices, i)
		p.fills = append(p.fills, p.orders[i])
	}

	// Remove completed orders from pending index
	if len(completedIndices) > 0 {
		p.removeCompletedOrders(candle.Pair, completedIndices)
	}

	p.updateEquityValues(candle)
}

// fillPrice reports whether a pending order is filled by the candle and at
// which price. Limit orders fill at their own price once the candle trades
// through it; stop orders fill at their stop price once it is touched. A buy
// limit rests below the market and a buy stop above it, the sell side is the
// mirror image.
func fillPrice(order model.Order, candle model.Candle) (float64, bool) {
	switch order.Type {
	case model.OrderTypeLimit, model.OrderTypeLimitMaker,
		model.OrderTypeTakeProfit, model.OrderTypeTakeProfitLimit:
		if order.Side == model.SideTypeBuy && candle.Low <= order.Price ||
			order.Side == model.SideTypeSell && candle.High >= order.Price {
			return order.Price, true
		}
	case model.OrderTypeStopLoss, model.OrderTypeStopLossLimit:
		if order.Stop == nil {
			return 0, false
		}
		if order.Side == model.SideTypeBuy && candle.High >= *order.Stop ||
			order.Side == model.SideTypeSell && candle.Low <= *order.Stop {
			return *order.Stop, true
		}
	}
	return 0, false
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

// InitialValue returns the initial balance of the base coin.
func (p *PaperWallet) InitialValue() float64 {
	p.Lock()
	defer p.Unlock()
	return p.initialValue
}

// FeesPaid returns the total of fees charged so far, in quote currency.
func (p *PaperWallet) FeesPaid() float64 {
	p.Lock()
	defer p.Unlock()
	return p.feesPaid
}

// Volume returns a copy of the traded volume per pair, in quote currency.
func (p *PaperWallet) Volume() map[string]float64 {
	p.Lock()
	defer p.Unlock()

	volume := make(map[string]float64, len(p.volume))
	for pair, value := range p.volume {
		volume[pair] = value
	}
	return volume
}

// TakeFills returns the orders filled or canceled by the wallet since the last
// call, in the order the changes happened, and clears the list. A backtest
// engine uses it to sync the order controller only when something changed.
func (p *PaperWallet) TakeFills() []model.Order {
	p.Lock()
	defer p.Unlock()

	fills := p.fills
	p.fills = nil
	return fills
}

// Equity returns the current value of the wallet in quote currency: the quote
// balance plus every asset priced at the close of its last candle. Short
// positions are valued at their liquidation price.
func (p *PaperWallet) Equity() float64 {
	p.Lock()
	defer p.Unlock()

	return p.equity()
}

// equity is Equity without the lock. The caller holds it.
func (p *PaperWallet) equity() float64 {
	var total float64
	for asset, info := range p.assets {
		if asset == p.baseCoin {
			continue
		}
		total += p.assetValue(asset, info)
	}

	if baseCoinInfo, ok := p.assets[p.baseCoin]; ok {
		total += baseCoinInfo.Lock + baseCoinInfo.Free
	}
	return total
}

// assetValue is the liquidation value of an asset in quote currency.
func (p *PaperWallet) assetValue(asset string, info *assetInfo) float64 {
	amount := info.Free + info.Lock
	pair := strings.ToUpper(asset + p.baseCoin)
	if amount < 0 {
		v := math.Abs(amount)
		return 2*v*p.avgShortPrice[pair] - v*p.lastCandle[pair].Close
	}
	return amount * p.lastCandle[pair].Close
}

// updateEquityValues records the equity and asset values for a complete
// candle. Candles of different pairs share timestamps, so a point with the
// same time as the last one replaces it: the curves keep one point per
// timestamp, priced with the latest candles of every pair.
func (p *PaperWallet) updateEquityValues(candle model.Candle) {
	if !candle.Complete {
		return
	}

	for asset, info := range p.assets {
		if asset == p.baseCoin {
			continue
		}
		pair := strings.ToUpper(asset + p.baseCoin)
		p.assetValues[asset] = appendOrReplace(p.assetValues[asset], AssetValue{
			Time:  candle.Time,
			Value: (info.Free + info.Lock) * p.lastCandle[pair].Close,
		})
	}

	p.equityValues = appendOrReplace(p.equityValues, AssetValue{
		Time:  candle.Time,
		Value: p.equity(),
	})
}

// appendOrReplace appends value to values, or replaces the last point when it
// has the same time.
func appendOrReplace(values []AssetValue, value AssetValue) []AssetValue {
	if n := len(values); n > 0 && values[n-1].Time.Equal(value.Time) {
		values[n-1] = value
		return values
	}
	return append(values, value)
}

func (p *PaperWallet) Account() (model.Account, error) {
	p.Lock()
	defer p.Unlock()

	return p.account()
}

// account is Account without the lock. The caller holds it.
func (p *PaperWallet) account() (model.Account, error) {
	balances := make([]model.Balance, 0, len(p.assets))
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
	acc, err := p.account()
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

	// both legs share one reservation, released when either fills or cancels
	groupID := p.id()
	err := p.lockOrder(groupID, side, pair, size, price, p.feeRate(model.OrderTypeLimitMaker))
	if err != nil {
		return nil, err
	}

	limitMaker := model.Order{
		ExchangeID: p.id(),
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
		ExchangeID: p.id(),
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
	p.addOrder(limitMaker, true)
	p.addOrder(stopOrder, true)

	return []model.Order{limitMaker, stopOrder}, nil
}

func (p *PaperWallet) CreateOrderLimit(side model.SideType, pair string,
	size float64, limit float64) (model.Order, error) {

	p.Lock()
	defer p.Unlock()

	if size == 0 {
		return model.Order{}, ErrInvalidQuantity
	}

	id := p.id()
	err := p.lockOrder(id, side, pair, size, limit, p.feeRate(model.OrderTypeLimit))
	if err != nil {
		return model.Order{}, err
	}
	order := model.Order{
		ExchangeID: id,
		CreatedAt:  p.lastCandle[pair].Time,
		UpdatedAt:  p.lastCandle[pair].Time,
		Pair:       pair,
		Side:       side,
		Type:       model.OrderTypeLimit,
		Status:     model.OrderStatusTypeNew,
		Price:      limit,
		Quantity:   size,
	}
	p.addOrder(order, true)
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

	id := p.id()
	err := p.lockOrder(id, model.SideTypeSell, pair, size, limit, p.feeRate(model.OrderTypeStopLossLimit))
	if err != nil {
		return model.Order{}, err
	}

	order := model.Order{
		ExchangeID: id,
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
	p.addOrder(order, true)
	return order, nil
}

func (p *PaperWallet) createOrderMarket(side model.SideType, pair string, size float64) (model.Order, error) {
	if size == 0 {
		return model.Order{}, ErrInvalidQuantity
	}

	price := p.lastCandle[pair].Close

	// the wallet may fill less than requested to make room for the fee
	filled, fee, err := p.fillOrder(side, pair, size, price, p.feeRate(model.OrderTypeMarket))
	if err != nil {
		return model.Order{}, err
	}

	if _, ok := p.volume[pair]; !ok {
		p.volume[pair] = 0
	}

	p.volume[pair] += price * filled

	order := model.Order{
		ExchangeID: p.id(),
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

	p.addOrder(order, false)

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

// Cancel cancels an open order and releases its reserved funds. Cancelling a
// leg of an OCO cancels the whole group, as exchanges do. It fails with
// ErrOrderNotOpen for orders already filled or canceled, so their balances are
// never touched twice.
func (p *PaperWallet) Cancel(order model.Order) error {
	p.Lock()
	defer p.Unlock()

	idx, ok := p.ordersByID[order.ExchangeID]
	if !ok {
		return ErrOrderNotFound
	}

	target := p.orders[idx]
	if target.Status != model.OrderStatusTypeNew {
		return &OrderError{Err: ErrOrderNotOpen, Pair: target.Pair, Quantity: target.Quantity}
	}

	for i, o := range p.orders {
		sameGroup := target.GroupID != nil && o.GroupID != nil && *o.GroupID == *target.GroupID
		if (i == idx || sameGroup) && o.Status == model.OrderStatusTypeNew {
			p.orders[i].Status = model.OrderStatusTypeCanceled
			p.orders[i].UpdatedAt = p.lastCandle[target.Pair].Time
		}
	}

	p.release(lockKey(target), target.Pair)
	return nil
}

// addOrder stores an order and, when it is still open, indexes it for the
// fills of the next candles.
func (p *PaperWallet) addOrder(order model.Order, pending bool) {
	idx := len(p.orders)
	p.orders = append(p.orders, order)
	p.ordersByID[order.ExchangeID] = idx
	if pending {
		p.pendingOrders[order.Pair] = append(p.pendingOrders[order.Pair], idx)
	}
}

func (p *PaperWallet) Order(_ string, id int64) (model.Order, error) {
	p.Lock()
	defer p.Unlock()

	if idx, ok := p.ordersByID[id]; ok {
		return p.orders[idx], nil
	}
	return model.Order{}, ErrOrderNotFound
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
