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

	err = repo.CreateOrder(&model.Order{
		ExchangeID: 1,
		Symbol:     "BTCUSDT",
		Side:       model.SideTypeBuy,
		Type:       model.OrderTypeLimit,
		Status:     model.OrderStatusTypeNew,
		Price:      10,
		Quantity:   1,
		CreatedAt:  now.Add(-time.Minute),
		UpdatedAt:  now.Add(-time.Minute),
	})
	require.NoError(t, err)

	err = repo.CreateOrder(&model.Order{
		ExchangeID: 2,
		Symbol:     "BTCUSDT",
		Side:       model.SideTypeBuy,
		Type:       model.OrderTypeLimit,
		Status:     model.OrderStatusTypeFilled,
		Price:      10,
		Quantity:   1,
		CreatedAt:  now.Add(time.Minute),
		UpdatedAt:  now.Add(time.Minute),
	})
	require.NoError(t, err)

	t.Run("pending order", func(t *testing.T) {
		pendings, err := repo.GetPendingOrders()
		require.NoError(t, err)
		require.Len(t, pendings, 1)
		require.Equal(t, model.OrderStatusTypeNew, pendings[0].Status)
	})

	t.Run("filter with date restriction", func(t *testing.T) {
		orders, err := repo.Filter(WithUpdateAtBeforeOrEqual(now))
		require.NoError(t, err)
		require.Len(t, orders, 1)
		require.Equal(t, orders[0].ExchangeID, int64(1))
	})

	t.Run("get all", func(t *testing.T) {
		orders, err := repo.Filter()
		require.NoError(t, err)
		require.Len(t, orders, 2)
		require.Equal(t, orders[0].ExchangeID, int64(1))
		require.Equal(t, orders[1].ExchangeID, int64(2))
	})
}
