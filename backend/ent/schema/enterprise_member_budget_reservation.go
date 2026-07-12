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

// EnterpriseMemberBudgetReservation makes member budget authorization durable
// across concurrent requests, retries, and process crashes.
type EnterpriseMemberBudgetReservation struct {
	ent.Schema
}

func (EnterpriseMemberBudgetReservation) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "enterprise_member_budget_reservations"},
	}
}

func (EnterpriseMemberBudgetReservation) Fields() []ent.Field {
	return []ent.Field{
		field.String("request_id").MaxLen(128).NotEmpty().Unique(),
		field.Int64("member_id"),
		field.Time("period_start").
			SchemaType(map[string]string{dialect.Postgres: "date"}),
		field.Float("reserved_usd").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.Float("actual_usd").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.String("status").
			MaxLen(20).
			Default("reserved"),
		field.Int64("usage_log_id").
			Optional().
			Nillable().
			Unique(),
		field.Time("expires_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
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

func (EnterpriseMemberBudgetReservation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("member", EnterpriseMember.Type).
			Ref("budget_reservations").
			Field("member_id").
			Unique().
			Required(),
	}
}

func (EnterpriseMemberBudgetReservation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("member_id", "period_start", "status"),
		index.Fields("status", "expires_at"),
	}
}
