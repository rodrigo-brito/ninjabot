package order

import (
	"context"
	"fmt"

	"github.com/rodrigo-brito/ninjabot/pkg/ent"
	"github.com/rodrigo-brito/ninjabot/pkg/exchange"
	"github.com/rodrigo-brito/ninjabot/pkg/model"
)

type Controller struct {
	ctx       context.Context
	exchange  exchange.Exchange
	storage   *ent.Client
	orderFeed FeedSubscription
}

func NewController(ctx context.Context, exchange exchange.Exchange, storage *ent.Client, orderFeed FeedSubscription) Controller {
	return Controller{
		ctx:       ctx,
		storage:   storage,
		exchange:  exchange,
		orderFeed: orderFeed,
	}
}

func (c Controller) createOrder(order *model.Order) error {
	register, err := c.storage.Order.Create().
		SetExchangeID(order.ExchangeID).
		SetDate(order.Date).
		SetNillableGroupID(order.GroupID).
		SetPrice(order.Price).
		SetNillablePriceLimit(order.PriceLimit).
		SetQuantity(order.Quantity).
		SetSide(string(order.Side)).
		SetSymbol(order.Symbol).
		SetType(string(order.Type)).
		SetStatus(string(order.Status)).Save(c.ctx)
	if err != nil {
		return fmt.Errorf("error on save order: %w", err)
	}
	order.ID = register.ID
	go c.orderFeed.Publish(*order, true)
	return nil
}

func (c Controller) OrderOCO(side exchange.OrderSide, symbol string, size, price, stop, stopLimit float64) ([]model.Order, error) {
	orders, err := c.exchange.OrderOCO(side, symbol, size, price, stop, stopLimit)
	if err != nil {
		return nil, err
	}

	for i := range orders {
		err := c.createOrder(&orders[i])
		if err != nil {
			return nil, err
		}
	}

	return orders, nil
}

func (c Controller) OrderLimit(side exchange.OrderSide, symbol string, size float64, limit float64) (model.Order, error) {
	order, err := c.exchange.OrderLimit(side, symbol, size, limit)
	if err != nil {
		return model.Order{}, nil
	}

	err = c.createOrder(&order)
	return order, nil
}

func (c Controller) OrderMarket(side exchange.OrderSide, symbol string, size float64) (model.Order, error) {
	order, err := c.exchange.OrderMarket(side, symbol, size)
	if err != nil {
		return model.Order{}, nil
	}

	err = c.createOrder(&order)
	return order, nil
}

func (c Controller) Cancel(order model.Order) error {
	err := c.exchange.Cancel(order)
	if err != nil {
		return err
	}

	_, err = c.storage.Order.UpdateOneID(order.ID).
		SetStatus(string(model.OrderStatusTypePendingCancel)).
		Save(c.ctx)
	return err
}
