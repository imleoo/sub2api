# TODO

- [ ] 月度对账（Vendor Report）Utilisation 行按模型逐笔记录单价，替代当前按天聚合反算 Unit Price/Discount Rate 的口径。
  - 现状：`backend/internal/service/statement_service.go` 中 `UnitPrice = CostBefore/|Qty|`、`DiscountRate = 1 - CostAfter/CostBefore` 均为同日多模型混合后的加权反算值，非真实牌价（详见 `claudedocs/月度对账功能设计方案.md`）。
  - 目标：改为按模型分组，每个模型一行（或子行），单价直接取自 `model_pricings`，不再反推。
  - 影响面：`Row` 结构（`backend/internal/pkg/statement/types.go`）、Excel 导出模板 A×B=C 恒等式（`backend/internal/pkg/xlsxreport/vendor_report.go`）、前端对账单展示。
