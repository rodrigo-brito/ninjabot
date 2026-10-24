package storage

import (
	"os"
	"testing"

	"gorm.io/gorm"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"

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

func TestFromSQL_Tag(t *testing.T) {
	path := t.TempDir() + "/tags.db"

	// a database created before orders had a tag
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE orders (id integer PRIMARY KEY AUTOINCREMENT, pair text, status text)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO orders (pair, status) VALUES ('BTCUSDT', 'FILLED')`).Error)

	repo, err := FromSQL(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err, "AutoMigrate adds the column")

	require.NoError(t, repo.CreateOrder(&model.Order{Pair: "ETHUSDT", Tag: "trend", Status: model.OrderStatusTypeNew}))
	orders, err := repo.Orders(WithPair("ETHUSDT"))
	require.NoError(t, err)
	require.Len(t, orders, 1)
	require.Equal(t, "trend", orders[0].Tag)

	old, err := repo.Orders(WithPair("BTCUSDT"))
	require.NoError(t, err)
	require.Len(t, old, 1)
	require.Empty(t, old[0].Tag)
}
