-- Phase 5 P5-1: 为现有非 lingjing 账号派生单 endpoint 记录（向后兼容）
-- 验收：老账号兼容（单 endpoint 自动派生）；lingjing 账号不参与（WHERE platform != 'lingjing'）
-- 文档：docs/generic-channel-design.md §6、docs/sprint-plan.md P5-1

-- stable_id 格式：<account_id>-default（保证全局唯一且稳定）
-- outbound_protocol：优先使用 account.outbound_protocol，空值按 platform 派生
-- base_url：从 credentials->>'base_url' 读取，无值时留空（非 generic 账号 base_url 由 handler 补充）

INSERT INTO endpoints (
    account_id,
    stable_id,
    outbound_protocol,
    base_url,
    auth_header,
    auth_scheme,
    models_source,
    priority,
    health,
    created_at,
    updated_at
)
SELECT
    a.id                                           AS account_id,
    CONCAT(a.id::text, '-default')                 AS stable_id,
    CASE
        WHEN a.outbound_protocol != '' THEN a.outbound_protocol
        WHEN a.platform = 'anthropic'  THEN 'anthropic_messages'
        WHEN a.platform = 'openai'     THEN 'openai_chat'
        WHEN a.platform = 'gemini'     THEN 'gemini_v1beta'
        WHEN a.platform = 'antigravity' THEN 'anthropic_messages'
        ELSE a.platform
    END                                            AS outbound_protocol,
    COALESCE(a.credentials->>'base_url', '')       AS base_url,
    'Authorization'                                AS auth_header,
    'Bearer'                                       AS auth_scheme,
    'remote'                                       AS models_source,
    100                                            AS priority,
    'healthy'                                      AS health,
    NOW()                                          AS created_at,
    NOW()                                          AS updated_at
FROM accounts a
WHERE a.platform != 'lingjing'
  AND NOT EXISTS (
      SELECT 1 FROM endpoints e
      WHERE e.account_id = a.id
        AND e.stable_id = CONCAT(a.id::text, '-default')
  );
