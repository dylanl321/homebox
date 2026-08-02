package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/schema/mixins"
)

// LocationLayout stores one overhead diagram for an entity.
type LocationLayout struct {
	ent.Schema
}

func (LocationLayout) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.BaseMixin{}}
}

func (LocationLayout) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"location_layout_fixed_canvas": "canvas_width = 1000 AND canvas_height = 700",
			"location_layout_revision":     "revision > 0",
		}),
	}
}

func (LocationLayout) Fields() []ent.Field {
	return []ent.Field{
		field.Int("canvas_width").Default(1000).Positive(),
		field.Int("canvas_height").Default(700).Positive(),
		field.Int("revision").Default(1).Positive(),
	}
}

func (LocationLayout) Indexes() []ent.Index {
	return []ent.Index{
		index.Edges("owner").Unique(),
	}
}

func (LocationLayout) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("owner", Entity.Type).
			Ref("location_layout").
			Unique().
			Required().
			Annotations(entsql.Annotation{OnDelete: entsql.Cascade}),
		edge.To("elements", LocationLayoutElement.Type).
			Annotations(entsql.Annotation{OnDelete: entsql.Cascade}),
	}
}
