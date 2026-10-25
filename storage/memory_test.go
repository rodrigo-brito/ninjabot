package storage

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/rodrigo-brito/ninjabot/model"
)

func TestNewMemory(t *testing.T) {
	storageUseCase(NewMemory(), t)
}

func TestMemory_UpdateUnknownOrder(t *testing.T) {
	require.Error(t, NewMemory().UpdateOrder(&model.Order{ID: 42}))
}

func TestMemory_ReturnsCopies(t *testing.T) {
	repo := NewMemory()
	order := &model.Order{Pair: "BTCUSDT", Status: model.OrderStatusTypeNew}
	require.NoError(t, repo.CreateOrder(order))

	orders, err := repo.Orders()
	require.NoError(t, err)
	orders[0].Status = model.OrderStatusTypeFilled

	orders, err = repo.Orders(WithStatus(model.OrderStatusTypeNew))
	require.NoError(t, err)
	require.Len(t, orders, 1, "mutating a returned order does not change the storage")
}

func TestMemory_OrderedByUpdateThenID(t *testing.T) {
	repo := NewMemory()
	base := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 12; i++ {
		// IDs 1..12, updated in reverse groups so the order differs from insertion
		require.NoError(t, repo.CreateOrder(&model.Order{UpdatedAt: base.Add(time.Duration(2-i/4) * time.Hour)}))
	}

	orders, err := repo.Orders()
	require.NoError(t, err)
	for i := 1; i < len(orders); i++ {
		prev, cur := orders[i-1], orders[i]
		require.True(t, prev.UpdatedAt.Before(cur.UpdatedAt) ||
			(prev.UpdatedAt.Equal(cur.UpdatedAt) && prev.ID < cur.ID), "position %d", i)
	}
	require.Equal(t, int64(9), orders[0].ID)
}

func TestMemory_Concurrent(t *testing.T) {
	repo := NewMemory()
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				order := &model.Order{Status: model.OrderStatusTypeNew}
				_ = repo.CreateOrder(order)
				order.Status = model.OrderStatusTypeFilled
				_ = repo.UpdateOrder(order)
				_, _ = repo.Orders(WithStatus(model.OrderStatusTypeNew))
			}
		}()
	}
	wg.Wait()

	orders, err := repo.Orders(WithStatus(model.OrderStatusTypeFilled))
	require.NoError(t, err)
	require.Len(t, orders, 800)
}

func BenchmarkStorage_Orders(b *testing.B) {
	bunt, err := FromMemory()
	require.NoError(b, err)

	backends := []struct {
		name string
		repo Storage
	}{
		{"memory", NewMemory()},
		{"buntdb", bunt},
	}

	base := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, backend := range backends {
		for i := 0; i < 1000; i++ {
			status := model.OrderStatusTypeFilled
			if i%50 == 0 {
				status = model.OrderStatusTypeNew
			}
			require.NoError(b, backend.repo.CreateOrder(&model.Order{
				Pair: "BTCUSDT", Status: status, UpdatedAt: base.Add(time.Duration(i) * time.Minute),
			}))
		}

		b.Run(fmt.Sprintf("%s/1000", backend.name), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = backend.repo.Orders(WithStatusIn(model.OrderStatusTypeNew, model.OrderStatusTypePartiallyFilled))
			}
		})
	}
}
