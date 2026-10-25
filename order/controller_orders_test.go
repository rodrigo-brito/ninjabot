package order

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/storage"
	"github.com/rodrigo-brito/ninjabot/testdata/mocks"
)

// newTestController wires a controller to a mocked exchange and an in-memory
// storage.
func newTestController(t *testing.T) (*Controller, *mocks.Exchange) {
	t.Helper()

	memory, err := storage.FromMemory()
	require.NoError(t, err)

	exchange := &mocks.Exchange{}
	controller := NewController(context.Background(), exchange, memory, NewOrderFeed())

	return controller, exchange
}

// filledOrder is the exchange reply for a market order.
func filledOrder(id int64, side model.SideType, price, quantity float64) model.Order {
	return model.Order{
		ExchangeID: id,
		Pair:       "BTCUSDT",
		Side:       side,
		Type:       model.OrderTypeMarket,
		Status:     model.OrderStatusTypeFilled,
		Price:      price,
		Quantity:   quantity,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

func TestControllerStatus(t *testing.T) {
	controller, _ := newTestController(t)

	require.Equal(t, Status(""), controller.Status(), "a fresh controller has no status yet")

	controller.Start()
	require.Equal(t, StatusRunning, controller.Status())

	controller.Start() // starting twice is a no-op
	require.Equal(t, StatusRunning, controller.Status())

	controller.Stop()
	require.Equal(t, StatusStopped, controller.Status())

	controller.Stop() // stopping twice is a no-op
	require.Equal(t, StatusStopped, controller.Status())
}

func TestControllerExchangeDelegation(t *testing.T) {
	controller, exchange := newTestController(t)

	account := model.Account{Balances: []model.Balance{{Asset: "USDT", Free: 1000}}}
	exchange.On("Account").Return(account, nil)
	exchange.On("LastQuote", mock.Anything, "BTCUSDT").Return(100.0, nil)
	exchange.On("Position", "BTCUSDT").Return(2.0, 1000.0, nil)
	exchange.On("Order", "BTCUSDT", int64(1)).Return(filledOrder(1, model.SideTypeBuy, 100, 1), nil)

	t.Run("Account", func(t *testing.T) {
		result, err := controller.Account()

		require.NoError(t, err)
		require.Equal(t, account, result)
	})

	t.Run("LastQuote", func(t *testing.T) {
		quote, err := controller.LastQuote("BTCUSDT")

		require.NoError(t, err)
		require.Equal(t, 100.0, quote)
	})

	t.Run("Order", func(t *testing.T) {
		order, err := controller.Order("BTCUSDT", 1)

		require.NoError(t, err)
		require.Equal(t, int64(1), order.ExchangeID)
	})

	t.Run("PositionValue reports an exchange failure", func(t *testing.T) {
		failing, exchange := newTestController(t)
		exchange.On("Position", "ETHUSDT").Return(0.0, 0.0, errors.New("exchange down"))

		_, err := failing.PositionValue("ETHUSDT")

		require.ErrorContains(t, err, "exchange down")
	})
}

func TestControllerSetNotifier(t *testing.T) {
	controller, _ := newTestController(t)
	notifier := &mocks.Notifier{}
	notifier.On("Notify", mock.Anything).Return()
	notifier.On("OnError", mock.Anything).Return()

	controller.SetNotifier(notifier)

	controller.notify("hello")
	controller.notifyError(errors.New("boom"))

	notifier.AssertCalled(t, "Notify", "hello")
	notifier.AssertNumberOfCalls(t, "OnError", 1)
}

func TestControllerCreateOrderFailures(t *testing.T) {
	failure := errors.New("exchange rejected the order")

	tests := []struct {
		name   string
		expect func(*mocks.Exchange)
		run    func(*Controller) error
	}{
		{
			name: "OCO",
			expect: func(e *mocks.Exchange) {
				e.On("CreateOrderOCO", model.SideTypeSell, "BTCUSDT", 1.0, 120.0, 90.0, 89.0).
					Return([]model.Order{}, failure)
			},
			run: func(c *Controller) error {
				_, err := c.CreateOrderOCO(model.SideTypeSell, "BTCUSDT", 1, 120, 90, 89)
				return err
			},
		},
		{
			name: "limit",
			expect: func(e *mocks.Exchange) {
				e.On("CreateOrderLimit", model.SideTypeBuy, "BTCUSDT", 1.0, 100.0).
					Return(model.Order{}, failure)
			},
			run: func(c *Controller) error {
				_, err := c.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 1, 100)
				return err
			},
		},
		{
			name: "market",
			expect: func(e *mocks.Exchange) {
				e.On("CreateOrderMarket", model.SideTypeBuy, "BTCUSDT", 1.0).
					Return(model.Order{}, failure)
			},
			run: func(c *Controller) error {
				_, err := c.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 1)
				return err
			},
		},
		{
			name: "market quote",
			expect: func(e *mocks.Exchange) {
				e.On("CreateOrderMarketQuote", model.SideTypeBuy, "BTCUSDT", 100.0).
					Return(model.Order{}, failure)
			},
			run: func(c *Controller) error {
				_, err := c.CreateOrderMarketQuote(model.SideTypeBuy, "BTCUSDT", 100)
				return err
			},
		},
		{
			name: "stop",
			expect: func(e *mocks.Exchange) {
				e.On("CreateOrderStop", "BTCUSDT", 1.0, 90.0).Return(model.Order{}, failure)
			},
			run: func(c *Controller) error {
				_, err := c.CreateOrderStop("BTCUSDT", 1, 90)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller, exchange := newTestController(t)
			notifier := &mocks.Notifier{}
			notifier.On("OnError", mock.Anything).Return()
			controller.SetNotifier(notifier)
			tt.expect(exchange)

			require.ErrorIs(t, tt.run(controller), failure)
			notifier.AssertNumberOfCalls(t, "OnError", 1)
		})
	}
}

func TestControllerCreateOrderSuccess(t *testing.T) {
	t.Run("OCO tracks both legs", func(t *testing.T) {
		controller, exchange := newTestController(t)
		group := int64(7)
		orders := []model.Order{
			{ExchangeID: 1, Pair: "BTCUSDT", Side: model.SideTypeSell, Type: model.OrderTypeLimitMaker,
				Status: model.OrderStatusTypeNew, Price: 120, Quantity: 1, GroupID: &group},
			{ExchangeID: 2, Pair: "BTCUSDT", Side: model.SideTypeSell, Type: model.OrderTypeStopLossLimit,
				Status: model.OrderStatusTypeNew, Price: 89, Quantity: 1, GroupID: &group},
		}
		exchange.On("CreateOrderOCO", model.SideTypeSell, "BTCUSDT", 1.0, 120.0, 90.0, 89.0).Return(orders, nil)

		created, err := controller.CreateOrderOCO(model.SideTypeSell, "BTCUSDT", 1, 120, 90, 89)

		require.NoError(t, err)
		require.Len(t, created, 2)
		require.Len(t, controller.pending, 2, "open orders are tracked until the exchange settles them")
	})

	t.Run("stop order is tracked", func(t *testing.T) {
		controller, exchange := newTestController(t)
		exchange.On("CreateOrderStop", "BTCUSDT", 1.0, 90.0).Return(model.Order{
			ExchangeID: 3, Pair: "BTCUSDT", Side: model.SideTypeSell, Type: model.OrderTypeStopLoss,
			Status: model.OrderStatusTypeNew, Price: 90, Quantity: 1,
		}, nil)

		order, err := controller.CreateOrderStop("BTCUSDT", 1, 90)

		require.NoError(t, err)
		require.Equal(t, model.OrderTypeStopLoss, order.Type)
		require.Len(t, controller.pending, 1)
	})

	t.Run("market quote order settles the trade", func(t *testing.T) {
		controller, exchange := newTestController(t)
		exchange.On("CreateOrderMarketQuote", model.SideTypeBuy, "BTCUSDT", 100.0).
			Return(filledOrder(4, model.SideTypeBuy, 100, 1), nil)

		order, err := controller.CreateOrderMarketQuote(model.SideTypeBuy, "BTCUSDT", 100)

		require.NoError(t, err)
		require.Equal(t, model.OrderStatusTypeFilled, order.Status)
		require.Equal(t, 1.0, controller.position["BTCUSDT"].Quantity)
		require.Empty(t, controller.pending, "a filled order is not pending")
	})
}

func TestControllerCancel(t *testing.T) {
	t.Run("marks the order as pending cancel", func(t *testing.T) {
		controller, exchange := newTestController(t)
		open := model.Order{ExchangeID: 5, Pair: "BTCUSDT", Side: model.SideTypeBuy,
			Type: model.OrderTypeLimit, Status: model.OrderStatusTypeNew, Price: 100, Quantity: 1}
		exchange.On("CreateOrderLimit", model.SideTypeBuy, "BTCUSDT", 1.0, 100.0).Return(open, nil)
		exchange.On("Cancel", mock.Anything).Return(nil)

		created, err := controller.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 1, 100)
		require.NoError(t, err)

		require.NoError(t, controller.Cancel(created))
		require.Equal(t, model.OrderStatusTypePendingCancel, controller.pending[created.ID].Status)
	})

	t.Run("propagates an exchange failure", func(t *testing.T) {
		controller, exchange := newTestController(t)
		exchange.On("Cancel", mock.Anything).Return(errors.New("order already filled"))

		err := controller.Cancel(model.Order{ID: 1, Pair: "BTCUSDT"})

		require.ErrorContains(t, err, "order already filled")
	})
}

func TestControllerUpdateOrders(t *testing.T) {
	t.Run("does nothing without pending orders", func(t *testing.T) {
		controller, exchange := newTestController(t)

		controller.UpdateOrders()

		exchange.AssertNotCalled(t, "Order", mock.Anything, mock.Anything)
	})

	t.Run("settles a fill and stops tracking the order", func(t *testing.T) {
		controller, exchange := newTestController(t)
		open := model.Order{ExchangeID: 6, Pair: "BTCUSDT", Side: model.SideTypeBuy,
			Type: model.OrderTypeLimit, Status: model.OrderStatusTypeNew, Price: 100, Quantity: 1}
		exchange.On("CreateOrderLimit", model.SideTypeBuy, "BTCUSDT", 1.0, 100.0).Return(open, nil)

		created, err := controller.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 1, 100)
		require.NoError(t, err)

		filled := created
		filled.Status = model.OrderStatusTypeFilled
		exchange.On("Order", "BTCUSDT", int64(6)).Return(filled, nil)

		controller.UpdateOrders()

		require.Empty(t, controller.pending)
		require.Equal(t, 1.0, controller.position["BTCUSDT"].Quantity)
	})

	t.Run("keeps tracking a partially filled order", func(t *testing.T) {
		controller, exchange := newTestController(t)
		open := model.Order{ExchangeID: 7, Pair: "BTCUSDT", Side: model.SideTypeBuy,
			Type: model.OrderTypeLimit, Status: model.OrderStatusTypeNew, Price: 100, Quantity: 2}
		exchange.On("CreateOrderLimit", model.SideTypeBuy, "BTCUSDT", 2.0, 100.0).Return(open, nil)

		created, err := controller.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 2, 100)
		require.NoError(t, err)

		partial := created
		partial.Status = model.OrderStatusTypePartiallyFilled
		exchange.On("Order", "BTCUSDT", int64(7)).Return(partial, nil)

		controller.UpdateOrders()

		require.Len(t, controller.pending, 1)
		require.Equal(t, model.OrderStatusTypePartiallyFilled, controller.pending[created.ID].Status)
	})

	t.Run("skips orders the exchange cannot report", func(t *testing.T) {
		controller, exchange := newTestController(t)
		open := model.Order{ExchangeID: 8, Pair: "BTCUSDT", Side: model.SideTypeBuy,
			Type: model.OrderTypeLimit, Status: model.OrderStatusTypeNew, Price: 100, Quantity: 1}
		exchange.On("CreateOrderLimit", model.SideTypeBuy, "BTCUSDT", 1.0, 100.0).Return(open, nil)

		created, err := controller.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 1, 100)
		require.NoError(t, err)

		exchange.On("Order", "BTCUSDT", int64(8)).Return(model.Order{}, errors.New("timeout"))

		controller.UpdateOrders()

		require.Len(t, controller.pending, 1, "the order stays pending for the next round")
		require.Equal(t, model.OrderStatusTypeNew, controller.pending[created.ID].Status)
	})

	t.Run("ignores an unchanged status", func(t *testing.T) {
		controller, exchange := newTestController(t)
		open := model.Order{ExchangeID: 9, Pair: "BTCUSDT", Side: model.SideTypeBuy,
			Type: model.OrderTypeLimit, Status: model.OrderStatusTypeNew, Price: 100, Quantity: 1}
		exchange.On("CreateOrderLimit", model.SideTypeBuy, "BTCUSDT", 1.0, 100.0).Return(open, nil)

		created, err := controller.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 1, 100)
		require.NoError(t, err)

		exchange.On("Order", "BTCUSDT", int64(9)).Return(created, nil)

		controller.UpdateOrders()

		require.Len(t, controller.pending, 1)
	})
}

func TestControllerLoadPendingOrders(t *testing.T) {
	t.Run("recovers the open orders left in storage", func(t *testing.T) {
		memory, err := storage.FromMemory()
		require.NoError(t, err)

		open := &model.Order{ExchangeID: 10, Pair: "BTCUSDT", Side: model.SideTypeBuy,
			Type: model.OrderTypeLimit, Status: model.OrderStatusTypeNew, Price: 100, Quantity: 1,
			CreatedAt: time.Now(), UpdatedAt: time.Now()}
		require.NoError(t, memory.CreateOrder(open))

		exchange := &mocks.Exchange{}
		exchange.On("Order", "BTCUSDT", int64(10)).Return(*open, nil).Maybe()

		controller := NewController(context.Background(), exchange, memory, NewOrderFeed())
		controller.Start()
		defer controller.Stop()

		require.Len(t, controller.pending, 1)
	})

	t.Run("reports a storage failure", func(t *testing.T) {
		controller, _ := newTestController(t)
		notifier := &mocks.Notifier{}
		notifier.On("OnError", mock.Anything).Return()
		controller.SetNotifier(notifier)
		controller.storage = failingStorage{}

		controller.loadPendingOrders()

		notifier.AssertNumberOfCalls(t, "OnError", 1)
		require.Empty(t, controller.pending)
	})
}

// failingStorage rejects every write and read, to exercise the recovery paths.
type failingStorage struct{}

var errStorageUnavailable = errors.New("storage unavailable")

func (failingStorage) CreateOrder(*model.Order) error { return errStorageUnavailable }

func (failingStorage) UpdateOrder(*model.Order) error { return errStorageUnavailable }

func (failingStorage) Orders(...storage.OrderFilter) ([]*model.Order, error) {
	return nil, errStorageUnavailable
}

// An order accepted by the exchange but not persisted must be reported: the
// controller would otherwise lose track of it.
func TestControllerStorageFailures(t *testing.T) {
	open := model.Order{ExchangeID: 20, Pair: "BTCUSDT", Side: model.SideTypeBuy,
		Type: model.OrderTypeLimit, Status: model.OrderStatusTypeNew, Price: 100, Quantity: 1}

	tests := []struct {
		name   string
		expect func(*mocks.Exchange)
		run    func(*Controller) error
	}{
		{
			name: "OCO",
			expect: func(e *mocks.Exchange) {
				e.On("CreateOrderOCO", model.SideTypeSell, "BTCUSDT", 1.0, 120.0, 90.0, 89.0).
					Return([]model.Order{open}, nil)
			},
			run: func(c *Controller) error {
				_, err := c.CreateOrderOCO(model.SideTypeSell, "BTCUSDT", 1, 120, 90, 89)
				return err
			},
		},
		{
			name: "limit",
			expect: func(e *mocks.Exchange) {
				e.On("CreateOrderLimit", model.SideTypeBuy, "BTCUSDT", 1.0, 100.0).Return(open, nil)
			},
			run: func(c *Controller) error {
				_, err := c.CreateOrderLimit(model.SideTypeBuy, "BTCUSDT", 1, 100)
				return err
			},
		},
		{
			name: "market",
			expect: func(e *mocks.Exchange) {
				e.On("CreateOrderMarket", model.SideTypeBuy, "BTCUSDT", 1.0).
					Return(filledOrder(21, model.SideTypeBuy, 100, 1), nil)
			},
			run: func(c *Controller) error {
				_, err := c.CreateOrderMarket(model.SideTypeBuy, "BTCUSDT", 1)
				return err
			},
		},
		{
			name: "market quote",
			expect: func(e *mocks.Exchange) {
				e.On("CreateOrderMarketQuote", model.SideTypeBuy, "BTCUSDT", 100.0).
					Return(filledOrder(22, model.SideTypeBuy, 100, 1), nil)
			},
			run: func(c *Controller) error {
				_, err := c.CreateOrderMarketQuote(model.SideTypeBuy, "BTCUSDT", 100)
				return err
			},
		},
		{
			name: "stop",
			expect: func(e *mocks.Exchange) {
				e.On("CreateOrderStop", "BTCUSDT", 1.0, 90.0).Return(open, nil)
			},
			run: func(c *Controller) error {
				_, err := c.CreateOrderStop("BTCUSDT", 1, 90)
				return err
			},
		},
		{
			name: "cancel",
			expect: func(e *mocks.Exchange) {
				e.On("Cancel", mock.Anything).Return(nil)
			},
			run: func(c *Controller) error {
				return c.Cancel(open)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exchange := &mocks.Exchange{}
			tt.expect(exchange)

			notifier := &mocks.Notifier{}
			notifier.On("OnError", mock.Anything).Return()

			controller := NewController(context.Background(), exchange, failingStorage{}, NewOrderFeed())
			controller.SetNotifier(notifier)

			require.ErrorIs(t, tt.run(controller), errStorageUnavailable)
			notifier.AssertNumberOfCalls(t, "OnError", 1)
		})
	}
}

// UpdateOrders keeps going when the storage rejects an update.
func TestControllerUpdateOrdersStorageFailure(t *testing.T) {
	exch := &mocks.Exchange{}
	filled := filledOrder(23, model.SideTypeBuy, 100, 1)
	exch.On("Order", "BTCUSDT", int64(23)).Return(filled, nil)

	notifier := &mocks.Notifier{}
	notifier.On("OnError", mock.Anything).Return()

	controller := NewController(context.Background(), exch, failingStorage{}, NewOrderFeed())
	controller.SetNotifier(notifier)
	controller.pending[1] = model.Order{ID: 1, ExchangeID: 23, Pair: "BTCUSDT",
		Status: model.OrderStatusTypeNew, Side: model.SideTypeBuy, Quantity: 1, Price: 100}

	controller.UpdateOrders()

	notifier.AssertNumberOfCalls(t, "OnError", 1)
	require.Len(t, controller.pending, 1, "the order is retried on the next round")
}

func TestSummarySaveReturns(t *testing.T) {
	result := &summary{
		Pair:            "BTCUSDT",
		WinLong:         []float64{100},
		WinLongPercent:  []float64{0.1},
		LoseLong:        []float64{-50},
		LoseLongPercent: []float64{-0.05},
	}

	t.Run("writes one return per line", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "returns.csv")

		require.NoError(t, result.SaveReturns(path))

		content, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, []string{"0.1000", "-0.0500"}, strings.Fields(string(content)))
	})

	t.Run("fails when the file cannot be created", func(t *testing.T) {
		err := result.SaveReturns(filepath.Join(t.TempDir(), "missing", "returns.csv"))

		require.Error(t, err)
	})
}

func TestSummaryString(t *testing.T) {
	result := &summary{
		Pair:            "BTCUSDT",
		WinLong:         []float64{100, 200},
		WinLongPercent:  []float64{0.1, 0.2},
		LoseLong:        []float64{-50},
		LoseLongPercent: []float64{-0.05},
		Volume:          1000,
	}

	table := result.String()

	require.Contains(t, table, "BTCUSDT")
	require.Contains(t, table, "Trades")
	require.Contains(t, table, "Volume")
	require.Contains(t, table, "250.0000 USDT", "profit is the sum of wins and losses")
}
