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

// TeamFundTransfer 企业↔员工余额划转台账（只追加）：grant（管理员划转）、
// reclaim（回收）、auto_topup（共享模式自动补给）。
//
// 删除策略：硬删除、不建 users 外键（与 TeamActivityLog 同理——台账须在
// 用户被物理删除后保留追溯）。
//
// zhiguofan fork-only: 企业组织与额度分配（Team 协作 v2）。
type TeamFundTransfer struct {
	ent.Schema
}

func (TeamFundTransfer) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "team_fund_transfers"},
	}
}

func (TeamFundTransfer) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("owner_user_id"),
		field.Int64("member_user_id"),
		field.String("direction").
			MaxLen(20),
		field.Float("amount").
			SchemaType(map[string]string{dialect.Postgres: "numeric(20,8)"}),
		field.Int64("operator_user_id"),
		field.String("note").
			Default("").
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (TeamFundTransfer) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_user_id", "created_at"),
		index.Fields("member_user_id"),
	}
}
