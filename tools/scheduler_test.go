package tools

import (
	"testing"

	"github.com/rodrigo-brito/ninjabot"
	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/rodrigo-brito/ninjabot/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockBroker is a mock implementation of the service.Broker interface
type MockBroker struct {
	mock.Mock
}

func (m *MockBroker) Account() (model.Account, error) {
	args := m.Called()
	return args.Get(0).(model.Account), args.Error(1)
}

func (m *MockBroker) Position(pair string) (asset, quote float64, err error) {
	args := m.Called(pair)
	return args.Get(0).(float64), args.Get(1).(float64), args.Error(2)
}

func (m *MockBroker) Order(pair string, id int64) (model.Order, error) {
	args := m.Called(pair, id)
	return args.Get(0).(model.Order), args.Error(1)
}

func (m *MockBroker) CreateOrderOCO(side model.SideType, pair string, size, price, stop, stopLimit float64) ([]model.Order, error) {
	args := m.Called(side, pair, size, price, stop, stopLimit)
	return args.Get(0).([]model.Order), args.Error(1)
}

func (m *MockBroker) CreateOrderLimit(side model.SideType, pair string, size float64, limit float64) (model.Order, error) {
	args := m.Called(side, pair, size, limit)
	return args.Get(0).(model.Order), args.Error(1)
}

func (m *MockBroker) CreateOrderMarket(side model.SideType, pair string, size float64) (model.Order, error) {
	args := m.Called(side, pair, size)
	return args.Get(0).(model.Order), args.Error(1)
}

func (m *MockBroker) CreateOrderMarketQuote(side model.SideType, pair string, quote float64) (model.Order, error) {
	args := m.Called(side, pair, quote)
	return args.Get(0).(model.Order), args.Error(1)
}

func (m *MockBroker) CreateOrderStop(pair string, quantity float64, limit float64) (model.Order, error) {
	args := m.Called(pair, quantity, limit)
	return args.Get(0).(model.Order), args.Error(1)
}

func (m *MockBroker) Cancel(order model.Order) error {
	args := m.Called(order)
	return args.Error(0)
}

// Ensure MockBroker implements service.Broker
var _ service.Broker = (*MockBroker)(nil)

func TestScheduler(t *testing.T) {
	pair := "BTCUSDT"
	scheduler := NewScheduler(pair)
	mockBroker := new(MockBroker)

	// Test BuyWhen
	scheduler.BuyWhen(1.0, func(df *ninjabot.Dataframe) bool {
		return df.Close.Last(0) > 100
	})

	// Test SellWhen
	scheduler.SellWhen(0.5, func(df *ninjabot.Dataframe) bool {
		return df.Close.Last(0) < 50
	})

	// Case 1: No condition met
	df := &ninjabot.Dataframe{
		Pair:  pair,
		Close: ninjabot.Series{75},
	}
	scheduler.Update(df, mockBroker)
	mockBroker.AssertNotCalled(t, "CreateOrderMarket")

	// Case 2: Buy condition met
	df.Close = ninjabot.Series{150}
	mockBroker.On("CreateOrderMarket", ninjabot.SideTypeBuy, pair, 1.0).Return(model.Order{}, nil)
	scheduler.Update(df, mockBroker)
	mockBroker.AssertExpectations(t)

	// Verify that the executed condition is removed
	assert.Len(t, scheduler.orderConditions, 1) // Only Sell condition remains

	// Case 3: Sell condition met
	df.Close = ninjabot.Series{40}
	mockBroker.On("CreateOrderMarket", ninjabot.SideTypeSell, pair, 0.5).Return(model.Order{}, nil)
	scheduler.Update(df, mockBroker)
	mockBroker.AssertExpectations(t)

	// Verify that all conditions are removed
	assert.Len(t, scheduler.orderConditions, 0)

	// Case 4: Broker error
	mockBrokerErr := new(MockBroker)
	scheduler.BuyWhen(1.0, func(df *ninjabot.Dataframe) bool {
		return true
	})
	mockBrokerErr.On("CreateOrderMarket", ninjabot.SideTypeBuy, pair, 1.0).Return(model.Order{}, assert.AnError)
	scheduler.Update(df, mockBrokerErr)
	mockBrokerErr.AssertExpectations(t)

	// Verify that the condition is NOT removed on error
	assert.Len(t, scheduler.orderConditions, 1)
}
