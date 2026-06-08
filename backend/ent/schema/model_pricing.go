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

		// 计费单位：token=按 token；second=按秒；image_generation=按张/次；video_generation=按视频/秒。
		field.Enum("pricing_unit").
			Values("token", "second", "image_generation", "video_generation").
			Default("token"),

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

		// Priority service tier 价格（service_tier=priority 时优先使用）
		field.Float("input_cost_per_token_priority").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(30,15)"}),

		field.Float("output_cost_per_token_priority").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(30,15)"}),

		field.Float("cache_read_input_token_cost_priority").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(30,15)"}),

		// Anthropic ephemeral cache 5min/1h 档位价格（仅 supports_cache_breakdown=true 生效）
		field.Float("cache_creation_5m_token_cost").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(30,15)"}),

		field.Float("cache_creation_1h_token_cost").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(30,15)"}),

		// 是否启用 5m/1h 分档计费
		field.Bool("supports_cache_breakdown").
			Default(false),

		// 图片输出 token 独立费率（多模态模型如灵境豆包），未设置则回退 output_cost_per_token
		field.Float("image_output_price_per_token").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(30,15)"}),

		// 长上下文计费：超过 threshold 时整个会话的 input/output 乘以对应 multiplier
		// 三个字段必须配套出现，缺一不应用长上下文计费
		field.Int64("long_context_input_token_threshold").
			Optional().
			Nillable(),

		field.Float("long_context_input_cost_multiplier").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(10,4)"}),

		field.Float("long_context_output_cost_multiplier").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(10,4)"}),

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
		// SSOT 重构后已被 source 字段语义覆盖，保留兼容老查询。
		field.Bool("is_custom").
			Default(false),

		// 视频按 token 计费的分档单价（JSON 文本，[]VideoPriceTier，¥/百万 token 原值）。
		// 通用化后视频分档不再内置，由上传的定价 JSON 解析后存入。
		field.Text("tier_pricing").
			Optional().
			Nillable(),

		// 是否启用（影响用户侧可见性和账号白名单选择器）
		field.Bool("is_enabled").
			Default(true),

		// SSOT 元数据：数据来源
		// litellm = 远端 JSON 定时同步；upstream_sync = 账号拉取 /v1/models；
		// manual = 管理员手工创建/编辑；bootstrap = PR-3 兜底注入；lingjing = 灵境静态价格
		field.String("source").
			MaxLen(20).
			Default("manual"),

		// SSOT 元数据：upstream_sync 时记录账号 name（便于"从该 provider 同步过来的模型"列表）
		field.String("source_provider").
			MaxLen(100).
			Default(""),

		// SSOT 元数据：触发入库的账号 id（仅 upstream_sync）
		field.Int64("source_account_id").
			Optional().
			Nillable(),

		// SSOT 元数据：定价状态
		// priced = 已配置上游价或自定义价；unpriced = 仅有 model_id 无价格，计费会被 block；
		// disabled = 显式标记禁用
		field.String("pricing_status").
			MaxLen(20).
			Default("unpriced"),

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
		index.Fields("source"),
		index.Fields("pricing_status"),
	}
}
