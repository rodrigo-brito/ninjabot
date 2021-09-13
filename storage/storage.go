//go:generate go run -mod=mod entgo.io/ent/cmd/ent generate ./schema

package storage

import (
	"context"
	"fmt"

	"entgo.io/ent/dialect"

	//nolint
	_ "github.com/mattn/go-sqlite3"
)

func FromMemory() (*Client, error) {
	return newClient("file::memory:?mode=memory&cache=shared&_fk=1&_mutex=full")
}

func FromFile(path string) (*Client, error) {
	return newClient(fmt.Sprintf("file:%s?cache=shared&_fk=1&_mutex=full", path))
}

func newClient(dataSource string) (*Client, error) {
	client, err := Open(dialect.SQLite, dataSource)
	if err != nil {
		return nil, err
	}

	if err := client.Schema.Create(context.Background()); err != nil {
		return nil, err
	}

	return client, err
}
