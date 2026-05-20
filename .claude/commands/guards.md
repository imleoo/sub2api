# /guards — Fork 守护检查

运行 `script/check_fork12_guards.sh`，验证所有 fork 自定义功能未被上游同步或重构误删。

在以下场景前后使用：
- 合并上游（`/sync-upstream` 分析后实际 merge 前后）
- 发布前（`/release-zhiguofan` 内部已集成，也可单独运行）
- 重构 service / routes / ent schema 后

---

## 执行步骤

### Step 1：运行守护脚本

```bash
cd /Users/leoobai/jiwu-project/SubPanel && bash script/check_fork12_guards.sh
```

### Step 2：额外关键字检查

```bash
grep -q 'ProtocolBucketEnabled' backend/internal/config/config.go \
  && echo "[PASS] P5 回滚开关 ProtocolBucketEnabled" \
  || echo "[FAIL] MISSING: ProtocolBucketEnabled in config.go"

grep -A2 "^on:" .github/workflows/*.yml 2>/dev/null \
  | grep -v "workflow_dispatch" | grep -v "^--$" | grep -v "^$" \
  && echo "[WARN] 发现非 workflow_dispatch 触发器，请检查 .github/workflows/*.yml" \
  || echo "[PASS] GitHub Workflows 触发器安全"
```

### Step 3：输出汇总

- 全部 `[PASS]` → 守护通过，可继续操作
- 出现 `[FAIL]` → 列出失败项，说明对应的 fork 功能点（P0–P5 / fork 编号），提示如何修复：
  - 通常是上游同步引入的回退，需要手动从 git history 恢复对应代码
  - 提供 `git log --all --oneline -- <file>` 命令帮助定位

---

## 守护规则速查

| 规则 | 文件 | Pattern | Fork |
|------|------|---------|------|
| promptAnalytics 挂载 ≥7 | routes/gateway.go | `promptAnalytics` | 4 |
| /lingjing/v1 路由组 | routes/gateway.go | `/lingjing/v1` | 12 |
| video/submit 端点 | routes/gateway.go | `video/submit` | 12 |
| ForcePlatform ≥3 处 | routes/gateway.go | `ForcePlatform\(` | 12 |
| LingjingPollRunner | service/lingjing_poll_runner.go | `LingjingPollRunner` | 12 |
| LingjingGatewayService | service/lingjing_gateway_service.go | `LingjingGatewayService` | 12 |
| LingjingTaskRepository | service/lingjing_task_port.go | `LingjingTaskRepository` | 12 |
| applyDiscount | service/billing_service.go | `applyDiscount` | 5 |
| loadDiscounts | service/pricing_service.go | `loadDiscounts` | 5 |
| IsResponseMaskingEnabled | service/account.go | `IsResponseMaskingEnabled` | 8 |
| isIdentityQuestion | service/gateway_response_masking.go | `isIdentityQuestion` | 8 |
| PlatformLingjing | domain/constants.go | `PlatformLingjing` | 12 |
| upstream_total_cost | ent/schema/usage_log.go | `upstream_total_cost` | P0-2 |
| async_task_id | ent/schema/usage_log.go | `async_task_id` | P0-2 |
| UpstreamCostResolver | service/upstream_cost.go | `UpstreamCostResolver` | P0-4 |
| NormalizeProvider | service/provider_normalize.go | `NormalizeProvider` | P1-1 |
| ProtocolAnthropicMessages | domain/protocol.go | `ProtocolAnthropicMessages` | P2-2 |
| ProtocolBucketEnabled | config/config.go | `ProtocolBucketEnabled` | P5 |
