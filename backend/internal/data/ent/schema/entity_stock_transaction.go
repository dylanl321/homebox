package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/schema/mixins"
)

// EntityStockTransaction is the durable, append-only stock operation ledger.
type EntityStockTransaction struct {
	ent.Schema
}

func (EntityStockTransaction) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.BaseMixin{}}
}

func (EntityStockTransaction) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("group_id", uuid.UUID{}),
		field.UUID("entity_id", uuid.UUID{}),
		field.UUID("actor_id", uuid.UUID{}).
			Optional().
			Nillable().
			Immutable(),
		field.Enum("operation").
			Values("adjust", "set", "transfer", "resolve_transfer", "resolve_remove", "legacy").
			Immutable(),
		field.String("workflow").
			MaxLen(100).
			Optional().
			Immutable(),
		field.UUID("source_location_id", uuid.UUID{}).
			Optional().
			Nillable().
			Immutable(),
		field.UUID("destination_location_id", uuid.UUID{}).
			Optional().
			Nillable().
			Immutable(),
		field.Float("quantity").
			Immutable(),
		field.Float("before_total").
			Immutable(),
		field.Float("after_total").
			Immutable(),
		field.Float("source_before").
			Optional().
			Nillable().
			Immutable(),
		field.Float("source_after").
			Optional().
			Nillable().
			Immutable(),
		field.Float("destination_before").
			Optional().
			Nillable().
			Immutable(),
		field.Float("destination_after").
			Optional().
			Nillable().
			Immutable(),
		field.String("reason").
			MaxLen(1000).
			Optional().
			Immutable(),
		field.String("idempotency_key").
			MaxLen(255).
			Immutable(),
		field.String("request_hash").
			MaxLen(64).
			Immutable(),
	}
}

func (EntityStockTransaction) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("group", Group.Type).
			Field("group_id").
			Ref("stock_transactions").
			Required().
			Unique(),
		edge.From("entity", Entity.Type).
			Field("entity_id").
			Ref("stock_transactions").
			Required().
			Unique().
			Annotations(entsql.Annotation{OnDelete: entsql.Cascade}),
	}
}

func (EntityStockTransaction) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("group_id", "idempotency_key").
			Unique(),
		index.Fields("group_id", "created_at"),
		index.Fields("entity_id", "created_at"),
		index.Fields("source_location_id"),
		index.Fields("destination_location_id"),
	}
}
