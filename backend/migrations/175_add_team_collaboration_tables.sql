-- 175_add_team_collaboration_tables.sql
-- 企业组织与额度分配（Team 协作 v2，zhiguofan fork-only）
-- 对应 ent schema: enterprise_profiles / team_departments / team_members /
--                  team_invitations / team_fund_transfers / team_activity_logs
--
-- v2 计费模型：员工 Key 消费扣员工自己的余额；企业通过真实划转分配额度。
-- allocated = 管理员手动划转；shared = 后台任务自动补给到目标水位。

-- 企业客户资料：存在此行 = 企业客户
CREATE TABLE IF NOT EXISTS enterprise_profiles (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    company_name TEXT NOT NULL,
    contact_name TEXT NOT NULL DEFAULT '',
    contact_phone VARCHAR(50) NOT NULL DEFAULT '',
    industry VARCHAR(100) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 一级部门（必须建在 team_members 之前：members.department_id 依赖本表）
CREATE TABLE IF NOT EXISTS team_departments (
    id BIGSERIAL PRIMARY KEY,
    owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    display_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_team_departments_owner_name ON team_departments(owner_user_id, name);
CREATE INDEX IF NOT EXISTS idx_team_departments_owner_user_id ON team_departments(owner_user_id);

-- 企业成员关系
CREATE TABLE IF NOT EXISTS team_members (
    id BIGSERIAL PRIMARY KEY,
    owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    member_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL DEFAULT 'member',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    department_id BIGINT NULL REFERENCES team_departments(id) ON DELETE SET NULL,
    quota_mode VARCHAR(20) NOT NULL DEFAULT 'allocated',
    auto_topup_threshold_usd NUMERIC(20,8) NULL,
    auto_topup_target_usd NUMERIC(20,8) NULL,
    granted_net_usd NUMERIC(20,8) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_team_members_owner_member ON team_members(owner_user_id, member_user_id);
CREATE INDEX IF NOT EXISTS idx_team_members_owner_user_id ON team_members(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_team_members_member_user_id ON team_members(member_user_id);
-- 自动补给扫描专用 partial index
CREATE INDEX IF NOT EXISTS idx_team_members_shared_active ON team_members(quota_mode)
    WHERE quota_mode = 'shared' AND status = 'active';

-- 邀请
CREATE TABLE IF NOT EXISTS team_invitations (
    id BIGSERIAL PRIMARY KEY,
    owner_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invited_email TEXT NOT NULL,
    token TEXT NOT NULL UNIQUE,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    expires_at TIMESTAMPTZ NOT NULL,
    last_sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    department_id BIGINT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'member',
    quota_mode VARCHAR(20) NOT NULL DEFAULT 'allocated',
    initial_grant_usd NUMERIC(20,8) NULL,
    accepted_by_user_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    accepted_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_team_invitations_owner_user_id ON team_invitations(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_team_invitations_invited_email ON team_invitations(invited_email);

-- 划转台账（只追加；不建 users 外键：物理删除用户后须保留追溯）
CREATE TABLE IF NOT EXISTS team_fund_transfers (
    id BIGSERIAL PRIMARY KEY,
    owner_user_id BIGINT NOT NULL,
    member_user_id BIGINT NOT NULL,
    direction VARCHAR(20) NOT NULL CHECK (direction IN ('grant', 'reclaim', 'auto_topup')),
    amount NUMERIC(20,8) NOT NULL CHECK (amount > 0),
    operator_user_id BIGINT NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_team_fund_transfers_owner_created ON team_fund_transfers(owner_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_team_fund_transfers_member ON team_fund_transfers(member_user_id);

-- 操作审计（只追加；不建 users 外键：同上保留追溯）
CREATE TABLE IF NOT EXISTS team_activity_logs (
    id BIGSERIAL PRIMARY KEY,
    owner_user_id BIGINT NOT NULL,
    actor_user_id BIGINT NOT NULL,
    action VARCHAR(50) NOT NULL,
    detail TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_team_activity_logs_owner_user_id ON team_activity_logs(owner_user_id);
