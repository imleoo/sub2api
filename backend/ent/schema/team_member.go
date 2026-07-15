package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// TeamMember 定义"企业成员"关系：member_user_id 是独立登录的员工账号，
// owner_user_id 是企业主体（创建企业的那个 User，不新建实体）。
// v2：员工自管 Key、消费扣自己余额；企业通过划转分配额度（allocated 手动 /
// shared 自动补给），granted_net_usd 跟踪企业净投入以约束回收上限。
//
// zhiguofan fork-only: 企业组织与额度分配（Team 协作 v2）。
type TeamMember struct {
	ent.Schema
}

func (TeamMember) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "team_members"},
	}
}

func (TeamMember) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (TeamMember) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("owner_user_id"),
		field.Int64("member_user_id"),
		field.String("role").
			MaxLen(20).
			Default("member"),
		field.String("status").
			MaxLen(20).
			Default("active"),
		field.Int64("department_id").
			Optional().
			Nillable(),
		field.String("quota_mode").
			MaxLen(20).
			Default("allocated"),
		field.Float("auto_topup_threshold_usd").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "numeric(20,8)"}),
		field.Float("auto_topup_target_usd").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "numeric(20,8)"}),
		field.Float("granted_net_usd").
			Default(0).
			SchemaType(map[string]string{dialect.Postgres: "numeric(20,8)"}),
	}
}

func (TeamMember) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_user_id", "member_user_id").Unique(),
		index.Fields("owner_user_id"),
		index.Fields("member_user_id"),
	}
}
