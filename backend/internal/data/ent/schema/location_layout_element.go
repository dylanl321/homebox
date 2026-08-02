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

// LocationLayoutElement stores a wall or direct-child location footprint.
type LocationLayoutElement struct {
	ent.Schema
}

func (LocationLayoutElement) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.BaseMixin{}}
}

func (LocationLayoutElement) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Checks(map[string]string{
			"location_layout_element_origin":   "x >= 0 AND x <= 1 AND y >= 0 AND y <= 1",
			"location_layout_element_rotation": "rotation >= -180 AND rotation <= 180",
			"location_layout_wall_geometry":    "kind <> 'wall' OR (end_x >= 0 AND end_x <= 1 AND end_y >= 0 AND end_y <= 1 AND entity_layout_placements IS NULL)",
			"location_layout_target_geometry":  "kind <> 'location' OR (width > 0 AND height > 0 AND x + width <= 1 AND y + height <= 1 AND entity_layout_placements IS NOT NULL)",
		}),
	}
}

func (LocationLayoutElement) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("kind").Values("wall", "location"),
		field.Float("x"),
		field.Float("y"),
		field.Float("width").Default(0),
		field.Float("height").Default(0),
		field.Float("end_x").Default(0),
		field.Float("end_y").Default(0),
		field.Float("rotation").Default(0),
		field.Int("z_order").Default(0),
	}
}

func (LocationLayoutElement) Indexes() []ent.Index {
	return []ent.Index{
		index.Edges("layout", "target").Unique(),
		index.Edges("layout"),
		index.Edges("target"),
	}
}

func (LocationLayoutElement) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("layout", LocationLayout.Type).
			Ref("elements").
			Unique().
			Required().
			Annotations(entsql.Annotation{OnDelete: entsql.Cascade}),
		edge.From("target", Entity.Type).
			Ref("layout_placements").
			Unique().
			Annotations(entsql.Annotation{OnDelete: entsql.Cascade}),
	}
}
