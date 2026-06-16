-- 移除 privacy 调度门控死字段 require_privacy_set。
-- Account.IsPrivacySet() 恒返回 true（所有平台均无 privacy 概念），
-- 故门控条件 RequirePrivacySet && !IsPrivacySet() 恒为 false，调度门控整体失效（inert）。
-- ent schema 已删除对应字段；本迁移同步清理 groups 表实际列。

ALTER TABLE groups DROP COLUMN IF EXISTS require_privacy_set;
