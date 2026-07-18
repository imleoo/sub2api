-- 用户余额月结快照（月度对账单 Vendor Report 的期初/期末精确来源）。
-- 可重算 rollup：一行 = 单用户单月，(user_id, period) 唯一幂等；不建 users 外键，
-- 对账记录须在用户被物理删除后保留追溯。恒等式：
--   closing_balance = opening_balance + deposit_total - withdraw_total
--                     + credit_total - utilisation_total
CREATE TABLE IF NOT EXISTS balance_snapshots (
    id                       BIGSERIAL PRIMARY KEY,
    user_id                  BIGINT         NOT NULL,
    period                   VARCHAR(7)     NOT NULL,
    opening_balance          DECIMAL(20,8)  NOT NULL DEFAULT 0,
    closing_balance          DECIMAL(20,8)  NOT NULL DEFAULT 0,
    deposit_total            DECIMAL(20,8)  NOT NULL DEFAULT 0,
    withdraw_total           DECIMAL(20,8)  NOT NULL DEFAULT 0,
    credit_total             DECIMAL(20,8)  NOT NULL DEFAULT 0,
    utilisation_gross_total  DECIMAL(20,10) NOT NULL DEFAULT 0,
    utilisation_total        DECIMAL(20,10) NOT NULL DEFAULT 0,
    computed_at              TIMESTAMPTZ    NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_balance_snapshots_user_period
    ON balance_snapshots (user_id, period);

CREATE INDEX IF NOT EXISTS idx_balance_snapshots_period
    ON balance_snapshots (period);
