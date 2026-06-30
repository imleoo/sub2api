-- 为渠道模型定价新增「图片输入 token 价格」列，用于多模态 embedding 等图文不同价场景。
-- 与既有 image_output_price 对称；未设置时计费回退到普通文本输入价（input_price）。
ALTER TABLE channel_model_pricing
    ADD COLUMN IF NOT EXISTS image_input_price decimal(30,15);

ALTER TABLE channel_account_stats_model_pricing
    ADD COLUMN IF NOT EXISTS image_input_price decimal(30,15);
