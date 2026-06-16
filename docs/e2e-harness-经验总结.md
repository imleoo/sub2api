# 自包含 E2E Harness 工程经验总结

> 沉淀自 2026-06 一次「为逆向清理做真实转发验证 → 沉淀为可复跑全功能 e2e」的实战。
> 相关产物：`script/e2e-test.sh`、`script/e2e.env.example`、
> `backend/internal/integration/e2e_full_test.go`、`e2e_full_provision_test.go`。
> 本文记录**可复用的模式**与**踩过的坑**，供后续自学与复用。

---

## 1. 核心架构模式：自包含 + 灰盒

一个「真·端到端」测试套件要回答两个问题：**环境从哪来**、**断言怎么写**。

### 1.1 自包含 harness 四步骨架

```
boot（起服务）→ seed（造数据）→ run（跑测试）→ teardown（拆环境，必须 trap）
```

- **boot**：复用项目已有的本地起服务脚本（本项目 `script/dev_local.sh`，连本机 Docker PG/Redis）。harness 只负责拉起 + 等 `/health`，不自己拼装服务。
- **teardown 用 `trap cleanup EXIT`**：无论测试通过/失败/中断，都要拆环境。只在「确实是自己起的」时才拆（用一个 `BOOTED_SERVER` 标志位区分「自起」与「连已有实例」两种模式）。
- **双模式**：`E2E_BASE_URL` 已设 → 连已运行实例、不起不拆；未设 → 自起自拆。一个脚本覆盖「本地快速调试」和「CI 全自动」。

### 1.2 灰盒：控制面 + 数据面一起用

真实业务 e2e 不是纯黑盒。要同时驱动两个面：

- **控制面（admin/user API）**：seed 账号/分组/网关 key、建用户设余额、查余额/用量。
- **数据面（网关）**：穿过被测网关发真实请求（`/v1/messages`、`/v1/chat/completions`、`/v1beta/models/{m}:generateContent`）。

断言 = 数据面行为（转发成功/被拦截）+ 控制面状态变化（余额扣减/用量入账）。

### 1.3 provisioning 放 Go，不放 bash

- **bash 只做编排**（起服务 / 注入 env / 跑 `go test` / 拆环境）。
- **复杂造数据放 Go**（typed helper：建账号/分组/key、建用户设余额、轮询余额变化）。
  bash + curl + python 拼 JSON 造「带特定余额/配额/限流的用户」既脆又难维护。
- Go 侧用 `sync.Once` 做**一次性共享 provisioning**（admin 登录 + 每平台一套账号/分组/key），
  各测试用 `requireProvision(t)` 懒取；环境不全则 `t.Skip` 整组。

---

## 2. Shell / Bash 踩坑（高频、隐蔽）

### 2.1 `${VAR:-default}` vs `${VAR-default}`（冒号之差）

想用 `E2E_RUN=""` 表示「跑全部、不加 -run 过滤」，结果空串仍回落默认值：

```bash
RUN_FILTER="${E2E_RUN:-TestE2EFull}"   # ❌ 冒号版：空串也触发默认 → 永远跑不了全部
RUN_FILTER="${E2E_RUN-TestE2EFull}"    # ✅ 无冒号：只有「未设」才默认，「显式空」保留为空
```

**规则**：`:-` 把「未设」**和**「空串」都当默认；`-`（无冒号）只把「未设」当默认。
需要区分「没传」与「传了空」时，去掉冒号。

### 2.2 裸 `$VAR` 紧贴中文/多字节字符 → "unbound variable"

`set -u` + `C.UTF-8` locale 下，`echo "端口 $BACKEND_PORT，跳过"` 报
`BACKEND_PORT: unbound variable`——bash 把后续多字节字节并进了变量名。

```bash
echo "端口 $BACKEND_PORT，跳过"     # ❌ 裸变量贴全角逗号，解析错乱
echo "端口=${BACKEND_PORT} 跳过"    # ✅ 花括号定界 + ASCII 标点
```

**规则**：echo/字符串里凡变量与 CJK 相邻，一律 `${VAR}` 加花括号，并尽量用 ASCII 标点隔开。

### 2.3 `set -euo pipefail` + 管道里的 `$?` 抓的是最后一段

提交前扫凭证时差点误判：

```bash
git diff --cached | grep -i 'secret' | head; echo "leak=$?"   # ❌ $? 是 head 的退出码(永远0)
# ✅ 用显式 if，且别接 head：
if git diff --cached | grep -iE 'sk-[0-9a-f]{32}'; then echo "发现凭证!"; else echo "干净"; fi
```

### 2.4 自动加载本地 env 文件

让 harness 自动 source 一个 gitignored 的凭证文件，免去每次手敲：

```bash
ENV_FILE="${E2E_ENV_FILE:-$SCRIPT_DIR/e2e.env}"
if [[ -f "$ENV_FILE" ]]; then
  set -a            # 之后所有赋值自动 export
  source "$ENV_FILE"
  set +a
fi
```

---

## 3. 凭证管理（安全红线）

- **三件套**：gitignored 真凭证文件（`script/e2e.env`）+ 提交的模板（`script/e2e.env.example`，占位值）+ harness 自动加载。
- **`.gitignore` 路径陷阱**：本项目 `.gitignore` 有一条裸 `scripts`，它**匹配任意层级**叫 scripts 的目录——`backend/scripts/` 被忽略，但根级 `script/`（单数）没被忽略。
  教训：放「要提交的脚本」前先 `git check-ignore -v <path>` 确认没被忽略；放「不提交的凭证」前确认**确实**被忽略。
- **提交前必扫**：`git diff --cached | grep -iE '<真key前缀>'`，确认暂存区零凭证再提交。
- **build tag 隔离**：e2e 测试文件加 `//go:build e2e`，普通 `go build/vet/test` 不碰它们；单独 `go vet -tags=e2e ./...` 做编译门禁。

---

## 4. 测试设计经验（最有价值的一节）

### 4.1 上游不可控 → 容忍式 skip，而非 fail

第三方上游会 404/502/限流，这**不是你网关的 bug**。要把「我方故障」和「上游限制」分开：

```go
if st != 200 {
    if st == 502 || bodyContains(body, "Upstream") || bodyContains(body, "temporarily unavailable") {
        t.Skipf("上游暂不可用（网关已转发并回传上游响应 st=%d）", st)  // ⏭ 上游问题
    }
    t.Fatalf("期望 200，实际 %d", st)                                  // ❌ 真故障
}
```

本次实测：`count_tokens` 上游 404、`gemini` 上游 502——都改成 skip。
关键认知：**网关把上游的非 200 如实回传，恰恰证明网关转发链路是通的**（不是网关内部 500）。

### 4.2 模型映射：穿网关测，别信直连探测

直连 openclaw 上游测 `gemini-2.5-flash` 返回 404，但**穿过我们网关**用同一模型名却 200——
因为网关有模型映射/路径规范化。

**教训**：测的是「网关」，模型可用性以「穿网关的结果」为准，别拿直连上游的探测结论下判断。

### 4.3 限额/配额是「消费后检查」→ sendUntilBlocked 模式

quota/rate-limit 的语义通常是 `used >= limit 才拒`。所以**第一次必过**（used=0），消费后第二次才被拦。
且计费可能**异步落账**，两次请求间要等账落定：

```go
func sendUntilBlocked(send func() (int, []byte), maxTries int) bool {
    for i := 0; i < maxTries; i++ {
        if st, _ := send(); st != 200 { return true }   // 被拦 → 成功验证
        time.Sleep(2 * time.Second)                     // 等异步计费落账，推进计数
    }
    return false
}
```

### 4.4 阈值语义要精确 + 验证前置条件

余额预检是 `Balance <= 0`（不是 `< 0`）。所以构造「余额不足」要把余额设成**恰好 0**；
且环境可能发注册赠额，断言前先确认前置成立、不成立就 skip：

```go
uid, _ := createUser(adminToken, email, pw, 0)          // 余额=0
if bal, _ := profileBalance(userTok); bal > 0 {
    t.Skipf("环境发了注册赠额 bal=%.8f，无法构造余额不足", bal)
}
// 然后断言请求被 403 拒绝
```

### 4.5 枚举合法值从 handler request struct 读

`status:"disabled"` 被 400 拒（`oneof` 校验失败）。正确值要去**源码**找：

```go
// internal/handler/api_key_handler.go
Status string `json:"status" binding:"omitempty,oneof=active inactive"`  // 合法值是 active|inactive
```

**教训**：传枚举/状态前，先 grep handler 的 `binding:"oneof=..."`，别猜。

### 4.6 计费断言：弱断言保稳健，对账做完整性

不要断言「精确成本」（依赖定价配置，脆）。分两层：

1. **行为弱断言**：可计费请求后余额**严格下降**；count_tokens 后余额**不变**。轮询等异步落账。
2. **对账强校验**（完整性）：`SUM(所有 usage_log.actual_cost) == 初始余额 - 当前余额`，逐分钱对得上，
   证明「记账 → 算成本 → 扣余额」全链路闭环无漂移。

### 4.7 优雅 skip 哲学（CI 友好）

「环境不全」(无 admin / 无上游 key) → `t.Skip`，不是 fail。
Go 把 SKIP 不算失败，套件整体仍 `ok`。这让同一套 e2e 在「有凭证的本地」跑全、在「无凭证的 CI」自动跳过。
**区分**：skip = 无法构造场景 / 上游限制；fail = 我方代码坏了。

---

## 5. 复用清单（下次搭类似 harness 照着走）

1. [ ] harness：`set -euo pipefail` + `trap cleanup EXIT` + `BOOTED_SERVER` 标志位 + 双模式（`E2E_BASE_URL`）
2. [ ] 自动加载 gitignored env：`set -a; source $ENV_FILE; set +a`；配 `.example` 模板
3. [ ] `git check-ignore -v` 确认凭证文件被忽略、脚本文件没被忽略
4. [ ] provisioning 放 Go（`sync.Once` 共享 + `requireXxx(t)` 懒取 + 环境不全 skip）
5. [ ] 枚举值去 handler `binding:"oneof="` 找，别猜
6. [ ] 上游非 200 区分 skip(上游) / fail(我方)
7. [ ] 限额测试用 sendUntilBlocked + 等异步落账
8. [ ] 计费用「弱行为断言 + 对账强校验」两层
9. [ ] build tag 隔离 + 单独 `go vet -tags=<tag>` 做编译门禁
10. [ ] 提交前 `git diff --cached | grep <key前缀>` 扫凭证；真 key 在对话/磁盘出现过则提醒轮换

---

## 6. 一句话心法

> **环境要自包含、凭证要 env 化、上游要容忍、断言要分层、提交前要扫密。**
