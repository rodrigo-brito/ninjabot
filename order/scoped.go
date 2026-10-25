package order

import (
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/service"
)

// ledger attributes the shared account to one strategy: the cash it was
// allocated, moved by its fills, and the quantity it holds of each pair.
type ledger struct {
	budget   float64
	cash     float64
	fees     float64
	quantity map[string]float64 // by pair, negative when short
}

// SetBudget allocates cash of the shared account to a tag. A strategy whose
// tag has a budget sees that budget, moved by its own fills, as its quote
// balance, and its own holdings as its asset balance.
func (c *Controller) SetBudget(tag string, amount float64) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.ledgers == nil {
		c.ledgers = make(map[string]*ledger)
	}
	c.ledgers[tag] = &ledger{budget: amount, cash: amount, quantity: make(map[string]float64)}
}

// fillPrice is the price an order was filled at.
func fillPrice(order *model.Order) float64 {
	if (order.Type == model.OrderTypeStopLoss || order.Type == model.OrderTypeStopLossLimit) && order.Stop != nil {
		return *order.Stop
	}
	return order.Price
}

// book moves the ledger of the tag of a filled order. The caller holds the
// lock.
func (c *Controller) book(order *model.Order) {
	l, ok := c.ledgers[order.Tag]
	if !ok || order.Tag == "" {
		return
	}
	value := fillPrice(order) * order.Quantity
	if order.Side == model.SideTypeBuy {
		l.cash -= value + order.Fee
		l.quantity[order.Pair] += order.Quantity
	} else {
		l.cash += value - order.Fee
		l.quantity[order.Pair] -= order.Quantity
	}
	l.fees += order.Fee
}

// TagEquity is the value of a tag's share of the account: its cash plus its
// holdings at the last close of every pair.
func (c *Controller) TagEquity(tag string) float64 {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	l, ok := c.ledgers[tag]
	if !ok {
		return 0
	}
	equity := l.cash
	for pair, quantity := range l.quantity {
		equity += quantity * c.lastPrice[pair]
	}
	return equity
}

// TagFees is the total of fees paid by a tag.
func (c *Controller) TagFees(tag string) float64 {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	if l, ok := c.ledgers[tag]; ok {
		return l.fees
	}
	return 0
}

// Scoped returns a broker for one of several strategies sharing the
// account: every order it sends carries the tag, and Position reports the
// tag's own holdings and, with a budget, its own cash. Account reports the
// whole shared account.
func (c *Controller) Scoped(tag string) service.Broker {
	return &scopedBroker{Controller: c, tag: tag}
}

type scopedBroker struct {
	*Controller
	tag string
}

func (b *scopedBroker) Position(pair string) (asset, quote float64, err error) {
	b.mtx.RLock()
	l, ok := b.ledgers[b.tag]
	if ok {
		asset, quote = l.quantity[pair], l.cash
	}
	b.mtx.RUnlock()
	if ok {
		return asset, quote, nil
	}

	// without a budget: the tag's holdings, and the shared quote balance
	_, shared, err := b.exchange.Position(pair)
	if err != nil {
		return 0, 0, err
	}
	b.mtx.RLock()
	defer b.mtx.RUnlock()
	if position, ok := b.position[positionKey(b.tag, pair)]; ok {
		asset = position.Quantity
		if position.Side == model.SideTypeSell {
			asset = -asset
		}
	}
	return asset, shared, nil
}

func (b *scopedBroker) CreateOrderOCO(side model.SideType, pair string, size, price, stop,
	stopLimit float64) ([]model.Order, error) {
	return b.createOrderOCO(b.tag, side, pair, size, price, stop, stopLimit)
}

func (b *scopedBroker) CreateOrderLimit(side model.SideType, pair string, size, limit float64) (model.Order, error) {
	return b.createOrderLimit(b.tag, side, pair, size, limit)
}

func (b *scopedBroker) CreateOrderMarket(side model.SideType, pair string, size float64) (model.Order, error) {
	return b.createOrderMarket(b.tag, side, pair, size)
}

func (b *scopedBroker) CreateOrderMarketQuote(side model.SideType, pair string, quote float64) (model.Order, error) {
	return b.createOrderMarketQuote(b.tag, side, pair, quote)
}

func (b *scopedBroker) CreateOrderStop(pair string, quantity float64, limit float64) (model.Order, error) {
	return b.createOrderStop(b.tag, pair, quantity, limit)
}
