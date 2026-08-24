package order

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rodrigo-brito/ninjabot/exchange"
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/service"
	"github.com/rodrigo-brito/ninjabot/storage"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/renderer"
	"github.com/olekukonko/tablewriter/tw"
	log "github.com/sirupsen/logrus"
)

// ErrBotStopped is returned by CreateOrder* when the controller has been stopped.
// Cancel is intentionally not gated, so open orders can still be cleaned up.
var ErrBotStopped = errors.New("bot is stopped")

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

	// Cached values to avoid repeated slice concatenations
	cachedWin         []float64
	cachedWinPercent  []float64
	cachedLose        []float64
	cachedLosePercent []float64
	cacheValid        bool
}

func (s *summary) invalidateCache() {
	s.cacheValid = false
}

func (s *summary) Win() []float64 {
	if !s.cacheValid {
		s.rebuildCache()
	}
	return s.cachedWin
}

func (s *summary) WinPercent() []float64 {
	if !s.cacheValid {
		s.rebuildCache()
	}
	return s.cachedWinPercent
}

func (s *summary) Lose() []float64 {
	if !s.cacheValid {
		s.rebuildCache()
	}
	return s.cachedLose
}

func (s *summary) LosePercent() []float64 {
	if !s.cacheValid {
		s.rebuildCache()
	}
	return s.cachedLosePercent
}

func (s *summary) rebuildCache() {
	s.cachedWin = append(s.WinLong, s.WinShort...)
	s.cachedWinPercent = append(s.WinLongPercent, s.WinShortPercent...)
	s.cachedLose = append(s.LoseLong, s.LoseShort...)
	s.cachedLosePercent = append(s.LoseLongPercent, s.LoseShortPercent...)
	s.cacheValid = true
}

func (s summary) Profit() float64 {
	profit := 0.0
	for _, value := range append(s.Win(), s.Lose()...) {
		profit += value
	}
	return profit
}

// SQN is the System Quality Number (Van Tharp): sqrt(trades) * mean / stddev
// of the trade results. It needs at least two trades with distinct results,
// and is 0 otherwise.
func (s summary) SQN() float64 {
	total := float64(len(s.Win()) + len(s.Lose()))
	if total < 2 {
		return 0
	}
	avgProfit := s.Profit() / total
	stdDev := 0.0
	for _, profit := range append(s.Win(), s.Lose()...) {
		stdDev += (profit - avgProfit) * (profit - avgProfit)
	}
	stdDev = math.Sqrt(stdDev / total)
	if stdDev == 0 {
		return 0
	}
	return math.Sqrt(total) * avgProfit / stdDev
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
	table := tablewriter.NewTable(tableString,
		tablewriter.WithRenderer(renderer.NewBlueprint(tw.Rendition{
			Symbols: tw.NewSymbols(tw.StyleASCII),
		})),
		tablewriter.WithRowAlignmentConfig(tw.CellAlignment{
			PerColumn: []tw.Align{tw.AlignLeft, tw.AlignRight},
		}),
	)
	_, quote := exchange.SplitAssetQuote(s.Pair)
	data := [][]string{
		{"Coin", s.Pair},
		{"Trades", strconv.Itoa(len(s.Lose()) + len(s.Win()))},
		{"Win", strconv.Itoa(len(s.Win()))},
		{"Loss", strconv.Itoa(len(s.Lose()))},
		{"% Win", fmt.Sprintf("%.1f", s.WinPercentage())},
		{"Payoff", fmt.Sprintf("%.3f", s.Payoff())},
		{"Pr.Fact", fmt.Sprintf("%.3f", s.ProfitFactor())},
		{"Profit", fmt.Sprintf("%.4f %s", s.Profit(), quote)},
		{"Volume", fmt.Sprintf("%.4f %s", s.Volume, quote)},
	}
	if err := table.Bulk(data); err != nil {
		log.Error(err)
	}

	if err := table.Render(); err != nil {
		log.Error(err)
	}

	return tableString.String()
}

func (s summary) SaveReturns(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, value := range s.WinPercent() {
		_, err = fmt.Fprintf(file, "%.4f\n", value)
		if err != nil {
			return err
		}
	}

	for _, value := range s.LosePercent() {
		_, err = fmt.Fprintf(file, "%.4f\n", value)
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

type Result struct {
	Pair          string
	ProfitPercent float64
	ProfitValue   float64
	Side          model.SideType
	Duration      time.Duration
	CreatedAt     time.Time
}

type Position struct {
	Side     model.SideType
	AvgPrice float64
	Quantity float64
	// Fee paid to open the position, in quote currency. It is settled against
	// the profit as the position is closed.
	Fee       float64
	CreatedAt time.Time
}

// quantityEpsilon is the relative tolerance used to consider two quantities
// equal. Quantities are accumulated in floating point (0.1 + 0.2 != 0.3), and
// an order sized with the position reported by the exchange may differ from
// the tracked quantity by a rounding error, which would leave a dust position
// open forever.
const quantityEpsilon = 1e-9

func sameQuantity(a, b float64) bool {
	return math.Abs(a-b) <= quantityEpsilon*math.Max(math.Abs(a), math.Abs(b))
}

// filledAt is the time an order was executed: the last update for orders that
// rested on the book, the creation for the ones filled immediately.
func filledAt(order *model.Order) time.Time {
	if !order.UpdatedAt.IsZero() {
		return order.UpdatedAt
	}
	return order.CreatedAt
}

func (p *Position) Update(order *model.Order) (result *Result, finished bool) {
	// Price is the average fill price reported by the exchange, also for stop
	// orders: live exchanges return the executed price, and the paper wallet
	// records the trigger price it filled at.
	price := order.Price

	if p.Side == order.Side {
		p.AvgPrice = (p.AvgPrice*p.Quantity + price*order.Quantity) / (p.Quantity + order.Quantity)
		p.Quantity += order.Quantity
		p.Fee += order.Fee
		return nil, false
	}

	// The order (partially) closes the position: settle the closed leg
	// against the position state before any mutation. Shorts profit when
	// the price falls.
	closedQuantity := math.Min(p.Quantity, order.Quantity)
	profitPrice := price - p.AvgPrice
	if p.Side == model.SideTypeSell {
		profitPrice = p.AvgPrice - price
	}

	// Both legs are charged a fee, but only the share matching the closed
	// quantity belongs to this result.
	var entryFee, exitFee float64
	if p.Quantity > 0 {
		entryFee = p.Fee * closedQuantity / p.Quantity
	}
	if order.Quantity > 0 {
		exitFee = order.Fee * closedQuantity / order.Quantity
	}

	order.Profit = profitPrice / p.AvgPrice
	order.ProfitValue = profitPrice * closedQuantity
	if fee := entryFee + exitFee; fee > 0 {
		order.ProfitValue -= fee
		order.Profit = order.ProfitValue / (p.AvgPrice * closedQuantity)
	}

	closedAt := filledAt(order)
	result = &Result{
		CreatedAt:     closedAt,
		Pair:          order.Pair,
		Duration:      closedAt.Sub(p.CreatedAt),
		ProfitPercent: order.Profit,
		ProfitValue:   order.ProfitValue,
		Side:          p.Side,
	}

	if sameQuantity(p.Quantity, order.Quantity) {
		finished = true
	} else if p.Quantity > order.Quantity {
		p.Quantity -= order.Quantity
		p.Fee -= entryFee
	} else {
		// the order overshoots and opens a position on the other side, the
		// share of its fee not consumed by the close opens the new position
		p.Fee = order.Fee - exitFee
		p.Quantity = order.Quantity - p.Quantity
		p.Side = order.Side
		p.CreatedAt = closedAt
		p.AvgPrice = price
	}

	return result, finished
}

type Controller struct {
	mtx            sync.RWMutex // Use RWMutex for better read concurrency
	updateMtx      sync.Mutex   // serializes UpdateOrders, so a fill is never settled twice
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

	position map[string]*Position

	// pending holds the orders waiting for an update from the exchange, keyed
	// by their storage id: the ones created by this controller plus, once
	// started, the open orders left in storage by a previous run.
	pending map[int64]model.Order
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
		position:       make(map[string]*Position),
		pending:        make(map[int64]model.Order),
	}
}

func (c *Controller) SetNotifier(notifier service.Notifier) {
	c.notifier = notifier
}

func (c *Controller) OnCandle(candle model.Candle) {
	c.lastPrice[candle.Pair] = candle.Close
}

func (c *Controller) updatePosition(o *model.Order) {
	// get filled orders before the current order
	position, ok := c.position[o.Pair]
	if !ok {
		c.position[o.Pair] = &Position{
			AvgPrice:  o.Price,
			Quantity:  o.Quantity,
			Fee:       o.Fee,
			CreatedAt: filledAt(o),
			Side:      o.Side,
		}
		return
	}

	result, closed := position.Update(o)
	if closed {
		delete(c.position, o.Pair)
	}

	if result != nil {
		// TODO: replace by a slice of Result
		summary := c.Results[o.Pair]
		summary.invalidateCache() // Invalidate cache when modifying
		if result.ProfitPercent >= 0 {
			if result.Side == model.SideTypeBuy {
				summary.WinLong = append(summary.WinLong, result.ProfitValue)
				summary.WinLongPercent = append(summary.WinLongPercent, result.ProfitPercent)
			} else {
				summary.WinShort = append(summary.WinShort, result.ProfitValue)
				summary.WinShortPercent = append(summary.WinShortPercent, result.ProfitPercent)
			}
		} else {
			if result.Side == model.SideTypeBuy {
				summary.LoseLong = append(summary.LoseLong, result.ProfitValue)
				summary.LoseLongPercent = append(summary.LoseLongPercent, result.ProfitPercent)
			} else {
				summary.LoseShort = append(summary.LoseShort, result.ProfitValue)
				summary.LoseShortPercent = append(summary.LoseShortPercent, result.ProfitPercent)
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

// isOpen reports whether an order still waits for an update from the exchange.
func isOpen(status model.OrderStatusType) bool {
	switch status {
	case model.OrderStatusTypeNew, model.OrderStatusTypePartiallyFilled, model.OrderStatusTypePendingCancel:
		return true
	}
	return false
}

// track registers an order created by the controller, so that UpdateOrders
// follows it until the exchange reports it filled or canceled.
// Must be called while holding c.mtx.
func (c *Controller) track(order model.Order) {
	if isOpen(order.Status) {
		c.pending[order.ID] = order
	}
}

// loadPendingOrders recovers the open orders left in storage by a previous
// run, so that their fills are still accounted after a restart.
func (c *Controller) loadPendingOrders() {
	orders, err := c.storage.Orders(storage.WithStatusIn(
		model.OrderStatusTypeNew,
		model.OrderStatusTypePartiallyFilled,
		model.OrderStatusTypePendingCancel,
	))
	if err != nil {
		c.notifyError(err)
		return
	}

	c.mtx.Lock()
	defer c.mtx.Unlock()
	for _, order := range orders {
		c.pending[order.ID] = *order
	}
}

// UpdateOrders fetches the status of the pending orders from the exchange,
// persists the ones that changed and settles their fills: the position, the
// profit of the trade and the order feed are updated here.
//
// A started controller runs it every second on its own. Backtests replay years
// of candles in a few seconds, so they must call it after every candle to
// account the fills in the order they happen; it is cheap when nothing is
// pending.
func (c *Controller) UpdateOrders() {
	c.updateMtx.Lock()
	defer c.updateMtx.Unlock()

	c.mtx.RLock()
	orders := make([]model.Order, 0, len(c.pending))
	for _, order := range c.pending {
		orders = append(orders, order)
	}
	c.mtx.RUnlock()

	if len(orders) == 0 {
		return
	}

	// settle in creation order, so that fills of the same tick are applied
	// to the position deterministically
	sort.Slice(orders, func(i, j int) bool { return orders[i].ID < orders[j].ID })

	// For each pending order, check for updates (no lock needed for reads)
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

	if len(updatedOrders) == 0 {
		return
	}

	// Lock only when updating internal state
	c.mtx.Lock()
	for _, processOrder := range updatedOrders {
		if isOpen(processOrder.Status) {
			c.pending[processOrder.ID] = processOrder
		} else {
			delete(c.pending, processOrder.ID)
		}
		c.processTrade(&processOrder)
		c.orderFeed.Publish(processOrder, false)
	}
	c.mtx.Unlock()
}

func (c *Controller) Status() Status {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	return c.status
}

// requireRunning rejects new orders only after an explicit Stop().
// Controllers that never called Start() keep the zero-value status and still
// accept orders, preserving the public API for direct embedding / tests.
// Must be called while holding c.mtx.
func (c *Controller) requireRunning() error {
	if c.status == StatusStopped {
		return ErrBotStopped
	}
	return nil
}

func (c *Controller) Start() {
	c.mtx.Lock()
	if c.status == StatusRunning {
		c.mtx.Unlock()
		return
	}
	c.status = StatusRunning
	c.mtx.Unlock()

	c.loadPendingOrders()

	go func() {
		ticker := time.NewTicker(c.tickerInterval)
		for {
			select {
			case <-ticker.C:
				c.UpdateOrders()
			case <-c.finish:
				ticker.Stop()
				return
			}
		}
	}()
	log.Info("Bot started.")
}

func (c *Controller) Stop() {
	c.mtx.Lock()
	if c.status != StatusRunning {
		c.mtx.Unlock()
		return
	}
	c.status = StatusStopped
	c.mtx.Unlock()

	c.UpdateOrders()
	c.finish <- true
	log.Info("Bot stopped.")
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
	c.mtx.RLock()
	price := c.lastPrice[pair]
	c.mtx.RUnlock()
	return asset * price, nil
}

func (c *Controller) Order(pair string, id int64) (model.Order, error) {
	return c.exchange.Order(pair, id)
}

func (c *Controller) CreateOrderOCO(side model.SideType, pair string, size, price, stop,
	stopLimit float64) ([]model.Order, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if err := c.requireRunning(); err != nil {
		return nil, err
	}

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
		c.track(orders[i])
		go c.orderFeed.Publish(orders[i], true)
	}

	return orders, nil
}

func (c *Controller) CreateOrderLimit(side model.SideType, pair string, size, limit float64) (model.Order, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if err := c.requireRunning(); err != nil {
		return model.Order{}, err
	}

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
	c.track(order)
	go c.orderFeed.Publish(order, true)
	log.Infof("[ORDER CREATED] %s", order)
	return order, nil
}

func (c *Controller) CreateOrderMarketQuote(side model.SideType, pair string, amount float64) (model.Order, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if err := c.requireRunning(); err != nil {
		return model.Order{}, err
	}

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
	c.track(order) // exchanges may report a market order as new and fill it right after
	go c.orderFeed.Publish(order, true)
	log.Infof("[ORDER CREATED] %s", order)
	return order, err
}

func (c *Controller) CreateOrderMarket(side model.SideType, pair string, size float64) (model.Order, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if err := c.requireRunning(); err != nil {
		return model.Order{}, err
	}

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
	c.track(order) // exchanges may report a market order as new and fill it right after
	go c.orderFeed.Publish(order, true)
	log.Infof("[ORDER CREATED] %s", order)
	return order, err
}

func (c *Controller) CreateOrderStop(pair string, size float64, limit float64) (model.Order, error) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	if err := c.requireRunning(); err != nil {
		return model.Order{}, err
	}

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
	c.track(order)
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
	if _, ok := c.pending[order.ID]; ok {
		c.pending[order.ID] = order
	}
	log.Infof("[ORDER CANCELED] %s", order)
	return nil
}
