package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// EnterpriseProfile 企业客户资料：用户补充企业信息自助升级后落一行，
// 该行存在即代表 user_id 是企业客户（可邀请员工、分配额度）。
//
// zhiguofan fork-only: 企业组织与额度分配（Team 协作 v2）。
type EnterpriseProfile struct {
	ent.Schema
}

func (EnterpriseProfile) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "enterprise_profiles"},
	}
}

func (EnterpriseProfile) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (EnterpriseProfile) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id").
			Unique(),
		field.String("company_name").
			NotEmpty().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("contact_name").
			Default("").
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("contact_phone").
			Default("").
			MaxLen(50),
		field.String("industry").
			Default("").
			MaxLen(100),
	}
}
