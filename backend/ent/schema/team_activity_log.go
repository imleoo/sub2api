package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// TeamActivityLog 记录企业账号协作场景下"谁做了什么"的操作审计，独立于
// usage_logs（usage_logs 记录网关调用消费，不记管理面操作）。
//
// 删除策略：硬删除（参照 PaymentAuditLog）——审计日志只追加，不需要软删除。
//
// zhiguofan fork-only: 企业账号多管理员共享额度与 Key 功能。
type TeamActivityLog struct {
	ent.Schema
}

func (TeamActivityLog) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "team_activity_logs"},
	}
}

func (TeamActivityLog) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("owner_user_id"),
		field.Int64("actor_user_id"),
		field.String("action").
			MaxLen(50),
		field.String("detail").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Default(""),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (TeamActivityLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_user_id"),
	}
}
