package storage

import (
	"context"
	"fmt"

	"entgo.io/ent/dialect"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rodrigo-brito/ninjabot/pkg/ent"
)

func New(path string) (*ent.Client, error) {
	client, err := ent.Open(dialect.SQLite, fmt.Sprintf("file:%s?cache=shared&_fk=1", path))
	if err != nil {
		return nil, err
	}

	if err := client.Schema.Create(context.Background()); err != nil {
		return nil, err
	}

	return client, err
}
