-- SSOT 重构 PR-5：pricing_drift_logs 表。
-- 记录 GetModelPricing 的 V1（pricingData + 家族 fuzzy）与 V2（catalog + aliasIdx）
-- 两条路径的差异；PR-6 切流前要求 ≥ 72h 0 条新增；PR-8 删除。

CREATE TABLE IF NOT EXISTS pricing_drift_logs (
    id            BIGSERIAL PRIMARY KEY,
    model_id      VARCHAR(200) NOT NULL,
    drift_kind    VARCHAR(32) NOT NULL DEFAULT '',
    v1_pricing    JSONB,
    v2_pricing    JSONB,
    hit_path      VARCHAR(64) NOT NULL DEFAULT '',
    occurred_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pricing_drift_logs_model_id    ON pricing_drift_logs (model_id);
CREATE INDEX IF NOT EXISTS idx_pricing_drift_logs_drift_kind  ON pricing_drift_logs (drift_kind);
CREATE INDEX IF NOT EXISTS idx_pricing_drift_logs_occurred_at ON pricing_drift_logs (occurred_at);
