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
		field.Int64("group_id").Optional(), // OCO orders
		field.Time("date"),
		field.String("symbol"),
		field.String("side"),
		field.String("type"),
		field.String("status"),
		field.Float("price"),
		field.Float("price_limit").Optional(), // Limit orders
		field.Float("quantity"),
	}
}

func (Order) Edges() []ent.Edge {
	return nil
}
