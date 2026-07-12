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

// EnterpriseMemberBudgetEntry is immutable evidence for request usage,
// migration opening balances, manual adjustments, and reconciliation repairs.
type EnterpriseMemberBudgetEntry struct {
	ent.Schema
}

func (EnterpriseMemberBudgetEntry) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "enterprise_member_budget_entries"},
	}
}

func (EnterpriseMemberBudgetEntry) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("member_id"),
		field.Time("period_start").
			SchemaType(map[string]string{dialect.Postgres: "date"}),
		field.String("kind").MaxLen(32).NotEmpty(),
		field.String("request_id").MaxLen(128).Optional().Nillable(),
		field.Float("amount_usd").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.Int64("usage_log_id").
			Optional().
			Nillable().
			Unique(),
		field.String("idempotency_key").
			MaxLen(128).
			NotEmpty().
			Unique(),
		field.Int64("actor_user_id").
			Optional().
			Nillable(),
		field.String("note").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Default(""),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (EnterpriseMemberBudgetEntry) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("member", EnterpriseMember.Type).
			Ref("budget_entries").
			Field("member_id").
			Unique().
			Required(),
	}
}

func (EnterpriseMemberBudgetEntry) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("member_id", "period_start", "created_at"),
		index.Fields("request_id").Unique(),
		index.Fields("kind", "created_at"),
	}
}
