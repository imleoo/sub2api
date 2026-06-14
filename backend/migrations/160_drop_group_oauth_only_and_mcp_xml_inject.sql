-- 移除订阅逆向遗留的分组字段：
--   - require_oauth_only：仅允许非 apikey（OAuth）账号关联，随 OAuth 账号类型移除而失效。
--   - mcp_xml_inject：MCP XML 协议注入开关，仅 antigravity 平台使用，为死开关（从未流入注入逻辑）。
-- ent schema 已删除对应字段；本迁移同步清理 groups 表实际列。

ALTER TABLE groups
    DROP COLUMN IF EXISTS require_oauth_only,
    DROP COLUMN IF EXISTS mcp_xml_inject;
