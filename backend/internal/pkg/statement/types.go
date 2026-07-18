// Package statement 定义月度对账单（Vendor Report）的共享数据结构，
// 供 service（拼装）、handler（JSON 响应）、xlsxreport（Excel 导出）三方复用，
// 避免导出层反向依赖 service。
//
// zhiguofan fork-only: 月度对账（Vendor Report）。
package statement

// 行类型（Nature），与 Eonreach 模板六种行一一对应。
const (
	NatureOpening     = "opening"
	NatureDeposit     = "deposit"
	NatureWithdraw    = "withdraw"
	NatureCredit      = "credit"
	NatureUtilisation = "utilisation"
	NatureClosing     = "closing"
)

// Row 对账单单行。金额字段一律正数量级，方向由 Nature 表达
// （与模板一致：Withdraw 在 Excel 中渲染为负数）。
type Row struct {
	Date   string `json:"date"`   // YYYY-MM-DD（按请求时区）
	Nature string `json:"nature"` // opening|deposit|withdraw|credit|utilisation|closing
	// Deposit/Withdraw/Credit 行
	Amount float64 `json:"amount,omitempty"`
	Note   string  `json:"note,omitempty"`
	// Utilisation 行（模板 A×B=C、C×(1−D)=I 列组）
	Qty          int64   `json:"qty,omitempty"`           // token 总量，负数（消耗）
	UnitPrice    float64 `json:"unit_price,omitempty"`    // 等效均价 = CostBefore/|Qty|（反算值）
	CostBefore   float64 `json:"cost_before,omitempty"`   // Σ total_cost（倍率前）
	DiscountRate float64 `json:"discount_rate,omitempty"` // 1 − CostAfter/CostBefore
	CostAfter    float64 `json:"cost_after,omitempty"`    // Σ actual_cost（实际扣款）
	// 逐行滚动余额（页面展示用；Excel 侧由活公式表达）
	RunningTotal float64 `json:"running_total"`
}

// Totals 六类合计与恒等式差额。
type Totals struct {
	Deposit           float64 `json:"deposit"`
	Withdraw          float64 `json:"withdraw"`
	Credit            float64 `json:"credit"`
	UtilisationBefore float64 `json:"utilisation_before"`
	UtilisationAfter  float64 `json:"utilisation_after"`
	// IdentityGap = closing − (opening + deposit − withdraw + credit − utilisationAfter)。
	// 非 0 说明存在未留痕的余额变动（如手工改余额）。
	IdentityGap float64 `json:"identity_gap"`
}

// Statement 单用户单月对账单。
type Statement struct {
	Period         string  `json:"period"` // YYYY-MM
	Timezone       string  `json:"timezone"`
	Source         string  `json:"source"` // snapshot=月结快照精确值 | computed=实时反推值
	Closed         bool    `json:"closed"` // false=当月未封账，数据仍在增长
	OpeningBalance float64 `json:"opening_balance"`
	ClosingBalance float64 `json:"closing_balance"`
	Rows           []Row   `json:"rows"`
	Totals         Totals  `json:"totals"`
	UserEmail      string  `json:"-"` // 导出文件名用，不进 JSON
}
