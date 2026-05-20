// Package schema 定义 Ent ORM 的数据库 schema。
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UsageLog 定义使用日志实体的 schema。
//
// 使用日志记录每次 API 调用的详细信息，包括 token 使用量、成本计算等。
// 这是一个只追加的表，不支持更新和删除。
type UsageLog struct {
	ent.Schema
}

// Annotations 返回 schema 的注解配置。
func (UsageLog) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "usage_logs"},
	}
}

// Fields 定义使用日志实体的所有字段。
func (UsageLog) Fields() []ent.Field {
	return []ent.Field{
		// 关联字段
		field.Int64("user_id"),
		field.Int64("api_key_id"),
		field.Int64("account_id"),
		field.String("request_id").
			MaxLen(64).
			NotEmpty(),
		field.String("model").
			MaxLen(100).
			NotEmpty(),
		// RequestedModel stores the client-requested model name for stable display and analytics.
		// NULL means historical rows written before requested_model dual-write was introduced.
		field.String("requested_model").
			MaxLen(100).
			Optional().
			Nillable(),
		// UpstreamModel stores the actual upstream model name when model mapping
		// is applied. NULL means no mapping — the requested model was used as-is.
		field.String("upstream_model").
			MaxLen(100).
			Optional().
			Nillable(),
		field.Int64("channel_id").Optional().Nillable().Comment("渠道 ID"),
		field.String("model_mapping_chain").MaxLen(500).Optional().Nillable().Comment("模型映射链"),
		field.String("billing_tier").MaxLen(50).Optional().Nillable().Comment("计费层级标签"),
		field.String("billing_mode").MaxLen(20).Optional().Nillable().Comment("计费模式：token/per_request/image"),
		field.Int64("group_id").
			Optional().
			Nillable(),
		field.Int64("subscription_id").
			Optional().
			Nillable(),

		// Token 计数字段
		field.Int("input_tokens").
			Default(0),
		field.Int("output_tokens").
			Default(0),
		field.Int("cache_creation_tokens").
			Default(0),
		field.Int("cache_read_tokens").
			Default(0),
		field.Int("cache_creation_5m_tokens").
			Default(0),
		field.Int("cache_creation_1h_tokens").
			Default(0),

		// 成本字段
		field.Float("input_cost").
			Default(0).
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
		field.Float("output_cost").
			Default(0).
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
		field.Float("cache_creation_cost").
			Default(0).
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
		field.Float("cache_read_cost").
			Default(0).
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
		field.Float("total_cost").
			Default(0).
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
		field.Float("actual_cost").
			Default(0).
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
		field.Float("rate_multiplier").
			Default(1).
			SchemaType(map[string]string{dialect.Postgres: "decimal(10,4)"}),

		// account_rate_multiplier: 账号计费倍率快照（NULL 表示按 1.0 处理）
		field.Float("account_rate_multiplier").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(10,4)"}),

		// 上游真实成本快照（Phase 0 P0-2 引入）
		// 与 pricing_source 严格同步：要么同时 NULL，要么同时非空。
		// NULL = 无上游成本快照（provider_pricing 未命中 / masking 短路 / 异步任务待回填），统计页须显式标注。
		// 不允许混入 LiteLLM / account_stats_pricing 估算。详见 docs/upstream-cost-snapshot.md §3.1。
		field.Float("upstream_unit_price_input").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
		field.Float("upstream_unit_price_output").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
		field.Float("upstream_unit_price_cache_creation").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
		field.Float("upstream_unit_price_cache_read").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
		field.Float("upstream_total_cost").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}).
			Comment("上游真实成本快照；NULL 表示无上游单价"),

		// Provider 规范化键快照（Phase 0 P0-2 引入）
		// 规范化规则见 docs/glossary.md §1.3 normalize_provider 别名表
		field.String("provider").
			MaxLen(50).
			Optional().
			Nillable().
			Comment("规范化的 provider_key，如 anthropic/openai/deepseek/siliconflow"),

		// 计价来源标签（Phase 0 P0-2 引入）
		// 与 upstream_total_cost 严格同步：provider_table（命中 provider_pricing）或 NULL
		// 预留 upstream_billing（未来接上游账单 API）；不允许写入 litellm/fallback 等估算来源
		field.String("pricing_source").
			MaxLen(20).
			Optional().
			Nillable(),

		// 异步任务计费回填（Phase 0 P0-7 引入，与 fork 12 lingjing_poll_runner 对齐）
		// 同步请求 NULL；lingjing 等异步任务记录 gen_task_id
		field.String("async_task_id").
			MaxLen(64).
			Optional().
			Nillable(),
		// 成本最终确定时刻；同步=request_end，异步=poll_runner 触发计费时刻
		field.Time("cost_finalized_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),

		// 其他字段
		field.Int8("billing_type").
			Default(0),
		field.Bool("stream").
			Default(false),
		field.Int("duration_ms").
			Optional().
			Nillable(),
		field.Int("first_token_ms").
			Optional().
			Nillable(),
		field.String("user_agent").
			MaxLen(512).
			Optional().
			Nillable(),
		field.String("ip_address").
			MaxLen(45). // 支持 IPv6
			Optional().
			Nillable(),

		// 图片生成字段（仅 gemini-3-pro-image 等图片模型使用）
		field.Int("image_count").
			Default(0),
		field.String("image_size").
			MaxLen(10).
			Optional().
			Nillable(),
		field.String("image_input_size").
			MaxLen(32).
			Optional().
			Nillable(),
		field.String("image_output_size").
			MaxLen(32).
			Optional().
			Nillable(),
		field.String("image_size_source").
			MaxLen(16).
			Optional().
			Nillable(),
		field.JSON("image_size_breakdown", map[string]int{}).
			Optional().
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		// Cache TTL Override 标记（管理员强制替换了缓存 TTL 计费）
		field.Bool("cache_ttl_overridden").
			Default(false),

		// 多 Endpoint 快照（Phase 5 P5-1 引入）
		// endpoint_id = Endpoint.stable_id 快照（修改 endpoint 不影响历史行）
		field.String("endpoint_id").
			MaxLen(128).
			Optional().
			Nillable().
			Comment("Endpoint.stable_id 快照；NULL = 非 generic 账号或单 endpoint 派生"),
		// endpoint_protocol = endpoint 的出站协议快照（取值见 glossary.md §1.1）
		field.String("endpoint_protocol").
			MaxLen(50).
			Optional().
			Nillable().
			Comment("endpoint 出站协议快照；NULL = 非 generic 账号"),

		// 时间戳（只有 created_at，日志不可修改）
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

// Edges 定义使用日志实体的关联关系。
func (UsageLog) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("usage_logs").
			Field("user_id").
			Required().
			Unique(),
		edge.From("api_key", APIKey.Type).
			Ref("usage_logs").
			Field("api_key_id").
			Required().
			Unique(),
		edge.From("account", Account.Type).
			Ref("usage_logs").
			Field("account_id").
			Required().
			Unique(),
		edge.From("group", Group.Type).
			Ref("usage_logs").
			Field("group_id").
			Unique(),
		edge.From("subscription", UserSubscription.Type).
			Ref("usage_logs").
			Field("subscription_id").
			Unique(),
	}
}

// Indexes 定义数据库索引，优化查询性能。
func (UsageLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("api_key_id"),
		index.Fields("account_id"),
		index.Fields("group_id"),
		index.Fields("subscription_id"),
		index.Fields("created_at"),
		index.Fields("model"),
		index.Fields("requested_model"),
		index.Fields("request_id"),
		// 复合索引用于时间范围查询
		index.Fields("user_id", "created_at"),
		index.Fields("api_key_id", "created_at"),
		// 分组维度时间范围查询（线上由 SQL 迁移创建 group_id IS NOT NULL 的部分索引）
		index.Fields("group_id", "created_at"),
	}
}
