package storage

import (
	"testing"
	"time"

	"github.com/rodrigo-brito/ninjabot/model"
	"github.com/stretchr/testify/require"
)

func TestNewBunt(t *testing.T) {
	now := time.Now()
	repo, err := FromMemory()
	require.NoError(t, err)

	firstOrder := &model.Order{
		ExchangeID: 1,
		Symbol:     "BTCUSDT",
		Side:       model.SideTypeBuy,
		Type:       model.OrderTypeLimit,
		Status:     model.OrderStatusTypeNew,
		Price:      10,
		Quantity:   1,
		CreatedAt:  now.Add(-time.Minute),
		UpdatedAt:  now.Add(-time.Minute),
	}
	err = repo.CreateOrder(firstOrder)
	require.NoError(t, err)

	secondOrder := &model.Order{
		ExchangeID: 2,
		Symbol:     "BTCUSDT",
		Side:       model.SideTypeBuy,
		Type:       model.OrderTypeLimit,
		Status:     model.OrderStatusTypeFilled,
		Price:      10,
		Quantity:   1,
		CreatedAt:  now.Add(time.Minute),
		UpdatedAt:  now.Add(time.Minute),
	}
	err = repo.CreateOrder(secondOrder)
	require.NoError(t, err)

	t.Run("filter with date restriction", func(t *testing.T) {
		orders, err := repo.Orders(WithUpdateAtBeforeOrEqual(now))
		require.NoError(t, err)
		require.Len(t, orders, 1)
		require.Equal(t, orders[0].ExchangeID, int64(1))
	})

	t.Run("get all", func(t *testing.T) {
		orders, err := repo.Orders()
		require.NoError(t, err)
		require.Len(t, orders, 2)
		require.Equal(t, orders[0].ExchangeID, int64(1))
		require.Equal(t, orders[1].ExchangeID, int64(2))
	})

	t.Run("update", func(t *testing.T) {
		firstOrder.Status = model.OrderStatusTypeCanceled
		err := repo.UpdateOrder(firstOrder)
		require.NoError(t, err)

		orders, err := repo.Orders(WithStatus(model.OrderStatusTypeCanceled))
		require.NoError(t, err)
		require.Len(t, orders, 1)
		require.Equal(t, firstOrder.ID, orders[0].ID)
		require.Equal(t, firstOrder.Price, orders[0].Price)
		require.Equal(t, firstOrder.Quantity, orders[0].Quantity)
	})
}
