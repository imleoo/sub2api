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

// ModelPricing holds the schema definition for the model pricing entity.
// 统一管理远端同步模型和手动自定义模型的计费定价信息。
type ModelPricing struct {
	ent.Schema
}

func (ModelPricing) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "model_pricings"},
	}
}

func (ModelPricing) Fields() []ent.Field {
	return []ent.Field{
		// 模型标识符，唯一键
		field.String("model_id").
			MaxLen(200).
			Unique().
			NotEmpty(),

		// 友好显示名称（可选）
		field.String("display_name").
			MaxLen(200).
			Optional().
			Nillable(),

		// 模型介绍描述（可选）
		field.Text("description").
			Optional().
			Nillable(),

		// 提供商，如 "anthropic", "openai", "lingjing"
		field.String("provider").
			MaxLen(100).
			Default(""),

		// 模式：chat / image_generation / video_generation
		field.String("mode").
			MaxLen(50).
			Default("chat"),

		// 远端同步价格字段（USD per token）
		field.Float("input_cost_per_token").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(30,15)"}),

		field.Float("output_cost_per_token").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(30,15)"}),

		field.Float("cache_creation_input_token_cost").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(30,15)"}),

		field.Float("cache_read_input_token_cost").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(30,15)"}),

		// 图片/视频生成专用价格（USD per image/per second）
		field.Float("output_cost_per_image").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(30,15)"}),

		field.Float("output_cost_per_image_token").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(30,15)"}),

		// 是否支持 prompt caching
		field.Bool("supports_prompt_caching").
			Default(false),

		// 自定义覆盖价格（优先于远端同步价格，null 表示使用远端同步价格）
		field.Float("custom_input_cost").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(30,15)"}),

		field.Float("custom_output_cost").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(30,15)"}),

		// 折扣率，null 或 0 均视为 1.0（不打折）
		// 生效价格 = (custom_cost ?? upstream_cost) × (discount_rate ?? 1.0)
		field.Float("discount_rate").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(10,4)"}),

		// true=手动添加，false=远端同步（同步时跳过手动添加的记录）
		field.Bool("is_custom").
			Default(false),

		// 是否启用（影响用户侧可见性和账号白名单选择器）
		field.Bool("is_enabled").
			Default(true),

		// 最后一次从远端同步的时间
		field.Time("last_synced_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),

		field.Time("created_at").
			Default(time.Now).
			Immutable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),

		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (ModelPricing) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("model_id").Unique(),
		index.Fields("provider"),
		index.Fields("is_custom"),
		index.Fields("is_enabled"),
	}
}
