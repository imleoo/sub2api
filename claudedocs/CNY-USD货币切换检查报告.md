# CNY/USD 货币切换检查报告

检查时间：2026-05-10

## 结论

后台的货币设置保存链路基本完整：管理员在设置页选择 `usd/cny` 和 `cny_rate` 后，会通过后台设置接口保存，并通过公共设置下发到前端全局 `appStore`。

但 CNY 切换“不完整”是真实存在的。问题主要不在后端设置保存，而在前端后台页面和统计组件里仍有大量金额展示硬编码 `$` 或 `(USD)`，同时部分组件使用本地 `formatCost()` 只格式化数字、不读取 `appStore.currencyMode/cnyRate`。因此切到 CNY 后，部分后台页面会继续显示美元符号或美元单位。

## 已验证完整的链路

1. 管理员设置页提供货币模式和汇率输入：
   - `frontend/src/views/admin/SettingsView.vue:5167-5189`
   - `frontend/src/views/admin/SettingsView.vue:7674-7675`

2. 后端更新接口接收 `currency_mode/cny_rate`：
   - `backend/internal/handler/admin/setting_handler.go:580-581`
   - `backend/internal/handler/admin/setting_handler.go:1525-1531`

3. 后端设置服务保存并读取公共设置：
   - `backend/internal/service/setting_service.go:684-687`
   - `backend/internal/service/setting_service.go:743-744`
   - `backend/internal/service/setting_service.go:1621-1623`
   - `backend/internal/service/setting_service.go:2793-2798`

4. 前端全局 store 会接收公共设置：
   - `frontend/src/stores/app.ts:293-307`

5. 项目已有可复用的货币模式感知工具：
   - `frontend/src/utils/format.ts:13-24` 的 `formatUSD()`
   - `frontend/src/utils/format.ts:79-102` 的 `formatCurrency()`

## 主要问题

### 1. 账号统计弹窗没有跟随 CNY

文件：
- `frontend/src/components/admin/account/AccountStatsModal.vue`
- `frontend/src/components/account/AccountStatsModal.vue`

证据：
- `frontend/src/components/admin/account/AccountStatsModal.vue:60-67`：总费用、用户计费、标准费用硬编码 `$`
- `frontend/src/components/admin/account/AccountStatsModal.vue:110-119`：日均费用硬编码 `$`
- `frontend/src/components/admin/account/AccountStatsModal.vue:173-179`：今日费用硬编码 `$`
- `frontend/src/components/admin/account/AccountStatsModal.vue:517-527`：图表 label 固定 `(USD)`
- `frontend/src/components/admin/account/AccountStatsModal.vue:574-575`：tooltip 固定 `$`
- `frontend/src/components/admin/account/AccountStatsModal.vue:608-612`：Y 轴固定 `$` 和 `(USD)`
- `frontend/src/components/admin/account/AccountStatsModal.vue:676-684`：本地 `formatCost()` 只返回数字，不做 CNY 换算

同类问题在 `frontend/src/components/account/AccountStatsModal.vue:72-79`、`frontend/src/components/account/AccountStatsModal.vue:553-648`、`frontend/src/components/account/AccountStatsModal.vue:712-720` 也存在。

影响：后台查看账号统计时，即使系统切到 CNY，弹窗里的卡片、折线图、tooltip、Y 轴仍按 USD 文案和 `$` 展示。

### 2. 用户统计弹窗没有跟随 CNY

文件：
- `frontend/src/components/admin/user/UserStatsModal.vue`

证据：
- `frontend/src/components/admin/user/UserStatsModal.vue:60-67`：总费用区域硬编码 `$`
- `frontend/src/components/admin/user/UserStatsModal.vue:106-115`：日均费用硬编码 `$`
- `frontend/src/components/admin/user/UserStatsModal.vue:169-213`：今日/最高费用硬编码 `$`
- `frontend/src/components/admin/user/UserStatsModal.vue:487`：图表 label 固定 `(USD)`
- `frontend/src/components/admin/user/UserStatsModal.vue:532-533`：tooltip 固定 `$`
- `frontend/src/components/admin/user/UserStatsModal.vue:558-562`：Y 轴固定 `$` 和 `(USD)`
- `frontend/src/components/admin/user/UserStatsModal.vue:618-627`：本地 `formatCost()` 只返回数字，不做 CNY 换算

影响：后台用户详情的消费统计仍显示 USD。

### 3. 后台统计图表组件没有跟随 CNY

文件：
- `frontend/src/components/charts/ModelDistributionChart.vue`
- `frontend/src/components/charts/EndpointDistributionChart.vue`
- `frontend/src/components/charts/GroupDistributionChart.vue`

证据：
- `frontend/src/components/charts/ModelDistributionChart.vue:143-150`：模型分布表格硬编码 `$`
- `frontend/src/components/charts/ModelDistributionChart.vue:443-466`：图表 tooltip 硬编码 `$`
- `frontend/src/components/charts/EndpointDistributionChart.vue:108-112`：端点分布表格硬编码 `$`
- `frontend/src/components/charts/EndpointDistributionChart.vue:268-270`：图表 tooltip 硬编码 `$`
- `frontend/src/components/charts/GroupDistributionChart.vue:77-83`：分组分布表格硬编码 `$`
- `frontend/src/components/charts/GroupDistributionChart.vue:219-220`：图表 tooltip 硬编码 `$`

影响：后台 Dashboard / Usage 里按模型、端点、分组查看消费分布时，CNY 模式不会完整生效。

### 4. 分组列表的用量摘要没有跟随 CNY

文件：
- `frontend/src/views/admin/GroupsView.vue`

证据：
- `frontend/src/views/admin/GroupsView.vue:263-276`：今日/累计用量硬编码 `$`
- `frontend/src/views/admin/GroupsView.vue:3546-3550`：本地 `formatCost()` 只返回数字，不做 CNY 换算

影响：后台分组列表的用量摘要仍显示美元。

### 5. 账号容量和窗口费用组件没有跟随 CNY

文件：
- `frontend/src/components/account/AccountCapacityCell.vue`
- `frontend/src/components/account/UsageProgressBar.vue`

证据：
- `frontend/src/components/account/AccountCapacityCell.vue:11`：5h 窗口费用限制固定 `'$' + formatCost(...)`
- `frontend/src/components/account/UsageProgressBar.vue:15-23`：窗口费用 A/U 展示固定 `$`
- `frontend/src/components/account/UsageProgressBar.vue:186-194`：费用值只 `toFixed(2)`，不做 CNY 换算

影响：账号列表/账号卡片中的窗口费用限制和窗口消费仍显示 USD。

### 6. 模型折扣价格页仍固定 USD

文件：
- `frontend/src/views/admin/ModelDiscountsView.vue`

证据：
- `frontend/src/views/admin/ModelDiscountsView.vue:92-95`：`formatPrice()` 固定返回 `$.../MTok`

影响：后台模型折扣页的模型单价不会跟随 CNY。

## 已经正确使用 CNY 感知格式化的后台页面

以下位置已经使用 `formatUSD()`，切换 CNY 时会按 `appStore.cnyRate` 展示：

- `frontend/src/views/admin/UsersView.vue:406`
- `frontend/src/views/admin/UsersView.vue:429-435`
- `frontend/src/views/admin/RedeemView.vue:102`
- `frontend/src/views/admin/PromoCodesView.vue:79`
- `frontend/src/views/admin/PromoCodesView.vue:350`

这说明修复方向不需要重做货币体系，主要是把遗漏页面从硬编码 `$` 迁移到已有 `formatUSD()` / `formatCurrency()`。

## 根因判断

根因是前端金额格式化不统一：

- 一部分页面已经接入 `formatUSD()`，能自动根据 `appStore.currencyMode` 显示 `$` 或 `¥`。
- 另一部分后台统计组件早期使用了局部 `formatCost()`，模板层再手动拼接 `$`。
- 图表 label、tooltip、Y 轴标题使用了固定 `(USD)` 文案，没有货币模式分支。

后端当前仍以 USD 作为存储和计费基准，这一点从工具函数注释也能确认：`frontend/src/utils/format.ts:9-11` 明确说明“存储值为 USD，CNY 模式按汇率换算”。因此修复应优先限定在展示层，不应改数据库字段或计费逻辑。

## 建议修复范围

1. 新增或复用统一 helper：
   - 优先复用 `formatUSD(amount, digits)`。
   - 对图表 label 可补一个 `currencyLabel()`，返回 `CNY` 或 `USD`。
   - 对图表轴和 tooltip 使用同一个金额格式化函数，避免 `$` 与 `¥` 混用。

2. 按页面分批修：
   - 第一批：`AccountStatsModal.vue`、`UserStatsModal.vue`、三个 `components/charts/*DistributionChart.vue`。
   - 第二批：`GroupsView.vue`、`AccountCapacityCell.vue`、`UsageProgressBar.vue`、`ModelDiscountsView.vue`。

3. 加前端测试：
   - 在 CNY store 状态下断言金额包含 `¥`，不包含 `$`。
   - 覆盖至少 `formatUSD()`、用户/账号统计弹窗、分布图 tooltip formatter。

## 验证建议

修复后至少执行：

```bash
cd frontend && pnpm exec vitest run src/views/admin/__tests__/SettingsView.spec.ts src/views/admin/__tests__/UsersView.spec.ts
cd frontend && pnpm run lint:check
```

如果改到图表组件，建议补跑对应图表测试或新增测试后运行：

```bash
cd frontend && pnpm exec vitest run src/components/charts
```

## 修复执行记录

更新时间：2026-05-10

已完成修复：

- 第一批报告内问题：账号统计、用户统计、模型/端点/分组分布图、分组列表用量摘要、账号容量/窗口费用、模型折扣价格页。
- 追加反馈问题：用户充值、并发变动记录、充值/订单展示、编辑分组、渠道管理定价、账号管理窗口成本、兑换码管理、优惠码管理、使用记录、兑换码最近活动。
- 追加补漏：用户端 Dashboard 的模型分布表格和最近使用记录金额展示。
- 导出补漏：用户端使用记录 CSV 导出的费用表头和值、后台使用记录 Excel 导出的费用表头和值。
- 表单输入规则：后端仍以 USD 存储和计费；CNY 模式下，分组金额输入、渠道价格输入、账号窗口成本输入显示 CNY，输入后保存前换算回 USD。

验证结果：

```bash
git diff --check
cd frontend && pnpm run typecheck
cd frontend && pnpm exec vitest run src/utils/__tests__/formatCurrency.spec.ts src/components/charts/__tests__/ModelDistributionChart.spec.ts src/components/charts/__tests__/GroupDistributionChart.spec.ts src/components/charts/__tests__/EndpointDistributionChart.spec.ts src/components/account/__tests__/UsageProgressBar.spec.ts src/components/admin/usage/__tests__/UsageTable.spec.ts src/views/user/__tests__/PaymentView.spec.ts src/components/payment/__tests__/PaymentStatusPanel.spec.ts src/views/admin/__tests__/SettingsView.spec.ts src/views/admin/__tests__/UsersView.spec.ts
cd frontend && pnpm exec vitest run src/components/user/dashboard/__tests__/UserDashboardCharts.spec.ts src/views/user/__tests__/UsageView.spec.ts
cd frontend && pnpm exec vitest run src/views/user/__tests__/UsageView.spec.ts
cd frontend && pnpm run lint:check
```

以上检查均通过。`lint:check` 仍有一个既有 warning：`frontend/src/api/__tests__/gateway_model_calls.spec.ts:611` 的 `client` 未使用，本次未改该文件。
