package schema

import (
	"time"

	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// TeamInvitation 记录企业账号邀请协作管理员的过程，支持邀请尚未注册的邮箱：
// 邀请人填邮箱 -> 生成 token 并发邮件 -> 被邀请人注册/登录该邮箱后凭 token 接受邀请。
//
// zhiguofan fork-only: 企业账号多管理员共享额度与 Key 功能。
type TeamInvitation struct {
	ent.Schema
}

func (TeamInvitation) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "team_invitations"},
	}
}

func (TeamInvitation) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (TeamInvitation) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("owner_user_id"),
		field.String("invited_email").
			NotEmpty().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("token").
			NotEmpty().
			Unique().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("status").
			MaxLen(20).
			Default("pending"),
		field.Time("expires_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("last_sent_at").
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Int64("department_id").
			Optional().
			Nillable(),
		field.String("role").
			MaxLen(20).
			Default("member"),
		field.String("quota_mode").
			MaxLen(20).
			Default("allocated"),
		field.Float("initial_grant_usd").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "numeric(20,8)"}),
		field.Int64("accepted_by_user_id").
			Optional().
			Nillable(),
		field.Time("accepted_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (TeamInvitation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_user_id"),
		index.Fields("invited_email"),
	}
}
