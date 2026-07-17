-- 模型目录（SSOT catalog）新增「图片输入 token 单价」列，补齐上游 0.1.158 的
-- LiteLLM input_cost_per_image_token 默认价链路（gpt-image-2 图片编辑等图文不同价场景）。
-- 未设置（NULL）时计费回退文本输入价 input_cost_per_token，向后兼容。
-- 与渠道级 channel_model_pricing.image_input_price（迁移 162/178）互补：渠道价优先，目录价兜底。
ALTER TABLE model_pricings
  ADD COLUMN IF NOT EXISTS input_cost_per_image_token decimal(30,15);
