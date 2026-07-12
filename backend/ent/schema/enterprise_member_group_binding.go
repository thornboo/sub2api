package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// EnterpriseMemberGroupBinding stores the enterprise owner's ordered group
// intent. Runtime authorization still intersects these rows with current user
// and group eligibility.
type EnterpriseMemberGroupBinding struct {
	ent.Schema
}

func (EnterpriseMemberGroupBinding) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "enterprise_member_group_bindings"},
		field.ID("member_id", "group_id"),
	}
}

func (EnterpriseMemberGroupBinding) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("member_id"),
		field.Int64("group_id"),
		field.Int("sort_order").NonNegative(),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (EnterpriseMemberGroupBinding) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("member", EnterpriseMember.Type).
			Unique().
			Required().
			Field("member_id"),
		edge.To("group", Group.Type).
			Unique().
			Required().
			Field("group_id"),
	}
}

func (EnterpriseMemberGroupBinding) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("group_id"),
		index.Fields("member_id", "sort_order", "group_id"),
	}
}
