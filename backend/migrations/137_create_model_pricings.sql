-- 创建 model_pricings 表，统一管理远端同步模型和手动自定义模型的计费定价信息

CREATE TABLE IF NOT EXISTS model_pricings (
    id                              BIGSERIAL        PRIMARY KEY,
    model_id                        VARCHAR(200)     NOT NULL,
    display_name                    VARCHAR(200),
    description                     TEXT,
    provider                        VARCHAR(100)     NOT NULL DEFAULT '',
    mode                            VARCHAR(50)      NOT NULL DEFAULT 'chat',
    input_cost_per_token            DECIMAL(30,15),
    output_cost_per_token           DECIMAL(30,15),
    cache_creation_input_token_cost DECIMAL(30,15),
    cache_read_input_token_cost     DECIMAL(30,15),
    output_cost_per_image           DECIMAL(30,15),
    output_cost_per_image_token     DECIMAL(30,15),
    supports_prompt_caching         BOOLEAN          NOT NULL DEFAULT FALSE,
    custom_input_cost               DECIMAL(30,15),
    custom_output_cost              DECIMAL(30,15),
    discount_rate                   DECIMAL(10,4),
    is_custom                       BOOLEAN          NOT NULL DEFAULT FALSE,
    is_enabled                      BOOLEAN          NOT NULL DEFAULT TRUE,
    last_synced_at                  TIMESTAMPTZ,
    created_at                      TIMESTAMPTZ      NOT NULL DEFAULT NOW(),
    updated_at                      TIMESTAMPTZ      NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS modelpricing_model_id
    ON model_pricings (model_id);

CREATE INDEX IF NOT EXISTS modelpricing_provider
    ON model_pricings (provider);

CREATE INDEX IF NOT EXISTS modelpricing_is_custom
    ON model_pricings (is_custom);

CREATE INDEX IF NOT EXISTS modelpricing_is_enabled
    ON model_pricings (is_enabled);
