package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

type Order struct {
	ent.Schema
}

func (Order) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id"),
		field.Int64("exchange_id"),
		field.Time("created_at"),
		field.Time("updated_at"),
		field.String("symbol"),
		field.String("side"),
		field.String("type"),
		field.String("status"),
		field.Float("price"),
		field.Float("quantity"),

		field.Int64("group_id").Optional(), // OCO orders
		field.Float("stop").Optional(),     // OCO / Stop limit orders
	}
}

func (Order) Edges() []ent.Edge {
	return nil
}
