package storage

import (
	"os"
	"testing"

	"gorm.io/gorm"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
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
