package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// EnterpriseMember is an enterprise-owned, non-login identity used to group
// API keys, ordered group access, budgets, and usage evidence.
type EnterpriseMember struct {
	ent.Schema
}

func (EnterpriseMember) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "enterprise_members"},
	}
}

func (EnterpriseMember) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (EnterpriseMember) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("enterprise_user_id"),
		field.String("member_code").
			MaxLen(100).
			NotEmpty(),
		field.String("name").
			MaxLen(100).
			NotEmpty(),
		field.String("status").
			MaxLen(20).
			Default("active"),
		field.Float("monthly_limit_usd").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Float("rate_limit_5h").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Float("rate_limit_1d").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Float("rate_limit_7d").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Int64("version").
			Default(1).
			Positive(),
		field.Time("removed_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (EnterpriseMember) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("enterprise_user", User.Type).
			Ref("enterprise_members").
			Field("enterprise_user_id").
			Unique().
			Required(),
		edge.To("api_keys", APIKey.Type),
		edge.To("usage_logs", UsageLog.Type),
		edge.To("groups", Group.Type).
			Through("enterprise_member_group_bindings", EnterpriseMemberGroupBinding.Type),
		edge.To("budget_periods", EnterpriseMemberBudgetPeriod.Type),
		edge.To("budget_reservations", EnterpriseMemberBudgetReservation.Type),
		edge.To("budget_entries", EnterpriseMemberBudgetEntry.Type),
	}
}

func (EnterpriseMember) Indexes() []ent.Index {
	return []ent.Index{
		// Archived members keep their code. A permanently removed tombstone is
		// assigned a server-only code before the original code can be reused.
		index.Fields("enterprise_user_id", "member_code").Unique(),
		// Composite tenant identity used by api_keys(member_id, user_id).
		index.Fields("id", "enterprise_user_id").Unique(),
		index.Fields("enterprise_user_id", "status"),
		index.Fields("deleted_at"),
		index.Fields("enterprise_user_id", "id").
			Annotations(entsql.IndexWhere("removed_at IS NULL")),
	}
}
