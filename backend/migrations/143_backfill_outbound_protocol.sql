-- Phase 2 P2-1 补充：按 platform 推导回填 account.outbound_protocol
-- 仅处理 outbound_protocol 为空的历史账号，已手填的不覆盖。
-- lingjing 保持空（其路由不走协议分流）。

UPDATE accounts
SET outbound_protocol = CASE
    WHEN platform = 'anthropic'   THEN 'anthropic_messages'
    WHEN platform = 'openai'      THEN 'openai_chat'
    WHEN platform = 'gemini'      THEN 'gemini_v1beta'
    WHEN platform = 'antigravity' THEN 'anthropic_messages'
    ELSE ''
END
WHERE outbound_protocol = ''
  AND platform != 'lingjing';
