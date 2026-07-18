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

// BalanceSnapshot 用户余额月结快照（可重算 rollup）：一行 = 单用户单月，自包含
// 期初/期末余额 + 四类资金流水汇总，用于月度对账单（Vendor Report）历史月的精确
// 期初/期末，并可在单行内校验恒等式：
//
//	closing_balance = opening_balance + deposit_total - withdraw_total
//	                  + credit_total - utilisation_total
//
// 删除策略：物理删/可重算、不建 users 外键（与 TeamFundTransfer 台账同理——对账
// 记录须在用户被物理删除后保留追溯）。表内只落精确月结快照（月结定时任务或管理员
// 手动重算），查不到某月行时由 service 层实时反推兜底、不落库。
//
// zhiguofan fork-only: 月度对账（Vendor Report）。
type BalanceSnapshot struct {
	ent.Schema
}

func (BalanceSnapshot) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "balance_snapshots"},
	}
}

func (BalanceSnapshot) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		// 月份桶，格式 YYYY-MM（如 2026-05）；按字典序即时间序，跨年正确。
		field.String("period").
			MaxLen(7),
		field.Float("opening_balance").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Float("closing_balance").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Float("deposit_total").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		// 提现/退款/企业回收合计，存正数量级，方向由恒等式中的减号表达。
		field.Float("withdraw_total").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Float("credit_total").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		// 消费折前合计 Σtotal_cost（倍率前标准计费），对账单折扣列用。
		field.Float("utilisation_gross_total").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}).
			Default(0),
		// 消费折后合计 Σactual_cost（实际扣款，存正数量级）。
		field.Float("utilisation_total").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}).
			Default(0),
		field.Time("computed_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (BalanceSnapshot) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "period").Unique(),
		index.Fields("period"),
	}
}
