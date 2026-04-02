-- 创建 keyword_stats 表用于提示词词云统计
-- 存储从用户提示词中提取的关键词频次统计（不存储原始提示词内容）

CREATE TABLE IF NOT EXISTS keyword_stats (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    api_key_id BIGINT NOT NULL,
    group_id BIGINT NOT NULL,
    keyword VARCHAR(64) NOT NULL,
    count INT NOT NULL DEFAULT 1,
    period VARCHAR(10) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 唯一约束：同一用户、同一关键词、同一月份只有一条记录
CREATE UNIQUE INDEX IF NOT EXISTS idx_keyword_stats_unique
    ON keyword_stats(user_id, keyword, period);

-- 查询索引：按用户和月份查询
CREATE INDEX IF NOT EXISTS idx_keyword_stats_user_period
    ON keyword_stats(user_id, period);

-- 查询索引：按关键词和月份查询（全局词云）
CREATE INDEX IF NOT EXISTS idx_keyword_stats_keyword_period
    ON keyword_stats(keyword, period);

-- 清理索引：按创建时间清理旧数据
CREATE INDEX IF NOT EXISTS idx_keyword_stats_created_at
    ON keyword_stats(created_at);