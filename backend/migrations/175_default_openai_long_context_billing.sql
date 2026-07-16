-- Keep mixed-version writers consistent before backfilling rows that already exist.
--
-- fork：上游此迁移原本还包含 spark 影子账号（parent_account_id/quota_dimension）向子账号
-- 传播 openai_long_context_billing_enabled 的逻辑；该账号层级体系已随订阅逆向清理（功能 35）
-- 整体移除，accounts 表没有 parent_account_id/quota_dimension 列，故本迁移只保留与扁平
-- openai 账号相关的默认值回填 + 布尔类型强制校验部分。
CREATE OR REPLACE FUNCTION public.enforce_openai_long_context_billing_extra()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.platform IS DISTINCT FROM 'openai' THEN
        RETURN NEW;
    END IF;

    NEW.extra := COALESCE(NEW.extra, '{}'::jsonb);
    IF NOT (NEW.extra ? 'openai_long_context_billing_enabled')
        AND TG_OP = 'UPDATE'
        AND OLD.platform = 'openai'
        AND jsonb_typeof(OLD.extra->'openai_long_context_billing_enabled') = 'boolean' THEN
        NEW.extra := jsonb_set(
            NEW.extra,
            '{openai_long_context_billing_enabled}',
            OLD.extra->'openai_long_context_billing_enabled',
            true
        );
    ELSIF NOT (NEW.extra ? 'openai_long_context_billing_enabled') THEN
        NEW.extra := jsonb_set(
            NEW.extra,
            '{openai_long_context_billing_enabled}',
            'false'::jsonb,
            true
        );
    END IF;

    IF jsonb_typeof(NEW.extra->'openai_long_context_billing_enabled') IS DISTINCT FROM 'boolean' THEN
        RAISE EXCEPTION 'openai_long_context_billing_enabled must be a boolean'
            USING ERRCODE = '22023';
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS accounts_propagate_openai_long_context_billing_extra ON accounts;

DROP TRIGGER IF EXISTS accounts_enforce_openai_long_context_billing_extra ON accounts;
CREATE TRIGGER accounts_enforce_openai_long_context_billing_extra
BEFORE INSERT OR UPDATE OF platform, extra
ON accounts
FOR EACH ROW
EXECUTE FUNCTION public.enforce_openai_long_context_billing_extra();

UPDATE accounts
SET extra = jsonb_set(
    COALESCE(extra, '{}'::jsonb),
    '{openai_long_context_billing_enabled}',
    'false'::jsonb,
    true
)
WHERE platform = 'openai'
  AND COALESCE(extra, '{}'::jsonb) ? 'openai_long_context_billing_enabled'
  AND jsonb_typeof(extra->'openai_long_context_billing_enabled') IS DISTINCT FROM 'boolean';

UPDATE accounts
SET extra = jsonb_set(
    COALESCE(extra, '{}'::jsonb),
    '{openai_long_context_billing_enabled}',
    'false'::jsonb,
    true
)
WHERE platform = 'openai'
  AND NOT (COALESCE(extra, '{}'::jsonb) ? 'openai_long_context_billing_enabled');
