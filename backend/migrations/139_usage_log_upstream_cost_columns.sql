-- Phase 0 P0-2: UsageLog 新增上游成本快照列（全部 nullable）
-- 文档：docs/upstream-cost-snapshot.md §2.1、docs/sprint-plan.md P0-2
-- 与 pricing_source 一致性约束：upstream_total_cost 与 pricing_source 同时 NULL 或同时非空。
-- 线上校验由应用层保证；此处不加 CHECK 约束避免迁移阻塞。

ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS upstream_unit_price_input       DECIMAL(20,10),
    ADD COLUMN IF NOT EXISTS upstream_unit_price_output      DECIMAL(20,10),
    ADD COLUMN IF NOT EXISTS upstream_unit_price_cache_creation DECIMAL(20,10),
    ADD COLUMN IF NOT EXISTS upstream_unit_price_cache_read  DECIMAL(20,10),
    ADD COLUMN IF NOT EXISTS upstream_total_cost             DECIMAL(20,10),
    ADD COLUMN IF NOT EXISTS provider                        VARCHAR(50),
    ADD COLUMN IF NOT EXISTS pricing_source                  VARCHAR(20),
    ADD COLUMN IF NOT EXISTS async_task_id                   VARCHAR(64),
    ADD COLUMN IF NOT EXISTS cost_finalized_at               TIMESTAMPTZ;
