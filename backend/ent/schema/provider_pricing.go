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

// ProviderPricing 是上游 Provider 单价表（Phase 0 P0-1 引入）。
// 与 UsageLog.upstream_unit_price_* / upstream_total_cost / pricing_source 字段配套，
// 由 service.UpstreamCostResolver.Resolve(...) 查询，严格两态：命中返回真实单价快照，
// 未命中保持 NULL，**不混入** LiteLLM/account_stats_pricing 等估算回退（详见 docs/upstream-cost-snapshot.md §3）。
type ProviderPricing struct {
	ent.Schema
}

func (ProviderPricing) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "provider_pricings"},
	}
}

func (ProviderPricing) Fields() []ent.Field {
	return []ent.Field{
		// 规范化 provider_key（见 docs/glossary.md §1.3 normalize_provider 别名表）
		field.String("provider").
			MaxLen(50).
			NotEmpty(),

		// 上游模型 ID。精确名（如 "claude-sonnet-4-6"）或 "*" 单星通配匹配所有模型
		field.String("model").
			MaxLen(100).
			NotEmpty(),

		// 计费模式：token / per_request / image
		// - token: 按四档单价 × token 数累加
		// - per_request: 按 input_price 单价 × 1 次（如 lingjing Seedance 视频任务）
		// - image: 按 input_price 单价 × image_count（如生图按张计费）
		field.String("billing_mode").
			MaxLen(20).
			Default("token"),

		// 单价（每 token 或每次/张价格，根据 billing_mode 解释）
		field.Float("input_price").
			Default(0).
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),

		field.Float("output_price").
			Default(0).
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),

		field.Float("cache_creation_price").
			Default(0).
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),

		field.Float("cache_read_price").
			Default(0).
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),

		// 货币单位。默认 USD；运营 BI 报表自行换算
		field.String("currency").
			MaxLen(8).
			Default("USD"),

		// 生效区间。effective_to 为 NULL 表示"持续生效"
		// FindEffective(provider, model, now) 命中规则：
		//   effective_from <= now AND (effective_to IS NULL OR now < effective_to)
		field.Time("effective_from").
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),

		field.Time("effective_to").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),

		// 单价来源标签。manual = 运营手工录入；litellm_sync = 后续 LiteLLM 同步（仅提示，不自动写）；
		// upstream_billing_api = 未来接上游账单 API（Phase 0 之外的独立工程）
		field.String("source").
			MaxLen(50).
			Default("manual"),

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

func (ProviderPricing) Indexes() []ent.Index {
	return []ent.Index{
		// 命中查询：给定 (provider, model, now()) 找当前生效价格
		index.Fields("provider", "model", "effective_from"),
	}
}
