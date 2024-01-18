package order

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
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
	Pair             string
	WinLong          []float64
	WinLongPercent   []float64
	WinShort         []float64
	WinShortPercent  []float64
	LoseLong         []float64
	LoseLongPercent  []float64
	LoseShort        []float64
	LoseShortPercent []float64
	Volume           float64
}

func (s summary) Win() []float64 {
	return append(s.WinLong, s.WinShort...)
}

func (s summary) WinPercent() []float64 {
	return append(s.WinLongPercent, s.WinShortPercent...)
}

func (s summary) Lose() []float64 {
	return append(s.LoseLong, s.LoseShort...)
}

func (s summary) LosePercent() []float64 {
	return append(s.LoseLongPercent, s.LoseShortPercent...)
}

func (s summary) Profit() float64 {
	profit := 0.0
	for _, value := range append(s.Win(), s.Lose()...) {
		profit += value
	}
	return profit
}

func (s summary) SQN() float64 {
	total := float64(len(s.Win()) + len(s.Lose()))
	avgProfit := s.Profit() / total
	stdDev := 0.0
	for _, profit := range append(s.Win(), s.Lose()...) {
		stdDev += math.Pow(profit-avgProfit, 2)
	}
	stdDev = math.Sqrt(stdDev / total)
	return math.Sqrt(total) * (s.Profit() / total) / stdDev
}

func (s summary) Payoff() float64 {
	avgWin := 0.0
	avgLose := 0.0

	for _, value := range s.WinPercent() {
		avgWin += value
	}

	for _, value := range s.LosePercent() {
		avgLose += value
	}

	if len(s.Win()) == 0 || len(s.Lose()) == 0 || avgLose == 0 {
		return 0
	}

	return (avgWin / float64(len(s.Win()))) / math.Abs(avgLose/float64(len(s.Lose())))
}

func (s summary) ProfitFactor() float64 {
	if len(s.Lose()) == 0 {
		return 0
	}
	profit := 0.0
	for _, value := range s.WinPercent() {
		profit += value
	}

	loss := 0.0
	for _, value := range s.LosePercent() {
		loss += value
	}
	return profit / math.Abs(loss)
}

func (s summary) WinPercentage() float64 {
	if len(s.Win())+len(s.Lose()) == 0 {
		return 0
	}

	return float64(len(s.Win())) / float64(len(s.Win())+len(s.Lose())) * 100
}

func (s summary) String() string {
	tableString := &strings.Builder{}
	table := tablewriter.NewWriter(tableString)
	_, quote := exchange.SplitAssetQuote(s.Pair)
	data := [][]string{
		{"Coin", s.Pair},
		{"Trades", strconv.Itoa(len(s.Lose()) + len(s.Win()))},
		{"Win", strconv.Itoa(len(s.Win()))},
		{"Loss", strconv.Itoa(len(s.Lose()))},
		{"% Win", fmt.Sprintf("%.1f", s.WinPercentage())},
		{"Payoff", fmt.Sprintf("%.1f", s.Payoff()*100)},
		{"Pr.Fact", fmt.Sprintf("%.1f", s.Payoff()*100)},
		{"Profit", fmt.Sprintf("%.4f %s", s.Profit(), quote)},
		{"Volume", fmt.Sprintf("%.4f %s", s.Volume, quote)},
	}
	table.AppendBulk(data)
	table.SetColumnAlignment([]int{tablewriter.ALIGN_LEFT, tablewriter.ALIGN_RIGHT})
	table.Render()
	return tableString.String()
}

func (s summary) SaveReturns(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, value := range s.WinPercent() {
		_, err = file.WriteString(fmt.Sprintf("%.4f\n", value))
		if err != nil {
			return err
		}
	}

	for _, value := range s.LosePercent() {
		_, err = file.WriteString(fmt.Sprintf("%.4f\n", value))
		if err != nil {
			return err
		}
	}
	return nil
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
	mockTrading    bool
	notifier       service.Notifier
	Results        map[string]*summary
	lastPrice      map[string]float64
	tickerInterval time.Duration
	finish         chan bool
	status         Status

	positions map[string]*model.Position
}

func NewController(ctx context.Context, exchange service.Exchange, storage storage.Storage,
	orderFeed *Feed, mockTrading bool) *Controller {

	return &Controller{
		ctx:            ctx,
		storage:        storage,
		exchange:       exchange,
		orderFeed:      orderFeed,
		mockTrading:    mockTrading,
		positions:      make(map[string]*model.Position),
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
	// Called only if the method is not overwritten in the exchange
	c.lastPrice[candle.Pair] = candle.Close
	c.positions[candle.Pair].CurrentCandle = &candle
}

func (c *Controller) updatePosition(o *model.Order) {
	// get filled orders before the current order
	//position, ok := c.exchange.Position(o.Pair)
	position := c.positions[o.Pair]

	if position == nil {
		position := model.Position{
			Pair:     o.Pair,
			Side:     o.Side,
			Quantity: o.Quantity,
			// last price not available
			EntryPrice: o.Price,
			Leverage:   o.Leverage,
			Size:       o.Quantity * o.Price,
			CurrentCandle: &model.Candle{
				Close: c.lastPrice[o.Pair],
			},
		}

		c.positions[o.Pair] = &position

		return
	}

	result, closed := position.Update(o)
	if closed {
		delete(c.positions, o.Pair)
	}

	if result != nil {
		// TODO: replace by a slice of Result
		if result.ProfitPercent >= 0 {
			if result.Side == model.SideTypeBuy {
				c.Results[o.Pair].WinLong = append(c.Results[o.Pair].WinLong, result.ProfitValue)
				c.Results[o.Pair].WinLongPercent = append(c.Results[o.Pair].WinLongPercent, result.ProfitPercent)
			} else {
				c.Results[o.Pair].WinShort = append(c.Results[o.Pair].WinShort, result.ProfitValue)
				c.Results[o.Pair].WinShortPercent = append(c.Results[o.Pair].WinShortPercent, result.ProfitPercent)
			}
		} else {
			if result.Side == model.SideTypeBuy {
				c.Results[o.Pair].LoseLong = append(c.Results[o.Pair].LoseLong, result.ProfitValue)
				c.Results[o.Pair].LoseLongPercent = append(c.Results[o.Pair].LoseLongPercent, result.ProfitPercent)
			} else {
				c.Results[o.Pair].LoseShort = append(c.Results[o.Pair].LoseShort, result.ProfitValue)
				c.Results[o.Pair].LoseShortPercent = append(c.Results[o.Pair].LoseShortPercent, result.ProfitPercent)
			}
		}

		_, quote := exchange.SplitAssetQuote(o.Pair)
		c.notify(fmt.Sprintf(
			"[PROFIT] %f %s (%f %%)\n`%s`",
			result.ProfitValue,
			quote,
			result.ProfitPercent*100,
			c.Results[o.Pair].String(),
		))
	}
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

	// update position size / avg price
	c.updatePosition(order)
}

func (c *Controller) updatePositions() {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	// Check active positions for liquidation
	for _, pos := range c.positions {
		if c.mockTrading && pos.CurrentCandle != nil && pos.Liquidated() {
			// Create an opposite market order if the position is liquidated (only for backtesting and paper wallet?)
			// If we're in the real market, the position will automatically be liquidated (futures)
			// Thus, isn't necessary to create an order, but we still need to remove the position from the map[string]*model.Position
			// Also, we can have a watcher that keep in sync the map with the info from the exchange
			c.CreateOrderMarket(pos.OppositeSide(), pos.Pair, pos.Quantity)
		}
	}
}

func (c *Controller) updateOrders() {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	//pending orders
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
					c.updatePositions()
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
		c.updatePositions()
		c.finish <- true
		log.Info("Bot stopped.")
	}
}

func (c *Controller) Account() (model.Account, error) {
	return c.exchange.Account()
}

func (c *Controller) Position(pair string) (*model.Position, bool) {
	//return c.exchange.Position(pair)

	if !c.mockTrading {
		// TODO: Update the positions using the exchange data
	}

	for _, pos := range c.positions {
		if pos.Pair == pair {
			pos.CurrentCandle = &model.Candle{Close: c.lastPrice[pair]}
			return pos, true
		}
	}

	return &model.Position{}, false
}

func (c *Controller) Positions() map[string]*model.Position {
	return c.exchange.Positions()
}

func (c *Controller) OpenPosition(side model.SideType, pair string, size float64, leverage int) error {
	for _, pos := range c.positions {
		if pos.Pair == pair && pos.Side == side {
			return errors.New("position already exists")
		}
	}

	_, err := c.CreateOrderMarketQuote(side, pair, size, leverage)
	if err != nil {
		return err
	}

	return nil
}

func (c *Controller) ClosePosition(position *model.Position) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	log.Infof("[Position] Close Position %s", position.Pair)
	order, err := c.exchange.CreateOrderMarket(position.OppositeSide(), position.Pair, position.Quantity)
	if err != nil {
		c.notifyError(err)
		return err
	}

	err = c.storage.CreateOrder(&order)
	if err != nil {
		c.notifyError(err)
		return err
	}

	// calculate profit
	c.processTrade(&order)
	go c.orderFeed.Publish(order, true)
	return nil
}

func (c *Controller) LastQuote(pair string) (float64, error) {
	return c.exchange.LastQuote(c.ctx, pair)
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

func (c *Controller) CreateOrderMarketQuote(side model.SideType, pair string, amount float64, leverage int) (model.Order, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	log.Infof("[ORDER] Creating MARKET %s order for %s", side, pair)
	order, err := c.exchange.CreateOrderMarketQuote(side, pair, amount, leverage)
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
