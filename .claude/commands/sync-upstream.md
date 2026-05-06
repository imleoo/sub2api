# /sync-upstream — 上游同步风险分析

在执行任何合并操作前，强制读取 fork 功能列表并对比上游差异，输出风险分析报告，为后续合并提供决策支持。**本命令只做分析，不执行任何 git 操作。**

## 执行步骤

### Step 1：读取 fork 功能列表（强制）

首先完整读取 `claudedocs/自定义开发功能列表.md`，这是判断合并风险的唯一依据。在完成阅读之前不得进行任何 diff 分析。

### Step 2：获取上游最新状态

运行以下命令获取当前 git 状态和上游差异：

```bash
git fetch upstream --quiet
git log --oneline upstream/main..HEAD          # fork 领先上游的提交
git log --oneline HEAD..upstream/main          # 上游领先 fork 的提交（待合并内容）
git diff upstream/main...HEAD --stat           # 所有差异文件统计
```

### Step 3：对每个高风险文件做交叉比对

对照功能列表中"快速风险评估表"的🔴高风险文件，逐一检查上游的待合并变更是否触及这些文件：

```bash
git diff HEAD..upstream/main --name-only       # 上游新增/修改的文件列表
```

将上游变更文件与以下高风险文件清单做交叉：
- `backend/cmd/server/wire_gen.go`
- `backend/internal/server/routes/admin.go`
- `backend/internal/server/routes/gateway.go`
- `frontend/src/router/index.ts`
- `backend/internal/service/billing_service.go`
- `backend/internal/service/pricing_service.go`
- `.github/workflows/*.yml`
- `frontend/src/i18n/locales/zh.ts` / `en.ts`
- `frontend/src/views/user/ModelsView.vue`
- `backend/internal/handler/admin/setting_handler.go`

### Step 4：输出风险分析报告

按以下格式输出报告，不要省略任何一节：

---

## 上游同步风险分析报告

**分析时间**：[当前时间]
**当前 fork 版本**：[读取 backend/cmd/server/VERSION]
**上游待合并提交数**：[数量]

### 待合并的上游提交摘要
[列出 upstream/main 领先的提交，每条一行，标注是否可能影响 fork 功能]

### 高风险文件冲突矩阵

| 高风险文件 | 上游是否修改 | 涉及的 fork 功能 | 风险等级 |
|-----------|------------|----------------|---------|
| ... | 是/否 | ... | 🔴/🟡/🟢 |

### fork 功能逐项风险评估

对功能列表中每个功能（模型广场、用户统计、词云、折扣、UI主题、语言切换、Masking、Workflows），说明：
- 上游变更是否触及相关文件
- 如果触及，具体哪些函数/结构体/路由需要在合并后手动恢复

### 合并建议

**整体风险评级**：🔴 高 / 🟡 中 / 🟢 低

**建议操作**：
- [ ] 可以直接运行 `./script/sync_upstream_to_zhiguofan.sh` 自动合并
- [ ] 需要手动合并，以下文件需要逐行审查：[列出文件]
- [ ] 建议暂缓合并，原因：[说明]

**合并后必须验证**：
```bash
# 根据本次实际影响到的功能，列出需要运行的最小验证集
```

---

完成报告后，等待用户确认是否继续执行实际的合并操作。不要自动运行 `sync_upstream_to_zhiguofan.sh`。
