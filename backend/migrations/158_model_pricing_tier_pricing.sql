-- 通用化：视频按 token 计费的分档单价（[]VideoPriceTier JSON，¥/百万 token 原值）。
-- 不再内置，由上传的定价 JSON 解析后存入；计费时按汇率折 USD。
ALTER TABLE model_pricings
  ADD COLUMN IF NOT EXISTS tier_pricing text;
