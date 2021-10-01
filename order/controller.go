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
	Symbol string
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
	_, quote := exchange.SplitAssetQuote(s.Symbol)
	data := [][]string{
		{"Coin", s.Symbol},
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
	tickerInterval time.Duration
	finish         chan bool
	status         Status
}

func NewController(ctx context.Context, exchange service.Exchange, storage storage.Storage, orderFeed *Feed, notifier service.Notifier) *Controller {
	return &Controller{
		ctx:            ctx,
		storage:        storage,
		exchange:       exchange,
		orderFeed:      orderFeed,
		notifier:       notifier,
		Results:        make(map[string]*summary),
		tickerInterval: time.Second,
		finish:         make(chan bool),
	}
}

func (c *Controller) calculateProfit(o *model.Order) (value, percent, volume float64, err error) {
	orders, err := c.storage.FilterOrders(o.UpdatedAt, model.OrderStatusTypeFilled, o.Symbol, o.ID)
	if err != nil {
		return 0, 0, 0, err
	}

	quantity := 0.0
	avgPrice := 0.0
	tradeVolume := 0.0

	for _, order := range orders {
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

		// We keep track of volume to have an indication of costs. (0.001%) binance.
		tradeVolume += order.Quantity * order.Price
	}

	cost := o.Quantity * avgPrice
	price := o.Price
	if o.Type == model.OrderTypeStopLoss || o.Type == model.OrderTypeStopLossLimit {
		price = *o.Stop
	}
	profitValue := o.Quantity*price - cost
	return profitValue, profitValue / cost, tradeVolume, nil
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
		c.notifier.OrError(err)
	}
}

func (c *Controller) processTrade(order *model.Order) {
	profitValue, profit, volume, err := c.calculateProfit(order)
	if err != nil {
		c.notifyError(fmt.Errorf("order/controller storage: %s", err))
		return
	}

	order.Profit = profit
	if _, ok := c.Results[order.Symbol]; !ok {
		c.Results[order.Symbol] = &summary{Symbol: order.Symbol}
	}

	if profitValue >= 0 {
		c.Results[order.Symbol].Win = append(c.Results[order.Symbol].Win, profitValue)
	} else {
		c.Results[order.Symbol].Lose = append(c.Results[order.Symbol].Lose, profitValue)
	}

	c.Results[order.Symbol].Volume = volume

	_, quote := exchange.SplitAssetQuote(order.Symbol)
	c.notify(fmt.Sprintf("[PROFIT] %f %s (%f %%)\n%s", profitValue, quote, profit*100, c.Results[order.Symbol].String()))
}

func (c *Controller) updateOrders() {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	orders, err := c.storage.GetPendingOrders()
	if err != nil {
		c.notifyError(fmt.Errorf("orderController/start: %s", err))
		c.mtx.Unlock()
		return
	}

	// For each pending order, check for updates
	var updatedOrders []model.Order
	for _, order := range orders {
		excOrder, err := c.exchange.Order(order.Symbol, order.ExchangeID)
		if err != nil {
			log.WithField("id", order.ExchangeID).Error("orderControler/get: ", err)
			continue
		}

		// no status change
		if excOrder.Status == order.Status {
			continue
		}

		excOrder.ID = order.ID

		err = c.storage.UpdateOrder(excOrder.ID, excOrder.UpdatedAt, excOrder.Status, excOrder.Quantity, excOrder.Price)
		if err != nil {
			c.notifyError(fmt.Errorf("orderControler/update: %s", err))
			continue
		}

		log.Infof("[ORDER %s] %s", excOrder.Status, excOrder)
		updatedOrders = append(updatedOrders, excOrder)
	}

	for _, processOrder := range updatedOrders {
		if processOrder.Side == model.SideTypeSell && processOrder.Status == model.OrderStatusTypeFilled {
			c.processTrade(&processOrder)
		}
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

func (c *Controller) Position(symbol string) (asset, quote float64, err error) {
	return c.exchange.Position(symbol)
}

func (c *Controller) Order(symbol string, id int64) (model.Order, error) {
	return c.exchange.Order(symbol, id)
}

func (c *Controller) CreateOrderOCO(side model.SideType, symbol string, size, price, stop,
	stopLimit float64) ([]model.Order, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	log.Infof("[ORDER] Creating OCO order for %s", symbol)
	orders, err := c.exchange.CreateOrderOCO(side, symbol, size, price, stop, stopLimit)
	if err != nil {
		c.notifyError(fmt.Errorf("order/controller exchange: %s", err))
		return nil, err
	}

	for i := range orders {
		err := c.storage.CreateOrder(&orders[i])
		if err != nil {
			c.notifyError(fmt.Errorf("order/controller storage: %s", err))
			return nil, err
		}
		go c.orderFeed.Publish(orders[i], true)
	}

	return orders, nil
}

func (c *Controller) CreateOrderLimit(side model.SideType, symbol string, size, limit float64) (model.Order, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	log.Infof("[ORDER] Creating LIMIT %s order for %s", side, symbol)
	order, err := c.exchange.CreateOrderLimit(side, symbol, size, limit)
	if err != nil {
		c.notifyError(fmt.Errorf("order/controller exchange: %s", err))
		return model.Order{}, err
	}

	err = c.storage.CreateOrder(&order)
	if err != nil {
		c.notifyError(fmt.Errorf("order/controller storage: %s", err))
		return model.Order{}, err
	}
	go c.orderFeed.Publish(order, true)
	log.Infof("[ORDER CREATED] %s", order)
	return order, nil
}

func (c *Controller) CreateOrderMarketQuote(side model.SideType, symbol string, amount float64) (model.Order, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	log.Infof("[ORDER] Creating MARKET %s order for %s", side, symbol)
	order, err := c.exchange.CreateOrderMarketQuote(side, symbol, amount)
	if err != nil {
		c.notifyError(fmt.Errorf("order/controller exchange: %s", err))
		return model.Order{}, err
	}

	err = c.storage.CreateOrder(&order)
	if err != nil {
		c.notifyError(fmt.Errorf("order/controller storage: %s", err))
		return model.Order{}, err
	}

	// calculate profit
	if order.Side == model.SideTypeSell {
		c.processTrade(&order)
	}

	go c.orderFeed.Publish(order, true)
	log.Infof("[ORDER CREATED] %s", order)
	return order, err
}

func (c *Controller) CreateOrderMarket(side model.SideType, symbol string, size float64) (model.Order, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	log.Infof("[ORDER] Creating MARKET %s order for %s", side, symbol)
	order, err := c.exchange.CreateOrderMarket(side, symbol, size)
	if err != nil {
		c.notifyError(fmt.Errorf("order/controller exchange: %s", err))
		return model.Order{}, err
	}

	err = c.storage.CreateOrder(&order)
	if err != nil {
		c.notifyError(fmt.Errorf("order/controller storage: %s", err))
		return model.Order{}, err
	}

	// calculate profit
	if order.Side == model.SideTypeSell {
		c.processTrade(&order)
	}

	go c.orderFeed.Publish(order, true)
	log.Infof("[ORDER CREATED] %s", order)
	return order, err
}

func (c *Controller) Cancel(order model.Order) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	log.Infof("[ORDER] Cancelling order for %s", order.Symbol)
	err := c.exchange.Cancel(order)
	if err != nil {
		return err
	}

	err = c.storage.UpdateOrderStatus(order.ID, model.OrderStatusTypePendingCancel)
	if err != nil {
		c.notifyError(fmt.Errorf("order/controller storage: %s", err))
		return err
	}
	log.Infof("[ORDER CANCELED] %s", order)
	return nil
}
