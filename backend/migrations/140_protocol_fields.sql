-- Phase 2 P2-1: 新增协议字段（双写兼容）
-- 文档：docs/glossary.md §3.3、docs/sprint-plan.md P2-1
-- group.inbound_protocol: 入站协议（anthropic_messages / openai_chat / openai_responses / gemini_v1beta）
-- account.outbound_protocol: 出站协议；空值表示历史账号，需按 platform/type 派生

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS inbound_protocol VARCHAR(30) NOT NULL DEFAULT '';

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS outbound_protocol VARCHAR(30) NOT NULL DEFAULT '';
