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

// EnterpriseMemberBudgetPeriod is a rebuildable control projection for one
// member and one site-billing-timezone calendar month.
type EnterpriseMemberBudgetPeriod struct {
	ent.Schema
}

func (EnterpriseMemberBudgetPeriod) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "enterprise_member_budget_periods"},
	}
}

func (EnterpriseMemberBudgetPeriod) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("member_id"),
		field.Time("period_start").
			SchemaType(map[string]string{dialect.Postgres: "date"}),
		field.String("timezone").
			MaxLen(64).
			Default("Asia/Shanghai"),
		field.Float("used_usd").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Float("reserved_usd").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Int64("version").
			Default(1).
			Positive(),
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

func (EnterpriseMemberBudgetPeriod) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("member", EnterpriseMember.Type).
			Ref("budget_periods").
			Field("member_id").
			Unique().
			Required(),
	}
}

func (EnterpriseMemberBudgetPeriod) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("member_id", "period_start").Unique(),
		index.Fields("period_start"),
	}
}
