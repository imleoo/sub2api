# Harness 工程实践：从逆向清理到 E2E 的闭环验证

> 沉淀自 2026-06 一段连续工程：**逆向清理后遗症 T1（删三类死代码）→ 真实转发验证 → 全功能 E2E 套件**。
> 表面是两件事，本质是同一件事的两半：**用 harness 把模糊目标变成可机械验证的闭环**。
> 逆向清理用的是「静态 harness」，E2E 是「动态 harness」，而二者的衔接点恰好是本框架最值得记住的一课。
>
> 关联产物：`claudedocs/逆向清理_后遗症收尾_follow-up.md`、`script/e2e-test.sh`、
> `backend/internal/integration/e2e_full_test.go`、`e2e_full_provision_test.go`。
> 哲学锚点：Karpathy「目标驱动执行：先定成功标准，循环直到验证通过」。

---

## 0. 这次工程的全貌（一条线）

1. **起点**：逆向订阅清理留下三类 inert 死代码（openai unused 孤儿、privacy schema 死功能、网关 `tokenType=="oauth"` 死分支）。
2. **删**：逐类删除，每一步靠编译器/linter 的反馈确认「没删错、删干净」。
3. **卡点**：类 1 改的是网关鉴权头**热路径**——`follow-up` 文档白纸黑字写「unit 测试兜不住转发行为正确性，必须真实 apikey/vertex 转发冒烟」。
4. **延伸**：于是搭了一套自包含 E2E，对真实上游验证转发 + 计费 + 限额闭环。

第 3 步那句话，就是本框架的核心命题：**「删代码」是结构问题（静态 harness 能管），但「删完行为不变」是行为问题（只有动态 harness 能证）**。两段工作不是两件事，是一条验证链的上下游。

---

## 1. 理论框架：什么是 harness

**定义**：harness 是把「做 X」翻译成「这个检查通过」的**闭环验证装置**。它把人脑里模糊的"应该对了"换成机器可判定的"绿/红"。

### 1.1 四要素

| 要素 | 含义 | 逆向清理（静态） | E2E（动态） |
|-----|------|----------------|------------|
| **目标→可验证标准** | 把"做 X"写成可机械判定的断言 | "删死代码" → `unused=0 且 build/vet/unit/integration 绿 且 rg 死符号清零` | "网关转发正确" → `穿网关请求得 200 + 正确响应 + 余额按预期变化` |
| **反馈源** | 谁来判绿/红 | 编译器、`golangci-lint unused`、nilness | 真实上游 + 被测网关 + admin/user API |
| **迭代循环** | 红了改、改了再判，直到绿 | 删→build→lint 报级联→再删 | 写断言→跑→看真实响应→调断言/修码 |
| **分级门禁** | 不过当前关不许进下一关 | 类3→类2→类1 逐阶段提交，失败即停 | 每测试独立 + 环境不全优雅 skip |

### 1.2 两类 harness（核心分类）

- **静态 harness（编译器 / linter 驱动）**
  反馈来自「不运行就能算」的分析。**快、确定、全程序可达性**（能看到你肉眼看不到的传递闭包）。
  管的是**结构正确**：没有死代码、没有未用符号、类型对、依赖闭合。

- **动态 harness（运行时 / E2E 驱动）**
  反馈来自「真把系统跑起来」。**慢、需要环境、只覆盖你实际跑到的路径**。
  管的是**行为正确**：真请求真响应、计费真扣、限额真拦。

### 1.3 选型原则（最重要）

> **结构变更用静态 harness；行为变更用动态 harness。静态 harness 永远证明不了行为正确。**

删一个函数，编译器/linter 能告诉你"没人再用了、编译还过"——但**它没法告诉你"删完后这条请求转发出去的字节和以前一样"**。后者要么靠形式化证明，要么靠把真实流量跑一遍。这就是为什么逆向清理的类 1（热路径）必须延伸出 E2E。

---

## 2. 案例 A：逆向清理 = 静态 harness

### 2.1 目标可验证化

「清掉逆向订阅死代码」太模糊，无法判定完成。翻译成可机械验证的门禁：

```
go build ./... && go vet ./...                 # 编译/vet 绿
go test -tags=unit ./internal/...              # 单测过
go test -tags=integration ./internal/...       # 集成过（Docker）
golangci-lint run ./...                        # unused 清零
rg 'tokenType == "oauth"|RequirePrivacySet|IsPrivacySet' backend/internal  # 死符号清零
```

### 2.2 反馈源：linter 的「传递闭包」是关键

死代码删除有**级联**：删 A 让只被 A 调用的 B 也变死，再让只被 B 调用的 C 变死……肉眼追不全。
`golangci-lint unused`（staticcheck U1000）做的是**全程序可达性分析**，一次性算出整个传递闭包。

**最反直觉、也最该记住的一课**：
> `tryWriteOpenAIImagesStreamEvent` 我 `grep` 数到 **12 处引用**，却被 linter 判 `unused`——因为那 12 处全在「同样 unused 的函数」内部，整簇从任何活入口都不可达。

**教训**：信 linter 的可达性分析，别信自己的 grep 计数。grep 数的是"文本出现次数"，linter 算的是"从入口可达否"——后者才是死活的真相。

### 2.3 迭代循环：编译器驱动级联删除

不要试图一次想全。**删一批 → `go build` + 重跑 linter → 它告诉你新冒出来的孤儿 → 再删**，直到 `unused=0`。
本次实测的级联链：删 oauth 死分支 → `getBetaHeader` 变死 → `mergeAnthropicBeta`/`mergeAnthropicBetaDropping` 变死 → 5 个 constants 变死 → 相关测试变死。一层层被 linter 揪出来，不是一开始就看得见的。

### 2.4 分级门禁：低风险先行、逐阶段提交

按风险从低到高（类3 unused 孤儿 → 类2 schema 死字段 → 类1 热路径）分阶段，**每阶段过全套门禁才提交、才进下一阶段，失败即停**。好处：

- 出问题回滚面小（一个阶段一个 commit）。
- 高风险阶段（类1）单独隔离，评审/验证聚焦。

### 2.5 静态 harness 的边界 → 交棒给动态

类 1 删完，静态门禁全绿、`rg` 清零。但 `follow-up` 文档拦了一道：
> 类 1 改网关核心热路径（鉴权头拼装），**unit 测试兜不住转发行为正确性**——必须真实 apikey/vertex 转发冒烟。

翻译成框架语言：**静态 harness 已到边界，结构对了但行为没证，必须接动态 harness**。于是有了案例 B。

---

## 3. 案例 B：E2E = 动态 harness

### 3.1 自包含四步骨架

```
boot（起服务）→ seed（造数据）→ run（跑测试）→ teardown（拆环境，必须 trap）
```

- **boot** 复用项目已有起服务脚本（`script/dev_local.sh`），harness 只拉起 + 等 `/health`。
- **teardown 用 `trap cleanup EXIT`**：通过/失败/中断都拆；用 `BOOTED_SERVER` 标志位区分「自起自拆」与「连已有实例不拆」。
- **双模式**：`E2E_BASE_URL` 已设 → 连已运行实例；未设 → 自起自拆。一个脚本覆盖本地调试 + CI。

### 3.2 灰盒：控制面 + 数据面一起用

真实业务 e2e 不是纯黑盒，要同时驱动：
- **控制面（admin/user API）**：seed 账号/分组/key、建用户设余额、查余额/用量。
- **数据面（网关）**：穿过被测网关发真请求。

断言 = 数据面行为（转发成功/被拦截）+ 控制面状态变化（余额扣减/用量入账）。

### 3.3 provisioning 放 Go，bash 只编排

复杂造数据（带特定余额/配额/限流的用户）放 typed Go helper；bash 只做起服务/注入 env/跑 `go test`/拆环境。
Go 侧 `sync.Once` 做一次性共享 provisioning，`requireProvision(t)` 懒取，环境不全 `t.Skip`。

---

## 4. 衔接：为什么 cleanup 必然走向 e2e（本框架最值钱的一课）

把两个案例叠在一起，得到一条**完整验证链**：

```
删代码 ──静态 harness（编译器/linter）──> 结构正确（没死代码、编译过、unused=0）
                                              │
                          静态到此为止，证明不了"行为不变"
                                              ▼
真实转发 ──动态 harness（E2E）──> 行为正确（转发字节对、计费闭环、限额生效）
```

- **逆向清理的"完成"定义本身就含动态验证**：follow-up 把"真实转发冒烟"列为类 1 的验收项。E2E 不是额外加的活，是 cleanup 验收链的最后一环。
- **E2E 反过来给了 cleanup 信心**：删完 oauth 死分支后，穿真实上游的 Claude/OpenAI/Gemini 转发全 200、计费逐分钱对账闭合——这才真正证明"删对了"。
- **通用规律**：任何「改了热路径/共享逻辑」的重构，静态绿只是必要条件，不是充分条件。充分条件是把真实行为跑一遍。**预算允许就把动态 harness 一起搭，别停在静态绿。**

---

## 5. 动态 harness 实操细节（踩坑 + 技法）

### 5.1 Shell / Bash 坑

**`${VAR:-default}` vs `${VAR-default}`（冒号之差）**——想用 `E2E_RUN=""` 跑全部，空串却回落默认：
```bash
RUN_FILTER="${E2E_RUN:-TestE2EFull}"   # ❌ 冒号：空串也触发默认
RUN_FILTER="${E2E_RUN-TestE2EFull}"    # ✅ 无冒号：只有"未设"才默认
```

**裸 `$VAR` 贴 CJK，`set -u` + C.UTF-8 报 unbound**——bash 把多字节字节并进变量名：
```bash
echo "端口 $BACKEND_PORT，跳过"     # ❌
echo "端口=${BACKEND_PORT} 跳过"    # ✅ 花括号定界 + ASCII 标点
```

**`set -o pipefail` 下管道 `$?` 抓的是最后一段**（扫密时差点误判）：
```bash
if git diff --cached | grep -iE 'sk-[0-9a-f]{32}'; then echo "发现凭证!"; else echo "干净"; fi
```

**自动加载 gitignored env**：`set -a; source "$ENV_FILE"; set +a`。

### 5.2 凭证管理（安全红线）

- 三件套：gitignored 真凭证（`script/e2e.env`）+ 提交模板（`.example`，占位值）+ harness 自动加载。
- **`.gitignore` 路径陷阱**：裸 `scripts` 规则匹配任意层级 → `backend/scripts/` 被忽略、根级 `script/`（单数）没被忽略。放脚本前 `git check-ignore -v` 确认；放凭证前确认**确实**被忽略。
- 提交前必扫暂存区凭证；`//go:build e2e` build tag 隔离，单独 `go vet -tags=e2e` 做编译门禁。

### 5.3 测试设计（动态 harness 的断言艺术）

- **上游不可控 → 容忍式 skip 而非 fail**：第三方 404/502 不是你网关的 bug。
  ```go
  if st != 200 {
      if st == 502 || bodyContains(body, "Upstream") { t.Skipf("上游暂不可用 st=%d", st) } // ⏭ 上游
      t.Fatalf("期望 200，实际 %d", st)                                                      // ❌ 我方
  }
  ```
  认知：**网关把上游非 200 如实回传，恰证明转发链路是通的**。本次 count_tokens(404)、gemini(502) 都走 skip。
- **模型映射穿网关测**：直连上游 `gemini-2.5-flash` 404、穿网关 200——网关有模型规范化。结论以"穿网关"为准。
- **限额是"消费后检查" → sendUntilBlocked + 等异步落账**：第一次必过（used=0），消费后第二次才拦。
- **阈值语义要精确 + 验证前置**：余额预检是 `Balance <= 0`（非 `< 0`），构造场景把余额设成恰好 0，且先确认环境没发赠额（发了就 skip）。
- **枚举合法值读 handler `oneof`**：`status` 合法值是 `active|inactive`（不是 `disabled`）——别猜，grep `binding:"oneof="`。
- **计费：弱行为断言 + 对账强校验**：不断言精确成本（脆），断言"扣了/没扣"；再用 `SUM(actual_cost) == 初始-当前余额` 逐分钱对账验闭环。
- **优雅 skip 哲学**：环境不全 → skip 不 fail，CI 友好。区分 skip(无法构造/上游限制) 与 fail(我方坏)。

---

## 6. 抽象出的共通 harness 原则（跨静/动态）

1. **目标可验证化**：任何"做 X"先翻译成一条机器能判绿/红的断言，否则无法判定完成。
2. **选对反馈源**：结构问题用编译器/linter（快、全、确定）；行为问题用真实运行（慢、需环境、覆盖你跑到的）。
3. **信工具的全局分析，别信局部直觉**：linter 可达性 > grep 计数；穿网关结果 > 直连探测。
4. **分级门禁、失败即停**：低风险先行，逐阶段提交，每关绿了才进下一关。
5. **容忍不可控**：依赖外部（上游/网络）的部分，区分"我方故障"与"外部限制"，后者 skip。
6. **静态绿 ≠ 行为对**：改热路径/共享逻辑，静态门禁只是必要条件，动态验证才充分。
7. **提交前自检**：扫凭证、build tag 隔离、`check-ignore` 确认。

---

## 7. 复用清单

**静态 harness（代码变更/重构）**
- [ ] 把目标写成门禁命令组（build/vet/test/lint/rg）
- [ ] 用 linter 的 unused/可达性当级联探测器，删→build→重跑 lint→再删
- [ ] 信可达性分析，别信 grep 计数
- [ ] 分级门禁、逐阶段提交、失败即停
- [ ] 改了热路径 → 明确标记"需动态验证"，别停在静态绿

**动态 harness（e2e/行为验证）**
- [ ] `set -euo pipefail` + `trap cleanup EXIT` + `BOOTED_SERVER` + 双模式
- [ ] 自动加载 gitignored env（`set -a; source; set +a`）+ `.example` 模板
- [ ] `git check-ignore -v` 双向确认；提交前扫密；build tag 隔离
- [ ] provisioning 放 Go（`sync.Once` + 懒取 + 环境不全 skip）
- [ ] 枚举值读 handler `oneof`；上游非 200 区分 skip/fail
- [ ] 限额用 sendUntilBlocked + 等异步落账；计费弱断言 + 对账强校验

---

## 8. 一句话心法

> **harness 是把"应该对了"换成"绿了"的装置。**
> 结构对不对，问编译器和 linter（静态）；行为对不对，把真东西跑一遍（动态）。
> **改了热路径，静态绿只是开始，别停。**
