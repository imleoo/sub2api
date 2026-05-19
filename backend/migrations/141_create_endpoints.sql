-- Phase 5 P5-1: 创建 endpoints 表（Account 1→N Endpoint）
-- 文档：docs/glossary.md §4、docs/generic-channel-design.md §6
-- lingjing 账号不参与迁移（WHERE platform != 'lingjing'）

CREATE TABLE IF NOT EXISTS endpoints (
    id                  BIGSERIAL       PRIMARY KEY,
    account_id          BIGINT          NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    stable_id           VARCHAR(128)    NOT NULL,
    outbound_protocol   VARCHAR(50)     NOT NULL,
    base_url            VARCHAR(512)    NOT NULL,
    auth_header         VARCHAR(64)     NOT NULL DEFAULT 'Authorization',
    auth_scheme         VARCHAR(32)     NOT NULL DEFAULT 'Bearer',
    models_source       VARCHAR(20)     NOT NULL DEFAULT 'remote',
    priority            INTEGER         NOT NULL DEFAULT 100,
    health              VARCHAR(20)     NOT NULL DEFAULT 'healthy',
    capabilities        JSONB,
    created_at          TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

-- (account_id, stable_id) 唯一约束：同一账号下 stable_id 不重复（UsageLog 快照依赖）
CREATE UNIQUE INDEX IF NOT EXISTS endpoint_account_stable_id
    ON endpoints (account_id, stable_id);

CREATE INDEX IF NOT EXISTS endpoint_account_id
    ON endpoints (account_id);

CREATE INDEX IF NOT EXISTS endpoint_protocol_health
    ON endpoints (outbound_protocol, health);

-- usage_logs 新增 endpoint_id / endpoint_protocol 快照字段（Phase 5 P5-1）
ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS endpoint_id       VARCHAR(128),
    ADD COLUMN IF NOT EXISTS endpoint_protocol VARCHAR(50);
