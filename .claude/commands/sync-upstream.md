# /sync-upstream — 上游同步风险分析

在执行任何合并操作前，强制读取 fork 功能列表并对比上游差异，输出风险分析报告，为后续合并提供决策支持。**本命令只做分析，不执行任何 git 操作。**

## 三分支同步模型

```
upstream/main
      │
      ▼  fast-forward（无冲突，main 不含 fork 功能）
    main
      │
      ▼  merge（冲突主战场：fork 自定义功能 vs 上游变更）
  zhiguofan              ← 稳定发布分支，含所有 fork 功能
      │
      ▼  merge（继承 zhiguofan 的冲突解决，额外冲突少）
feature/maas-refactor    ← 活跃开发分支，含 MAAS P0-P5 重构
```

**每次同步必须按序完成三步，不能跳过中间分支。**
- `main`：永远对齐上游，禁止混入 fork 功能
- `zhiguofan`：fork 功能稳定基线，冲突在此解决
- `feature/maas-refactor`：从 `zhiguofan` 继承已解决的冲突，不直接从 `main`/`upstream` 合并

---

## 执行步骤

### Step 1：读取 fork 功能列表（强制）

首先完整读取 `自定义开发功能列表.md`，这是判断合并风险的唯一依据。在完成阅读之前不得进行任何 diff 分析。

### Step 2：获取三个分支的当前状态

```bash
git fetch upstream --quiet

# 版本号快照
cat backend/cmd/server/VERSION                          # 当前工作分支版本
git show main:backend/cmd/server/VERSION                # main 版本
git show zhiguofan:backend/cmd/server/VERSION           # zhiguofan 版本
git show upstream/main:backend/cmd/server/VERSION       # 上游最新版本

# main 落后上游几个提交
git log --oneline main..upstream/main

# zhiguofan 落后上游几个提交（待合并内容，冲突分析的主体）
git log --oneline zhiguofan..upstream/main

# maas-refactor 与 zhiguofan 的相对状态
git log --oneline feature/maas-refactor..zhiguofan      # zhiguofan 有而 maas 没有的提交
git log --oneline zhiguofan..feature/maas-refactor      # maas 独有的提交（MAAS 功能）

# 上游变更文件列表（用于冲突矩阵，以 zhiguofan 为基准）
git diff zhiguofan..upstream/main --name-only
git diff zhiguofan..upstream/main --stat
```

### Step 3：高风险文件交叉比对

以 `zhiguofan` 为基准，将上游变更文件与以下高风险清单做交叉：

**🔴 高风险（fork 自定义 + 上游都改）**
- `backend/cmd/server/wire_gen.go`
- `backend/internal/server/routes/admin.go`
- `backend/internal/server/routes/gateway.go`
- `backend/internal/handler/admin/setting_handler.go`
- `backend/internal/service/billing_service.go`
- `backend/internal/service/pricing_service.go`
- `backend/internal/config/config.go`
- `backend/ent/schema/usage_log.go`
- `frontend/src/router/index.ts`
- `frontend/src/i18n/locales/zh.ts` / `en.ts`
- `.github/workflows/*.yml`

**🟡 中风险（fork 改过，上游偶尔改）**
- `backend/internal/service/gateway_service.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/scheduler_snapshot_service.go`
- `frontend/src/views/user/ModelsView.vue`
- `frontend/src/types/index.ts`
- `frontend/src/utils/platformColors.ts`

如果 `feature/maas-refactor` 与 `zhiguofan` 有分歧，还需额外检查 MAAS 专属高风险文件：
- `backend/internal/domain/protocol.go`
- `backend/internal/server/routes/gateway.go`（协议分流 10+ 处）
- `backend/internal/service/upstream_cost.go`

### Step 4：输出风险分析报告

按以下格式输出报告，不要省略任何一节：

---

## 上游同步风险分析报告

**分析时间**：[当前时间]

| 分支 | 当前版本 | 落后上游提交数 | 状态 |
|------|---------|--------------|------|
| `upstream/main` | [版本] | — | 基准 |
| `main` | [版本] | [数量，0=已同步] | ✅/⚠️ |
| `zhiguofan` | [版本] | [数量] | ✅/⚠️ |
| `feature/maas-refactor` | [版本] | [相对 zhiguofan 的状态] | ✅/⚠️ |

### 待合并的上游提交摘要（以 zhiguofan 为基准）

[列出 upstream/main 领先 zhiguofan 的提交，每条一行，标注是否可能影响 fork 功能]

### zhiguofan 合并的高风险文件冲突矩阵

| 高风险文件 | 上游是否修改 | 涉及的 fork 功能 | 风险等级 |
|-----------|------------|----------------|---------|
| ... | 是/否 | ... | 🔴/🟡/🟢 |

### maas-refactor 额外分歧分析

说明 `feature/maas-refactor` 与 `zhiguofan` 的差异：
- `zhiguofan` 有而 `maas-refactor` 没有的提交（是否需要先从 zhiguofan 补入）
- `maas-refactor` 独有的 MAAS 功能是否与上游变更产生二次冲突

### fork 功能逐项风险评估

对功能列表中每个 fork 功能，说明：
- 上游变更是否触及相关文件
- 如果触及，具体哪些函数/结构体/路由需要在合并后手动恢复

### 合并建议

**整体风险评级**：🔴 高 / 🟡 中 / 🟢 低

**建议操作顺序（三步必须按序执行）**：

**第一步 — `main` 同步上游**（无冲突，fast-forward）
- [ ] `git checkout main && git merge upstream/main --ff-only`
- [ ] `git push origin main`
- 验证：`git show main:backend/cmd/server/VERSION` 应与上游一致

**第二步 — `zhiguofan` 合并 `main`**（冲突主战场）
- [ ] 冲突少，可尝试 `git checkout zhiguofan && git merge main --no-edit`
- [ ] 冲突多，需手动合并，以下文件须逐行审查：[列出文件]
- [ ] 合并后检查 `backend/cmd/server/VERSION` 主号是否为 `1`
- [ ] 合并后检查所有 `.github/workflows/*.yml` 的 `on:` 是否为 `workflow_dispatch:`
- 验证：`cd backend && go build ./cmd/server/ && go test -tags=unit ./internal/service ./internal/handler/...`

**第三步 — `feature/maas-refactor` 合并 `zhiguofan`**（继承已解决的冲突）
- [ ] `git checkout feature/maas-refactor && git merge zhiguofan`
- [ ] MAAS 专属文件（`gateway.go` 协议分流、`wire_gen.go` MAAS 注入点）需二次确认
- 验证：
```bash
grep -q 'promptAnalytics' backend/internal/server/routes/gateway.go && echo OK || echo MISSING
grep -q '/lingjing/v1' backend/internal/server/routes/gateway.go && echo OK || echo MISSING
grep -q 'getGroupInboundProtocol' backend/internal/server/routes/gateway.go && echo OK || echo MISSING
grep -q 'ProtocolBucketEnabled' backend/internal/config/config.go && echo OK || echo MISSING
grep -q 'upstream_total_cost' backend/ent/schema/usage_log.go && echo OK || echo MISSING
cd backend && go build ./cmd/server/
cd frontend && pnpm exec vitest run src/views/user/__tests__/ModelsView.spec.ts src/i18n/__tests__/usageServiceTierLocales.spec.ts
```

---

完成报告后，等待用户确认是否继续执行实际的合并操作。不要自动运行任何 git 命令。
