package order

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/service"
	"github.com/rodrigo-brito/ninjabot/storage"

	"github.com/olekukonko/tablewriter"
	log "github.com/sirupsen/logrus"
)

type summary struct {
	Pair   string
	Win    []float64
	Lose   []float64
	Volume float64
}

func (s summary) Profit() float64 {
	profit := 0.0
	for _, value := range append(s.Win, s.Lose...) {
		profit += value
	}
	return profit
}

func (s summary) SQN() float64 {
	total := float64(len(s.Win) + len(s.Lose))
	avgProfit := s.Profit() / total
	stdDev := 0.0
	for _, profit := range append(s.Win, s.Lose...) {
		stdDev += math.Pow(profit-avgProfit, 2)
	}
	stdDev = math.Sqrt(stdDev / total)
	return math.Sqrt(total) * (s.Profit() / total) / stdDev
}

func (s summary) Payoff() float64 {
	avgWin := 0.0
	avgLose := 0.0

	for _, value := range s.Win {
		avgWin += value
	}

	for _, value := range s.Lose {
		avgLose += value
	}

	if len(s.Win) == 0 || len(s.Lose) == 0 || avgLose == 0 {
		return 0
	}

	return (avgWin / float64(len(s.Win))) / math.Abs(avgLose/float64(len(s.Lose)))
}

func (s summary) WinPercentage() float64 {
	if len(s.Win)+len(s.Lose) == 0 {
		return 0
	}

	return float64(len(s.Win)) / float64(len(s.Win)+len(s.Lose)) * 100
}

func (s summary) String() string {
	tableString := &strings.Builder{}
	table := tablewriter.NewWriter(tableString)
	_, quote := exchange.SplitAssetQuote(s.Pair)
	data := [][]string{
		{"Coin", s.Pair},
		{"Trades", strconv.Itoa(len(s.Lose) + len(s.Win))},
		{"Win", strconv.Itoa(len(s.Win))},
		{"Loss", strconv.Itoa(len(s.Lose))},
		{"% Win", fmt.Sprintf("%.1f", s.WinPercentage())},
		{"Payoff", fmt.Sprintf("%.1f", s.Payoff()*100)},
		{"Profit", fmt.Sprintf("%.4f %s", s.Profit(), quote)},
		{"Volume", fmt.Sprintf("%.4f %s", s.Volume, quote)},
	}
	table.AppendBulk(data)
	table.SetColumnAlignment([]int{tablewriter.ALIGN_LEFT, tablewriter.ALIGN_RIGHT})
	table.Render()
	return tableString.String()
}

type Status string

const (
	StatusRunning Status = "running"
	StatusStopped Status = "stopped"
	StatusError   Status = "error"
)

type Controller struct {
	mtx            sync.Mutex
	ctx            context.Context
	exchange       service.Exchange
	storage        storage.Storage
	orderFeed      *Feed
	notifier       service.Notifier
	Results        map[string]*summary
	lastPrice      map[string]float64
	tickerInterval time.Duration
	finish         chan bool
	status         Status
}

func NewController(ctx context.Context, exchange service.Exchange, storage storage.Storage,
	orderFeed *Feed) *Controller {

	return &Controller{
		ctx:            ctx,
		storage:        storage,
		exchange:       exchange,
		orderFeed:      orderFeed,
		lastPrice:      make(map[string]float64),
		Results:        make(map[string]*summary),
		tickerInterval: time.Second,
		finish:         make(chan bool),
	}
}

func (c *Controller) SetNotifier(notifier service.Notifier) {
	c.notifier = notifier
}

func (c *Controller) OnCandle(candle model.Candle) {
	c.lastPrice[candle.Pair] = candle.Close
}

func (c *Controller) calculateProfit(o *model.Order) (value, percent float64, err error) {
	// get filled orders before the current order
	orders, err := c.storage.Orders(
		storage.WithUpdateAtBeforeOrEqual(o.UpdatedAt),
		storage.WithStatus(model.OrderStatusTypeFilled),
		storage.WithPair(o.Pair),
	)
	if err != nil {
		return 0, 0, err
	}

	quantity := 0.0
	avgPrice := 0.0

	for _, order := range orders {
		// skip current order
		if o.ID == order.ID {
			continue
		}

		// calculate avg price
		if order.Side == model.SideTypeBuy {
			price := order.Price
			if order.Type == model.OrderTypeStopLoss || order.Type == model.OrderTypeStopLossLimit {
				price = *order.Stop
			}
			avgPrice = (order.Quantity*price + avgPrice*quantity) / (order.Quantity + quantity)
			quantity += order.Quantity
		} else {
			quantity = math.Max(quantity-order.Quantity, 0)
		}
	}

	if quantity == 0 {
		return 0, 0, nil
	}

	cost := o.Quantity * avgPrice
	price := o.Price
	if o.Type == model.OrderTypeStopLoss || o.Type == model.OrderTypeStopLossLimit {
		price = *o.Stop
	}
	profitValue := o.Quantity*price - cost
	return profitValue, profitValue / cost, nil
}

func (c *Controller) notify(message string) {
	log.Info(message)
	if c.notifier != nil {
		c.notifier.Notify(message)
	}
}

func (c *Controller) notifyError(err error) {
	log.Error(err)
	if c.notifier != nil {
		c.notifier.OnError(err)
	}
}

func (c *Controller) processTrade(order *model.Order) {
	if order.Status != model.OrderStatusTypeFilled {
		return
	}

	// initializer results map if needed
	if _, ok := c.Results[order.Pair]; !ok {
		c.Results[order.Pair] = &summary{Pair: order.Pair}
	}

	// register order volume
	c.Results[order.Pair].Volume += order.Price * order.Quantity

	// calculate profit only to sell orders
	if order.Side != model.SideTypeSell {
		return
	}

	profitValue, profit, err := c.calculateProfit(order)
	if err != nil {
		c.notifyError(err)
		return
	}

	order.Profit = profit
	if profitValue >= 0 {
		c.Results[order.Pair].Win = append(c.Results[order.Pair].Win, profitValue)
	} else {
		c.Results[order.Pair].Lose = append(c.Results[order.Pair].Lose, profitValue)
	}

	_, quote := exchange.SplitAssetQuote(order.Pair)
	c.notify(fmt.Sprintf("[PROFIT] %f %s (%f %%)\n`%s`", profitValue, quote, profit*100, c.Results[order.Pair].String()))
}

func (c *Controller) updateOrders() {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	// pending orders
	orders, err := c.storage.Orders(storage.WithStatusIn(
		model.OrderStatusTypeNew,
		model.OrderStatusTypePartiallyFilled,
		model.OrderStatusTypePendingCancel,
	))
	if err != nil {
		c.notifyError(err)
		c.mtx.Unlock()
		return
	}

	// For each pending order, check for updates
	var updatedOrders []model.Order
	for _, order := range orders {
		excOrder, err := c.exchange.Order(order.Pair, order.ExchangeID)
		if err != nil {
			log.WithField("id", order.ExchangeID).Error("orderControler/get: ", err)
			continue
		}

		// no status change
		if excOrder.Status == order.Status {
			continue
		}

		excOrder.ID = order.ID
		err = c.storage.UpdateOrder(&excOrder)
		if err != nil {
			c.notifyError(err)
			continue
		}

		log.Infof("[ORDER %s] %s", excOrder.Status, excOrder)
		updatedOrders = append(updatedOrders, excOrder)
	}

	for _, processOrder := range updatedOrders {
		c.processTrade(&processOrder)
		c.orderFeed.Publish(processOrder, false)
	}
}

func (c *Controller) Status() Status {
	return c.status
}

func (c *Controller) Start() {
	if c.status != StatusRunning {
		c.status = StatusRunning
		go func() {
			ticker := time.NewTicker(c.tickerInterval)
			for {
				select {
				case <-ticker.C:
					c.updateOrders()
				case <-c.finish:
					ticker.Stop()
					return
				}
			}
		}()
		log.Info("Bot started.")
	}
}

func (c *Controller) Stop() {
	if c.status == StatusRunning {
		c.status = StatusStopped
		c.updateOrders()
		c.finish <- true
		log.Info("Bot stopped.")
	}
}

func (c *Controller) Account() (model.Account, error) {
	return c.exchange.Account()
}

func (c *Controller) Position(pair string) (asset, quote float64, err error) {
	return c.exchange.Position(pair)
}

func (c *Controller) LastQuote(pair string) (float64, error) {
	return c.exchange.LastQuote(c.ctx, pair)
}

func (c *Controller) PositionValue(pair string) (float64, error) {
	asset, _, err := c.exchange.Position(pair)
	if err != nil {
		return 0, err
	}
	return asset * c.lastPrice[pair], nil
}

func (c *Controller) Order(pair string, id int64) (model.Order, error) {
	return c.exchange.Order(pair, id)
}

func (c *Controller) CreateOrderOCO(side model.SideType, pair string, size, price, stop,
	stopLimit float64) ([]model.Order, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	log.Infof("[ORDER] Creating OCO order for %s", pair)
	orders, err := c.exchange.CreateOrderOCO(side, pair, size, price, stop, stopLimit)
	if err != nil {
		c.notifyError(err)
		return nil, err
	}

	for i := range orders {
		err := c.storage.CreateOrder(&orders[i])
		if err != nil {
			c.notifyError(err)
			return nil, err
		}
		go c.orderFeed.Publish(orders[i], true)
	}

	return orders, nil
}

func (c *Controller) CreateOrderLimit(side model.SideType, pair string, size, limit float64) (model.Order, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	log.Infof("[ORDER] Creating LIMIT %s order for %s", side, pair)
	order, err := c.exchange.CreateOrderLimit(side, pair, size, limit)
	if err != nil {
		c.notifyError(err)
		return model.Order{}, err
	}

	err = c.storage.CreateOrder(&order)
	if err != nil {
		c.notifyError(err)
		return model.Order{}, err
	}
	go c.orderFeed.Publish(order, true)
	log.Infof("[ORDER CREATED] %s", order)
	return order, nil
}

func (c *Controller) CreateOrderMarketQuote(side model.SideType, pair string, amount float64) (model.Order, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	log.Infof("[ORDER] Creating MARKET %s order for %s", side, pair)
	order, err := c.exchange.CreateOrderMarketQuote(side, pair, amount)
	if err != nil {
		c.notifyError(err)
		return model.Order{}, err
	}

	err = c.storage.CreateOrder(&order)
	if err != nil {
		c.notifyError(err)
		return model.Order{}, err
	}

	// calculate profit
	c.processTrade(&order)
	go c.orderFeed.Publish(order, true)
	log.Infof("[ORDER CREATED] %s", order)
	return order, err
}

func (c *Controller) CreateOrderMarket(side model.SideType, pair string, size float64) (model.Order, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	log.Infof("[ORDER] Creating MARKET %s order for %s", side, pair)
	order, err := c.exchange.CreateOrderMarket(side, pair, size)
	if err != nil {
		c.notifyError(err)
		return model.Order{}, err
	}

	err = c.storage.CreateOrder(&order)
	if err != nil {
		c.notifyError(err)
		return model.Order{}, err
	}

	// calculate profit
	c.processTrade(&order)
	go c.orderFeed.Publish(order, true)
	log.Infof("[ORDER CREATED] %s", order)
	return order, err
}

func (c *Controller) CreateOrderStop(pair string, size float64, limit float64) (model.Order, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	log.Infof("[ORDER] Creating STOP order for %s", pair)
	order, err := c.exchange.CreateOrderStop(pair, size, limit)
	if err != nil {
		c.notifyError(err)
		return model.Order{}, err
	}

	err = c.storage.CreateOrder(&order)
	if err != nil {
		c.notifyError(err)
		return model.Order{}, err
	}
	go c.orderFeed.Publish(order, true)
	log.Infof("[ORDER CREATED] %s", order)
	return order, nil
}

func (c *Controller) Cancel(order model.Order) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	log.Infof("[ORDER] Cancelling order for %s", order.Pair)
	err := c.exchange.Cancel(order)
	if err != nil {
		return err
	}

	order.Status = model.OrderStatusTypePendingCancel
	err = c.storage.UpdateOrder(&order)
	if err != nil {
		c.notifyError(err)
		return err
	}
	log.Infof("[ORDER CANCELED] %s", order)
	return nil
}
