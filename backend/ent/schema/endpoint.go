package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Endpoint 表示一个具体可请求的上游入口（base_url + path + auth）。
// Phase 5 P5-1：Account 1→N Endpoint 关系；lingjing 账号不参与迁移。
// 详见 docs/glossary.md §4、docs/generic-channel-design.md §6。
type Endpoint struct {
	ent.Schema
}

func (Endpoint) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "endpoints"},
	}
}

func (Endpoint) Fields() []ent.Field {
	return []ent.Field{
		// 所属账号
		field.Int64("account_id"),

		// stable_id: 应用层生成的稳定标识符，用于 UsageLog.endpoint_id 快照。
		// 格式建议：<provider>-<protocol>，如 wanjie-openai_chat。
		// 一旦写入不变更（历史快照依赖此字段）。
		field.String("stable_id").
			MaxLen(128).
			Comment("应用层稳定标识符，UsageLog 快照使用"),

		// outbound_protocol: 本 endpoint 的出站协议能力。
		// 取值见 docs/glossary.md §1.1：anthropic_messages / openai_chat / openai_responses / gemini_v1beta
		field.String("outbound_protocol").
			MaxLen(50).
			Comment("出站协议，取值见 glossary.md §1.1"),

		// base_url: 上游 endpoint 的根地址，不含尾部斜杠。
		field.String("base_url").
			MaxLen(512),

		// auth_header / auth_scheme: HTTP 鉴权配置。
		// auth_header 典型值：Authorization（Bearer token）/ x-api-key（Anthropic）
		field.String("auth_header").
			MaxLen(64).
			Default("Authorization"),
		field.String("auth_scheme").
			MaxLen(32).
			Default("Bearer"),

		// models_source: 模型列表来源策略。
		// remote = 调用 /models endpoint；manual = 前端手填；static_preset = 内置预设
		field.String("models_source").
			MaxLen(20).
			Default("remote").
			Comment("remote / manual / static_preset"),

		// priority: 调度优先级，数值越小越优先。
		field.Int("priority").
			Default(100),

		// health: 当前健康状态（由监控服务更新）。
		// healthy / degraded / disabled
		field.String("health").
			MaxLen(20).
			Default("healthy").
			Comment("healthy / degraded / disabled"),

		// capabilities: JSON 数组，记录该 endpoint 能力标签。
		// 与 docs/glossary.md §3.3 RequestFeatures 对应，用于调度过滤。
		field.JSON("capabilities", []string{}).
			Optional(),

		// 时间戳
		field.Time("created_at").
			Immutable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (Endpoint) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("account", Account.Type).
			Ref("endpoints").
			Field("account_id").
			Required().
			Unique(),
	}
}

func (Endpoint) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("account_id"),
		index.Fields("account_id", "stable_id").Unique(),
		index.Fields("outbound_protocol", "health"),
	}
}
