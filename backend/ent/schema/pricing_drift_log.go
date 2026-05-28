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

// PricingDriftLog 记录 SSOT 重构期间 V1（pricingData + 家族 fuzzy）与
// V2（catalog + aliasIdx）两条 GetModelPricing 路径产生的价格差异。
//
// 用途：PR-5 影子比对期写入；PR-6 切流前要求 ≥ 72h 0 条新增；PR-8 删除。
type PricingDriftLog struct {
	ent.Schema
}

func (PricingDriftLog) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "pricing_drift_logs"},
	}
}

func (PricingDriftLog) Fields() []ent.Field {
	return []ent.Field{
		// 触发漂移的请求模型名（未归一化）
		field.String("model_id").
			MaxLen(200).
			NotEmpty(),

		// 漂移类型：missing_v1 / missing_v2 / price_mismatch
		// - missing_v1：V1 返回 nil 但 V2 命中
		// - missing_v2：V2 返回 nil 但 V1 命中
		// - price_mismatch：两边都命中但 input/output/cache 任一字段不一致
		field.String("drift_kind").
			MaxLen(32),

		// V1 路径解析后的价格快照（ModelPricing JSON），nil 用空 JSON
		field.JSON("v1_pricing", map[string]any{}).
			Optional(),

		// V2 路径解析后的价格快照（DBModelPricing 投影到 ModelPricing 后），nil 用空 JSON
		field.JSON("v2_pricing", map[string]any{}).
			Optional(),

		// 命中路径详情（catalog / alias / family_fuzzy / fallback）—— 调试用
		field.String("hit_path").
			MaxLen(64).
			Optional(),

		// 漂移发现时刻
		field.Time("occurred_at").
			Default(time.Now).
			Immutable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (PricingDriftLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("model_id"),
		index.Fields("drift_kind"),
		index.Fields("occurred_at"),
	}
}
