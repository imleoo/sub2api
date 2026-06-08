-- 价格数据治理清洗（一次性，幂等）。
-- 仅处理无歧义、不改变模型集、不损坏计费的安全项；wanjie 存量按运营决策不在此自动重算
-- （靠下次 sync-from-wanjie@6.8 自然收敛）；seedance 变体的 per-second 基价分歧（¥0.08 vs ¥0.215）
-- 需运营按火山官网确认，本迁移不擅自收敛，仅随 lingjing 缩放统一应用汇率。

-- 1) 清理污染：非图片/视频模型不应有 output_cost_per_image（实测 4 条 gemini-3.1-pro-* 等 chat 行）。
UPDATE model_pricings
   SET output_cost_per_image = NULL, updated_at = NOW()
 WHERE mode NOT IN ('image_generation', 'video_generation')
   AND output_cost_per_image IS NOT NULL;

-- 2) 真未配价行标 unpriced（按 mode 应有的价格列全空），供 pricing_health 展示；不在此停用，交运营决策。
UPDATE model_pricings
   SET pricing_status = 'unpriced', updated_at = NOW()
 WHERE input_cost_per_token IS NULL
   AND output_cost_per_token IS NULL
   AND output_cost_per_image IS NULL
   AND output_cost_per_image_token IS NULL
   AND custom_input_cost IS NULL
   AND custom_output_cost IS NULL
   AND pricing_status <> 'unpriced';

-- 3) lingjing 行汇率统一：迁移 152 按 7.28 硬编码入库，统一缩放到系统汇率 6.8。
--    只缩放价格列、不改模型集；幂等由迁移体系（checksum 不可变 + 仅执行一次）保证。
UPDATE model_pricings
   SET output_cost_per_image = output_cost_per_image * (7.28 / 6.8),
       updated_at            = NOW()
 WHERE source = 'lingjing'
   AND pricing_status = 'priced'
   AND output_cost_per_image IS NOT NULL;

UPDATE model_pricings
   SET output_cost_per_image_token = output_cost_per_image_token * (7.28 / 6.8),
       updated_at                  = NOW()
 WHERE source = 'lingjing'
   AND pricing_status = 'priced'
   AND output_cost_per_image_token IS NOT NULL;
