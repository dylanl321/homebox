package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
	"github.com/sysadminsmedia/homebox/backend/internal/data/ent/schema/mixins"
)

// homebox-fork: qr-login
//
// QRLoginTokens holds short-lived, single-use tokens for QR-code device login.
// A logged-in user mints a token that another device (e.g. a phone) redeems
// once to establish a full session. used_at is set on consumption so a replay
// cannot reuse the row.
type QRLoginTokens struct {
	ent.Schema
}

func (QRLoginTokens) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
	}
}

func (QRLoginTokens) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("user_id", uuid.UUID{}),
		field.Bytes("token").
			Unique(),
		field.Time("expires_at").
			Default(func() time.Time { return time.Now().Add(2 * time.Minute) }),
		field.Time("used_at").
			Optional().
			Nillable(),
	}
}

func (QRLoginTokens) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("qr_login_tokens").
			Field("user_id").
			Unique().
			Required().
			Annotations(entsql.Annotation{
				OnDelete: entsql.Cascade,
			}),
	}
}

func (QRLoginTokens) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("token"),
		index.Fields("user_id"),
	}
}
