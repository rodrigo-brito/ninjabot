package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/rodrigo-brito/ninjabot/model"
)

func TestFromSQL(t *testing.T) {
	file, err := os.CreateTemp(os.TempDir(), "*.db")
	require.NoError(t, err)
	defer func() {
		os.RemoveAll(file.Name())
	}()

	repo, err := FromSQL(sqlite.Open(file.Name()), &gorm.Config{})
	require.NoError(t, err)

	storageUseCase(repo, t)
}

func TestFromSQLErrors(t *testing.T) {
	t.Run("fails when the database cannot be opened", func(t *testing.T) {
		// A directory is not a valid sqlite file.
		_, err := FromSQL(sqlite.Open(t.TempDir()), &gorm.Config{})

		require.Error(t, err)
	})

	t.Run("fails when the schema cannot be migrated", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "readonly.db")
		require.NoError(t, os.WriteFile(file, nil, 0o400))

		_, err := FromSQL(sqlite.Open("file:"+file+"?mode=ro"), &gorm.Config{})

		require.Error(t, err)
	})
}

func TestSQLOrdersFilters(t *testing.T) {
	repo, err := FromSQL(sqlite.Open(filepath.Join(t.TempDir(), "orders.db")), &gorm.Config{})
	require.NoError(t, err)

	now := time.Now()
	require.NoError(t, repo.CreateOrder(&model.Order{
		ExchangeID: 1, Pair: "BTCUSDT", Side: model.SideTypeBuy, Type: model.OrderTypeLimit,
		Status: model.OrderStatusTypeNew, Price: 100, Quantity: 1, CreatedAt: now, UpdatedAt: now,
	}))
	require.NoError(t, repo.CreateOrder(&model.Order{
		ExchangeID: 2, Pair: "ETHUSDT", Side: model.SideTypeSell, Type: model.OrderTypeMarket,
		Status: model.OrderStatusTypeFilled, Price: 10, Quantity: 2, CreatedAt: now, UpdatedAt: now,
	}))

	filtered, err := repo.Orders(WithPair("BTCUSDT"), WithStatus(model.OrderStatusTypeNew))

	require.NoError(t, err)
	require.Len(t, filtered, 1)
	require.Equal(t, "BTCUSDT", filtered[0].Pair)
}
