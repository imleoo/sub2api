-- Phase 0 P0-1: 创建 provider_pricings 表（上游成本快照）
-- 文档：docs/upstream-cost-snapshot.md §2.2、docs/sprint-plan.md P0-1

CREATE TABLE IF NOT EXISTS provider_pricings (
    id                  BIGSERIAL        PRIMARY KEY,
    provider            VARCHAR(50)      NOT NULL,
    model               VARCHAR(100)     NOT NULL,
    billing_mode        VARCHAR(20)      NOT NULL DEFAULT 'token',
    input_price         DECIMAL(20,10)   NOT NULL DEFAULT 0,
    output_price        DECIMAL(20,10)   NOT NULL DEFAULT 0,
    cache_creation_price DECIMAL(20,10)  NOT NULL DEFAULT 0,
    cache_read_price    DECIMAL(20,10)   NOT NULL DEFAULT 0,
    currency            VARCHAR(8)       NOT NULL DEFAULT 'USD',
    effective_from      TIMESTAMPTZ      NOT NULL DEFAULT NOW(),
    effective_to        TIMESTAMPTZ,
    source              VARCHAR(50)      NOT NULL DEFAULT 'manual',
    created_at          TIMESTAMPTZ      NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ      NOT NULL DEFAULT NOW()
);

-- 命中查询索引：给定 (provider, model, now()) 找当前生效价格
CREATE INDEX IF NOT EXISTS providerpricings_provider_model_from
    ON provider_pricings (provider, model, effective_from);
