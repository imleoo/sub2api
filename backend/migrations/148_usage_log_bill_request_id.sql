-- 下游对账标识列 + 索引（对账 API 按此列点查）
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS bill_request_id VARCHAR(64);
CREATE INDEX IF NOT EXISTS idx_usage_logs_bill_request_id ON usage_logs (bill_request_id);
