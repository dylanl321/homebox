package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/schema/mixins"
)

// EntityStockAllocation stores the quantity of one item at one location.
type EntityStockAllocation struct {
	ent.Schema
}

func (EntityStockAllocation) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.BaseMixin{}}
}

func (EntityStockAllocation) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("entity_id", uuid.UUID{}),
		field.UUID("location_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.Float("quantity"),
		field.Bool("is_default").
			Default(false),
	}
}

func (EntityStockAllocation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("entity", Entity.Type).
			Field("entity_id").
			Ref("stock_allocations").
			Required().
			Unique(),
		edge.From("location", Entity.Type).
			Field("location_id").
			Ref("stock_location_allocations").
			Unique(),
	}
}

func (EntityStockAllocation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("entity_id", "location_id").
			Unique(),
		index.Fields("location_id"),
		index.Fields("entity_id", "is_default"),
	}
}
