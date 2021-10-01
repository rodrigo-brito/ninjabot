package storage

import (
	"testing"
	"time"

	"github.com/rodrigo-brito/ninjabot/model"
)

func TestStorage(t *testing.T) {
	storage, err := New(FromMemory())
	if err != nil {
		t.Error(err)
	}

	order1 := &model.Order{
		ExchangeID: 20,
		Status:     model.OrderStatusTypeFilled,
		Price:      100,
		Quantity:   2,
	}

	err = storage.CreateOrder(order1)
	if err != nil {
		t.Error(err)
	}

	order2 := &model.Order{
		ExchangeID: 21,
		Status:     model.OrderStatusTypeFilled,
		Price:      150,
		Quantity:   1,
	}

	err = storage.CreateOrder(order2)
	if err != nil {
		t.Error(err)
	}

	order3 := &model.Order{
		ExchangeID: 20,
		Status:     model.OrderStatusTypeNew,
		Price:      10,
		Quantity:   1,
	}

	err = storage.CreateOrder(order3)
	if err != nil {
		t.Error(err)
	}

	order4 := &model.Order{
		ExchangeID: 21,
		Status:     model.OrderStatusTypePartiallyFilled,
		Price:      100,
		Quantity:   2,
	}

	err = storage.CreateOrder(order4)
	if err != nil {
		t.Error(err)
	}

	order5 := &model.Order{
		ExchangeID: 20,
		Status:     model.OrderStatusTypePendingCancel,
		Price:      150,
		Quantity:   1,
	}

	err = storage.CreateOrder(order5)
	if err != nil {
		t.Error(err)
	}

	t.Run("get pending orders", func(t *testing.T) {
		orders, err := storage.GetPendingOrders()
		if err != nil {
			t.Error(err)
		}

		if len(orders) != 3 || orders[0].ID != order3.ID || orders[1].ID != order4.ID || orders[2].ID != order5.ID {
			t.Error("received invalid pending orders: ", orders)
		}
	})

	t.Run("filter orders", func(t *testing.T) {
		orders, err := storage.FilterOrders(time.Now(), model.OrderStatusTypeFilled, order1.Symbol, order1.ID)
		if err != nil {
			t.Error(err)
		}

		if len(orders) != 1 || orders[0].ID != order2.ID {
			t.Error("received invalid orders from filter: ", orders)
		}
	})

	t.Run("update status", func(t *testing.T) {
		err := storage.UpdateOrderStatus(order3.ID, model.OrderStatusTypeFilled)
		if err != nil {
			t.Error(err)
		}

		orders, err := storage.FilterOrders(time.Now(), model.OrderStatusTypeFilled, order1.Symbol, order1.ID)
		if err != nil {
			t.Error(err)
		}

		if len(orders) != 2 || orders[0].ID != order2.ID || orders[1].ID != order3.ID {

			t.Error("received invalid orders from filter: ", orders)
		}
	})

	t.Run("update order", func(t *testing.T) {
		err := storage.UpdateOrder(order4.ID, time.Now(), model.OrderStatusTypeFilled, 5, 200)
		if err != nil {
			t.Error(err)
		}

		orders, err := storage.FilterOrders(time.Now(), model.OrderStatusTypeFilled, order1.Symbol, order1.ID)
		if err != nil {
			t.Error(err)
		}

		if len(orders) != 3 || orders[0].ID != order2.ID || orders[1].ID != order3.ID ||
			orders[2].ID != order4.ID || orders[2].Quantity != 5 || orders[2].Price != 200 {

			t.Error("received invalid orders from filter: ", orders)
		}
	})
}
