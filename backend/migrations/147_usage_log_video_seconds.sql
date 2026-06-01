-- 视频生成时长（秒）。仅 video_generation 模式有值，配合 model_pricings.output_cost_per_image
-- (USD/秒) 计算计费金额。历史行默认为 0，不影响 token 计费链路。
ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS video_seconds NUMERIC(10,3) NOT NULL DEFAULT 0;
