-- 扩展 pricing_unit 取值，支持 image_generation / video_generation。
-- 配合 ent schema field.Enum("pricing_unit").Values("token","second","image_generation","video_generation")。
-- pricing_unit 物理类型为 varchar(20)+CHECK（见 153），此处放宽 CHECK 并按 mode 回填。

ALTER TABLE model_pricings
  DROP CONSTRAINT IF EXISTS model_pricings_pricing_unit_check;

ALTER TABLE model_pricings
  ADD CONSTRAINT model_pricings_pricing_unit_check
  CHECK (pricing_unit IN ('token', 'second', 'image_generation', 'video_generation'));

-- 按 mode 回填 pricing_unit（mode 字段可靠存 image_generation/video_generation）。
-- 分列约定：按次价（output_cost_per_image）的图片行标 image_generation；
-- 仅按 token 计价（output_cost_per_image_token，per-image-token）的图片行保留 token。
UPDATE model_pricings
   SET pricing_unit = 'image_generation', updated_at = NOW()
 WHERE mode = 'image_generation'
   AND output_cost_per_image IS NOT NULL
   AND pricing_unit <> 'image_generation';

-- 视频行统一标 video_generation（价格在 output_cost_per_image 按秒 或 output_cost_per_image_token）。
UPDATE model_pricings
   SET pricing_unit = 'video_generation', updated_at = NOW()
 WHERE mode = 'video_generation'
   AND pricing_unit <> 'video_generation';
