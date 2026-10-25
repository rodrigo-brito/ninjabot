package storage

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/buntdb"

	"github.com/rodrigo-brito/ninjabot/model"
)

func TestFromFile(t *testing.T) {
	file, err := os.CreateTemp(os.TempDir(), "*.db")
	require.NoError(t, err)
	defer func() {
		os.RemoveAll(file.Name())
	}()
	db, err := FromFile(file.Name())
	require.NoError(t, err)
	require.NotNil(t, db)
}

func TestNewBunt(t *testing.T) {
	repo, err := FromMemory()
	require.NoError(t, err)

	storageUseCase(repo, t)
}

func TestNewBuntErrors(t *testing.T) {
	t.Run("fails when the database file cannot be opened", func(t *testing.T) {
		// A directory is not a valid buntdb file.
		_, err := FromFile(t.TempDir())

		require.Error(t, err)
	})
}

func TestBuntOrdersSkipsCorruptedEntries(t *testing.T) {
	repo, err := FromMemory()
	require.NoError(t, err)

	require.NoError(t, repo.CreateOrder(&model.Order{
		ExchangeID: 1, Pair: "BTCUSDT", Side: model.SideTypeBuy, Type: model.OrderTypeLimit,
		Status: model.OrderStatusTypeNew, Price: 100, Quantity: 1,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}))

	// Write a value that is not an order, as a corrupted database would hold.
	bunt := repo.(*Bunt)
	require.NoError(t, bunt.db.Update(func(tx *buntdb.Tx) error {
		_, _, err := tx.Set("999", `{"updated_at": "not an order"}`, nil)
		return err
	}))

	orders, err := repo.Orders()

	require.NoError(t, err)
	require.Len(t, orders, 1, "the unreadable entry is skipped")
}
