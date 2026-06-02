-- 更新 signup_source check constraint，添加 phone 注册方式
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_signup_source_check;
ALTER TABLE users ADD CONSTRAINT users_signup_source_check
    CHECK (signup_source::text = ANY (ARRAY[
        'email'::character varying,
        'phone'::character varying,
        'linuxdo'::character varying,
        'wechat'::character varying,
        'oidc'::character varying,
        'github'::character varying,
        'google'::character varying,
        'dingtalk'::character varying
    ]::text[]));
