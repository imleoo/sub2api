-- 模型定价 SSOT 重构 PR-1：扩展 model_pricings 表字段。
-- 1) 补齐当前已被计费引用但 schema 缺失的字段（long_context / priority / cache_5m/1h / image_output）
-- 2) 引入 SSOT 元数据列（source / pricing_status / source_provider / source_account_id）
-- 全部 nullable，幂等。回滚使用 DROP COLUMN IF EXISTS。

ALTER TABLE model_pricings
    ADD COLUMN IF NOT EXISTS long_context_input_token_threshold BIGINT,
    ADD COLUMN IF NOT EXISTS long_context_input_cost_multiplier DECIMAL(10, 4),
    ADD COLUMN IF NOT EXISTS long_context_output_cost_multiplier DECIMAL(10, 4),
    ADD COLUMN IF NOT EXISTS input_cost_per_token_priority DECIMAL(30, 15),
    ADD COLUMN IF NOT EXISTS output_cost_per_token_priority DECIMAL(30, 15),
    ADD COLUMN IF NOT EXISTS cache_read_input_token_cost_priority DECIMAL(30, 15),
    ADD COLUMN IF NOT EXISTS cache_creation_5m_token_cost DECIMAL(30, 15),
    ADD COLUMN IF NOT EXISTS cache_creation_1h_token_cost DECIMAL(30, 15),
    ADD COLUMN IF NOT EXISTS supports_cache_breakdown BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS image_output_price_per_token DECIMAL(30, 15),
    ADD COLUMN IF NOT EXISTS source VARCHAR(20) NOT NULL DEFAULT 'manual',
    ADD COLUMN IF NOT EXISTS source_provider VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS source_account_id BIGINT,
    ADD COLUMN IF NOT EXISTS pricing_status VARCHAR(20) NOT NULL DEFAULT 'unpriced';

-- 回填 pricing_status：有任意上游价格或自定义价格则视为已定价
UPDATE model_pricings
SET pricing_status = 'priced'
WHERE pricing_status = 'unpriced'
  AND (
        input_cost_per_token IS NOT NULL
        OR output_cost_per_token IS NOT NULL
        OR custom_input_cost IS NOT NULL
        OR custom_output_cost IS NOT NULL
        OR output_cost_per_image IS NOT NULL
        OR output_cost_per_image_token IS NOT NULL
  );

-- 回填 source：根据 is_custom + provider 推断
-- - is_custom=false → litellm（远端 JSON 同步）
-- - is_custom=true AND provider='lingjing' → lingjing（代码内置静态价格）
-- - is_custom=true 其余 → manual（管理员手工或上游同步未区分时）
UPDATE model_pricings SET source = 'litellm' WHERE is_custom = FALSE AND source = 'manual';
UPDATE model_pricings SET source = 'lingjing' WHERE is_custom = TRUE AND LOWER(provider) = 'lingjing' AND source = 'manual';

-- 索引：source / pricing_status 用于折扣页过滤
CREATE INDEX IF NOT EXISTS idx_model_pricings_source ON model_pricings (source);
CREATE INDEX IF NOT EXISTS idx_model_pricings_pricing_status ON model_pricings (pricing_status);
