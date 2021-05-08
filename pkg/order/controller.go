package order

import (
	"context"
	"fmt"
	"time"

	"github.com/rodrigo-brito/ninjabot/pkg/ent"
	"github.com/rodrigo-brito/ninjabot/pkg/ent/order"
	"github.com/rodrigo-brito/ninjabot/pkg/exchange"
	"github.com/rodrigo-brito/ninjabot/pkg/model"

	log "github.com/sirupsen/logrus"
)

type Controller struct {
	ctx       context.Context
	exchange  exchange.Exchange
	storage   *ent.Client
	orderFeed *FeedSubscription
}

func NewController(ctx context.Context, exchange exchange.Exchange, storage *ent.Client,
	orderFeed *FeedSubscription) *Controller {

	return &Controller{
		ctx:       ctx,
		storage:   storage,
		exchange:  exchange,
		orderFeed: orderFeed,
	}
}

func (c Controller) Start() {
	go func() {
		for range time.NewTicker(10 * time.Second).C {
			// get pending orders
			orders, err := c.storage.Order.Query().
				Where(order.StatusIn(
					string(model.OrderStatusTypeNew),
					string(model.OrderStatusTypePartiallyFilled),
					string(model.OrderStatusTypePendingCancel),
				)).
				Order(ent.Desc(order.FieldDate)).
				All(c.ctx)
			if err != nil {
				log.Error("orderController/start:", err)
				continue
			}

			// For each pending order, check for updates
			for _, order := range orders {
				excOrder, err := c.exchange.Order(order.Symbol, order.ExchangeID)
				if err != nil {
					log.WithField("id", order.ExchangeID).Error("orderControler/get: ", err)
					continue
				}

				// no status change
				if string(excOrder.Status) == order.Status {
					continue
				}

				excOrder.ID = order.ID

				_, err = order.Update().
					SetStatus(string(excOrder.Status)).
					SetQuantity(excOrder.Quantity).
					SetPrice(excOrder.Price).Save(c.ctx)
				if err != nil {
					log.Error("orderControler/update: ", err)
					continue
				}

				log.Infof("[ORDER %s] %s", excOrder.Status, excOrder)
				c.orderFeed.Publish(excOrder, false)
			}
		}
	}()
}

func (c Controller) createOrder(order *model.Order) error {
	register, err := c.storage.Order.Create().
		SetExchangeID(order.ExchangeID).
		SetDate(order.Date).
		SetPrice(order.Price).
		SetQuantity(order.Quantity).
		SetSide(string(order.Side)).
		SetSymbol(order.Symbol).
		SetType(string(order.Type)).
		SetStatus(string(order.Status)).
		SetNillableStop(order.Stop).
		SetNillableGroupID(order.GroupID).
		Save(c.ctx)
	if err != nil {
		return fmt.Errorf("error on save order: %w", err)
	}
	order.ID = register.ID
	go c.orderFeed.Publish(*order, true)
	return nil
}

func (c Controller) Account() (model.Account, error) {
	return c.exchange.Account()
}

func (c Controller) Order(symbol string, id int64) (model.Order, error) {
	return c.exchange.Order(symbol, id)
}

func (c Controller) OrderOCO(side model.SideType, symbol string, size, price, stop,
	stopLimit float64) ([]model.Order, error) {

	log.Infof("[ORDER] Creating OCO order for %s", symbol)
	orders, err := c.exchange.OrderOCO(side, symbol, size, price, stop, stopLimit)
	if err != nil {
		log.Errorf("order/controller exchange: %s", err)
		return nil, err
	}

	for i := range orders {
		err := c.createOrder(&orders[i])
		if err != nil {
			log.Errorf("order/controller storage: %s", err)
			return nil, err
		}
	}

	return orders, nil
}

func (c Controller) OrderLimit(side model.SideType, symbol string, size, limit float64) (model.Order, error) {
	log.Infof("[ORDER] Creating LIMIT %s order for %s", side, symbol)
	order, err := c.exchange.OrderLimit(side, symbol, size, limit)
	if err != nil {
		log.Errorf("order/controller exchange: %s", err)
		return model.Order{}, err
	}

	err = c.createOrder(&order)
	if err != nil {
		log.Errorf("order/controller storage: %s", err)
		return model.Order{}, err
	}
	log.Infof("[ORDER CREATED] %s", order)
	return order, nil
}

func (c Controller) OrderMarket(side model.SideType, symbol string, size float64) (model.Order, error) {
	log.Infof("[ORDER] Creating MARKET %s order for %s", side, symbol)
	order, err := c.exchange.OrderMarket(side, symbol, size)
	if err != nil {
		log.Errorf("order/controller exchange: %s", err)
		return model.Order{}, err
	}

	err = c.createOrder(&order)
	if err != nil {
		log.Errorf("order/controller storage: %s", err)
		return model.Order{}, err
	}
	log.Infof("[ORDER CREATED] %s", order)
	return order, err
}

func (c Controller) Cancel(order model.Order) error {
	log.Infof("[ORDER] Cancelling order for %s", order.Symbol)
	err := c.exchange.Cancel(order)
	if err != nil {
		return err
	}

	_, err = c.storage.Order.UpdateOneID(order.ID).
		SetStatus(string(model.OrderStatusTypePendingCancel)).
		Save(c.ctx)
	if err != nil {
		log.Errorf("order/controller storage: %s", err)
		return err
	}
	log.Infof("[ORDER CANCELED] %s", order)
	return nil
}
