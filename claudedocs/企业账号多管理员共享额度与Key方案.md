# 企业组织与额度分配（Team 协作 v2）方案

> 状态：v2 设计定稿，实施中，2026-07-15
> 前身：v1「共享控制面」模型（2026-07-14 实现，未推送即废弃，见文末附录）

## Context

v1 模型让协作管理员通过 `X-Team-Id` 请求头切换上下文、共管企业 owner 的 Key 和余额。整体测试后确认不符合企业管理要求，用户提出五点新需求：

1. 企业既可**共享额度**，又可给团队成员**分配额度**
2. 企业可设置**多个管理员**，管理额度分配
3. 用户注册后通过补充信息**自助升级为企业客户**，升级后邀请员工加入
4. 员工**自行管理自己的 API Key**，不共享 Key
5. 管理员可设置**一级部门**，邀请时按部门邀请

分支 `feature/team-collaboration` 未推送、迁移 175 未发布，直接在原地重构。

## 核心架构决策

| 决策点 | 结论 | 理由 |
|---|---|---|
| 计费模型 | **真实划转余额**：分配额度 = 从企业 User 余额原子划转到员工 User 余额；员工 Key 的 owner 就是员工本人，消费扣员工自己的余额 | **网关热路径 / api_keys / usage_logs 零改动**。已核实 `DeductBalance` 是 `AddBalance(-x)` 原子自减（`user_repo.go:780`），与划转事务的行锁并发安全 |
| 共享额度语义 | **自动补给**：`quota_mode=shared` 的成员由后台任务在余额 < 阈值时自动从企业余额划转补到目标水位 | 不碰热路径；分钟级滞后可接受（明示 SLA：滞后窗口内员工余额可能耗尽触发 402/429，下一轮补给自动恢复） |
| 分配额度语义 | `quota_mode=allocated`：管理员手动划转固定金额，员工消费自己的余额，花完为止 | |
| 企业升级 | 自助即时生效：填企业名称/联系人等补充信息 → 落 `enterprise_profiles` 行；存在该行 = 企业客户 | 无平台审批环节（后续需要可加开关） |
| 部门 | 一级（企业→部门→员工），`department_id` 挂在成员关系上 | |
| 角色 | `owner`（企业创建者，隐式超管）/ `admin`（部门 CRUD、邀请/移除 member、划转/回收、看报表台账）/ `member`（只看自己所属企业信息）；**仅 owner 可升降 admin** | |
| 旧模式处置 | X-Team-Id 上下文切换链路整体废弃回退（中间件 + ~26 处 `GetResourceOwnerID` 接入点 + 前端 Switcher/拦截器） | 员工各管各的 Key 后，"切换上下文共管资源"语义不复存在 |
| 命名 | 保留 `team_*` 表名/代码命名，前端文案统一用「企业」 | 降低重构面 |

## 数据模型

**新表**（迁移 175 改写，未发布可直接改）：

- `enterprise_profiles`：`user_id` UNIQUE FK(users, CASCADE)、`company_name`、`contact_name`、`contact_phone`、`industry`、时间戳。存在此行 = 企业客户。
- `team_departments`：`id`、`owner_user_id`、`name`、`display_order`、时间戳；UNIQUE(owner_user_id, name)。建表顺序必须在 team_members 之前（FK 依赖）。
- `team_fund_transfers`（划转台账）：`owner_user_id`、`member_user_id`、`direction` CHECK IN('grant','reclaim','auto_topup')、`amount` NUMERIC(20,8)、`operator_user_id`、`note`、`created_at`；索引 (owner_user_id, created_at DESC)、(member_user_id)。**不建 users 外键**（与 team_activity_logs 同理：台账须在用户被物理删除后保留追溯）。

**改表**：

- `team_members` 加列：`department_id` BIGINT NULL FK(team_departments, SET NULL)、`quota_mode` VARCHAR(20) DEFAULT 'allocated'、`auto_topup_threshold_usd`/`auto_topup_target_usd` NUMERIC(20,8) NULL、`granted_net_usd` NUMERIC(20,8) DEFAULT 0；`role` 默认改 'member'；补给扫描 partial index：`(quota_mode) WHERE quota_mode='shared' AND status='active'`。
- `team_invitations` 加列：`department_id` NULL、`role` DEFAULT 'member'、`quota_mode`、`initial_grant_usd` NULL（接受邀请后自动划转的初始额度）。

## 关键实现要点

### 原子划转（TeamFundRepository.TransferBalance）

- 事务内按 `min(fromID,toID), max(fromID,toID)` 顺序对两个 users 行 `SELECT ... FOR UPDATE`（固定锁序防死锁；模板参照 `user_platform_quota_repo.go:IncrementUsageWithReset`）。
- 锁内校验：grant/auto_topup 源=企业，余额 ≥ amount；reclaim 源=员工，amount ≤ min(`granted_net_usd`, 员工当前 balance)。
- 双 UPDATE balance → INSERT `team_fund_transfers` → UPDATE `team_members.granted_net_usd`（grant/topup 加、reclaim 减），同一事务提交。
- **不复用** `UpdateBalance`/`DeductBalance`（单行独立操作，无跨行原子性）。
- `granted_net_usd` 语义 = 企业对该成员的净投入，仅用于约束回收上限，不代表员工可用余额。已知限制：资金同质，回收无法精确区分员工自充部分，min(granted_net, balance) 是保守上限，台账全程透明。

### 自动补给（TeamAutoTopupService）

- 仿 `idempotency_cleanup_service.go` 的 Start/stop chan/ticker 后台服务模式，默认 60s 一轮；wire cleanup 链停止。
- 多实例互斥用现成的 `tryAcquireSingletonLeaderLock`（`service/leader_lock.go`，Redis 优先 + pg advisory 兜底）。
- 每轮扫描：`team_members JOIN users` 找 `quota_mode='shared' AND status='active' AND balance < auto_topup_threshold_usd`，LIMIT 200，逐条 `TransferBalance(auto_topup)` 补到 target；企业余额不足记 activity log 告警并跳过该条。

### 邀请流程扩展

`InviteMember` 增加 departmentID/role/quotaMode/initialGrant 参数（写入邀请行）；`AcceptInvitation` 成员事务提交后追加 initial_grant 划转，划转失败（如企业余额不足）不回滚成员加入，记 activity log。v1 已实现并保留：token 邀请/邮件、60s 重发限流（TryMarkSent 原子认领）、FOR UPDATE 行锁接受、成员上限 50。

### 报表

复用现成的 `GetBatchUserUsageStats(ctx, userIDs, start, end)`（`usage_log_repo_stats.go:476`）：成员列表附当前余额、granted_net、近 30 天消费。员工 Key 的 usage_logs 天然归属员工 user_id，无需归因改造。

## API 变更

新增：`POST/GET /user/enterprise/profile`（升级/查询）；`GET/POST/PUT/DELETE /user/team/departments`；`POST /user/team/members/:id/grant|reclaim`；`PUT /user/team/members/:id/quota-settings|department|role`；`GET /user/team/transfers`（台账分页）；`GET /user/team/report`。
修改：`POST /user/team/invitations` 请求体加 department_id/role/quota_mode/initial_grant_usd。
删除：X-Team-Id 上下文相关；`GET /user/teams` 保留但改为返回所属企业信息。

**admin 跨企业管理（实施中补充发现的设计缺口）**：企业允许设置多个 admin，一个 admin 可能同时管理"自己创建的企业"（若有）和"加入的别人的企业"。回退 X-Team-Id 后若所有 team/enterprise 接口的 ownerUserID 硬编码为 `subject.UserID`，非 owner 的 admin 将永远无法管理自己加入的企业——这与需求②"企业可设置多个管理员"矛盾。修复：`team_handler.go` 新增 `resolveTeamOwnerID(c, selfUserID)`，解析**请求级、显式**的可选 query 参数 `?owner_user_id=`（不传默认自己），供 team/enterprise 这组接口专用；鉴权仍由各 Service 内 `authorizeTeamManager` 校验调用者对该 ownerUserID 是否确有 owner/admin 权限，解析本身不做鉴权。与 v1 的关键区别：v1 的 X-Team-Id 是全局请求头，隐式影响 Key/支付/用量等所有资源判断；v2 的 `owner_user_id` 仅作用于 team/enterprise 接口，且每次请求显式声明，不做 header 级隐式状态。前端 `TeamMembersView.vue` 用右上角"正在管理"下拉框（数据源 `teamStore.manageableJoinedTeams`，即 `joinedTeams` 中 `role==='admin'` 的企业）驱动该参数，已通过浏览器实测验证。

## 前端改造

- 删：`TeamSwitcher.vue`、`TeamContextBanner.vue`、`client.ts` X-Team-Id 注入、`auth.ts` 团队上下文清理。
- 新增 `components/user/profile/EnterpriseUpgradeCard.vue`（ProfileView 卡片式接入）。
- `TeamMembersView.vue` 改造为企业管理页：成员列表加部门/额度模式/余额/granted_net 列 + 划转/回收对话框 + 部门管理 Tab + 台账 Tab；邀请对话框加部门/角色/额度模式/初始划转。
- `stores/team.ts` 从「上下文切换」改为「所属企业信息」；`TeamInviteAcceptView.vue` 保留；i18n 在 78 条基础上新增约 40 条（zh/en），文案统一「企业」。

## 实施阶段

P0 文档（本文件 + CHANGELOG + 功能列表）→ P1 回退 X-Team-Id 链路 → P2 数据模型 → P3 企业升级+部门 → P4 划转核心 → P5 自动补给 → P6 成员管理扩展+报表 → P7 前端 → P8 端到端冒烟。每阶段结束保持可编译可测试（wire_gen.go 手工维护点多，每阶段必跑 `go build ./...`）。

## 验证方案

1. 每阶段：`go build ./... && go test -tags=unit ./internal/service ./internal/repository ./internal/server ./internal/handler/...`
2. 迁移重放：本地 dev 库 drop team 表 + 清 schema_migrations 175 记录 → 重启 → `\d` 核对。
3. 真实 HTTP 冒烟：A 升级企业 → 建部门 → 邀请 B（指定部门+初始额度）→ B 接受后 B 余额增加/A 余额减少/台账有记录 → B 自己建 Key 调用扣自己余额 → A 划转/回收 → shared 成员余额拨低等自动补给 → A 查报表见 B 消费。
4. 前端：`pnpm run typecheck && pnpm run lint:check && pnpm test:run`。
5. 测试更新：v1 的 26 个 team_service 单测中 InviteMember 系列签名更新、Accept 系列加 initial_grant 断言、RemoveMember 明确「移除不自动回收余额」；新增并发反向划转无死锁、余额不足、回收上限、AutoTopup 幂等、重复升级用例；`team_context_test.go` 随中间件删除。

---

## 附录：v1「共享控制面」模型（已废弃）

v1（2026-07-14）的设计：不新建实体，协作管理员通过 `X-Team-Id` 头切换上下文共管 owner 的 Key 与余额，`middleware.TeamContext` 解析 `GetResourceOwnerID` 替换 ~26 处 handler 的 `subject.UserID`。刻意零改动网关数据面。

**废弃原因**：整体测试后确认「共管同一批 Key」不符合企业管理要求——企业需要的是员工各管各的 Key、企业统一控制额度分配，而非多人操作同一资源池。v1 的邀请流程（token/邮件/限流/行锁接受）、三张表、审计机制在 v2 中保留复用；X-Team-Id 上下文切换链路整体回退。

v1 曾修复并在 v2 中继续有效的问题（详见 git 历史与本文档历史版本）：gin 路由通配符冲突（accept 路由改 `/invitations/accept/:token`）、缺失迁移 175、成员上限并发竞态（FOR UPDATE 行锁）、重发限流 TOCTOU（原子条件 UPDATE 认领）、审计表不建外键、测试 fake 默认值对齐。
