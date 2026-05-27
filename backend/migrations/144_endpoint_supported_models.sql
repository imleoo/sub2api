-- 功能 25：endpoint 新增 supported_models（JSONB 数组），记录该端点实际支持转发的模型 ID。
-- 空数组/NULL = 未配置（行为同改造前：管理员 GetAvailableModels 仍返回各 platform 默认模型）。

ALTER TABLE endpoints
    ADD COLUMN IF NOT EXISTS supported_models JSONB;
