package storage

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
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
