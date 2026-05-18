package schema

import (
	"fmt"

	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// LingjingTask 京东云灵境豆包系列异步生成任务（文生视频/图生视频）。
type LingjingTask struct {
	ent.Schema
}

func (LingjingTask) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "lingjing_tasks"},
	}
}

func (LingjingTask) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (LingjingTask) Fields() []ent.Field {
	return []ent.Field{
		// JD 云返回的任务 ID，全局唯一
		field.String("gen_task_id").
			MaxLen(128).
			Unique(),
		// 任务类型：text2video / image2video
		field.String("task_type").
			MaxLen(20).
			Validate(validateLingjingTaskType),
		// 任务状态：pending / processing / succeeded / failed
		field.String("status").
			MaxLen(20).
			Default("pending").
			Validate(validateLingjingTaskStatus),
		field.String("error_message").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Optional().
			Nillable(),
		// 成功后的视频/图片 URL（无水印）
		field.String("result_url").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Optional().
			Nillable(),
		// 原始请求参数（JSON），用于审计和重试
		field.JSON("request_body", map[string]any{}),
		// 关联信息
		field.Int64("user_id"),
		field.Int64("api_key_id"),
		field.Int64("account_id"),
		field.Int64("group_id").
			Optional().
			Nillable(),
		// 模型信息
		field.String("model").
			MaxLen(100),
		field.String("duration").
			MaxLen(10),
		field.String("mode").
			MaxLen(10),
		// 计费
		field.Float("cost").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Bool("billed").
			Default(false),
		// 轮询状态
		field.Int("poll_attempts").
			Default(0),
		field.Time("started_at").
			Optional().
			Nillable(),
		field.Time("finished_at").
			Optional().
			Nillable(),
	}
}

func (LingjingTask) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("gen_task_id").Unique(),
		index.Fields("status", "created_at"),
		index.Fields("user_id"),
		index.Fields("api_key_id"),
		index.Fields("account_id"),
	}
}

func validateLingjingTaskType(t string) error {
	switch t {
	case "text2video", "image2video":
		return nil
	default:
		return fmt.Errorf("invalid lingjing task type: %s", t)
	}
}

func validateLingjingTaskStatus(s string) error {
	switch s {
	case "pending", "processing", "succeeded", "failed":
		return nil
	default:
		return fmt.Errorf("invalid lingjing task status: %s", s)
	}
}
