package storage

import (
	"log"
	"os"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestFromSQL(t *testing.T) {
	file, err := os.CreateTemp(os.TempDir(), "*.db")
	require.NoError(t, err)
	log.Println(file.Name())
	defer func() {
		os.Remove(file.Name())
	}()

	repo, err := FromSQL(sqlite.Open(file.Name()), &gorm.Config{})
	require.NoError(t, err)

	storageUseCase(repo, t)
}
