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

// TeamDepartment 企业一级部门：企业（owner_user_id）下的扁平部门列表，
// 成员通过 team_members.department_id 归属部门，邀请时可指定部门。
//
// zhiguofan fork-only: 企业组织与额度分配（Team 协作 v2）。
type TeamDepartment struct {
	ent.Schema
}

func (TeamDepartment) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "team_departments"},
	}
}

func (TeamDepartment) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (TeamDepartment) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("owner_user_id"),
		field.String("name").
			NotEmpty().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Int("display_order").
			Default(0),
	}
}

func (TeamDepartment) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_user_id", "name").Unique(),
		index.Fields("owner_user_id"),
	}
}
