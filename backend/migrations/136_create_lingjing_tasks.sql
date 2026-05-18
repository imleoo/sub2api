-- 创建 lingjing_tasks 表，用于存储京东云灵境豆包系列异步生成任务（文生视频/图生视频）

CREATE TABLE IF NOT EXISTS lingjing_tasks (
    id            BIGSERIAL PRIMARY KEY,
    gen_task_id   VARCHAR(128)     NOT NULL,
    task_type     VARCHAR(20)      NOT NULL,
    status        VARCHAR(20)      NOT NULL DEFAULT 'pending',
    error_message TEXT,
    result_url    TEXT,
    request_body  JSONB            NOT NULL DEFAULT '{}',
    user_id       BIGINT           NOT NULL,
    api_key_id    BIGINT           NOT NULL,
    account_id    BIGINT           NOT NULL,
    group_id      BIGINT,
    model         VARCHAR(100)     NOT NULL DEFAULT '',
    duration      VARCHAR(10)      NOT NULL DEFAULT '',
    mode          VARCHAR(10)      NOT NULL DEFAULT '',
    cost          DECIMAL(20, 8)   NOT NULL DEFAULT 0,
    billed        BOOLEAN          NOT NULL DEFAULT FALSE,
    poll_attempts INT              NOT NULL DEFAULT 0,
    started_at    TIMESTAMPTZ,
    finished_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ      NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ      NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS lingjing_tasks_gen_task_id_key
    ON lingjing_tasks (gen_task_id);

CREATE INDEX IF NOT EXISTS idx_lingjing_tasks_status_created_at
    ON lingjing_tasks (status, created_at);

CREATE INDEX IF NOT EXISTS idx_lingjing_tasks_user_id
    ON lingjing_tasks (user_id);

CREATE INDEX IF NOT EXISTS idx_lingjing_tasks_api_key_id
    ON lingjing_tasks (api_key_id);

CREATE INDEX IF NOT EXISTS idx_lingjing_tasks_account_id
    ON lingjing_tasks (account_id);
