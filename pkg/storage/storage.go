package storage

import (
	"context"
	"fmt"

	"entgo.io/ent/dialect"
	//nolint
	_ "github.com/mattn/go-sqlite3"
	"github.com/rodrigo-brito/ninjabot/pkg/ent"
)

func NewMemory() (*ent.Client, error) {
	return newClient("file:ent?mode=memory&cache=shared&_fk=1")
}

func New(path string) (*ent.Client, error) {
	return newClient(fmt.Sprintf("file:%s?cache=shared&_fk=1", path))
}

func newClient(dataSource string) (*ent.Client, error) {
	client, err := ent.Open(dialect.SQLite, dataSource)
	if err != nil {
		return nil, err
	}

	if err := client.Schema.Create(context.Background()); err != nil {
		return nil, err
	}

	return client, err
}
