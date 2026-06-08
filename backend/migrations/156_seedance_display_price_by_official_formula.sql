-- 按火山官方 token 公式重写灵境 seedance 的「展示用」per-second 价（一次性，幂等）。
-- 实际计费已改为 BillingService.CalculateSeedanceVideoCost（token 公式 × doubao.json 分档单价），
-- output_cost_per_image 仅用于后台/广场展示，统一为官方公式 720p 无声基准：
--   858(720p 32-patch 网格/帧) × 24fps × ¥8/百万token（在线无声）÷ 6.8 = USD/秒。
-- 真实账单按请求实际分辨率/有声分档，可能与此展示值不同。
UPDATE model_pricings
   SET output_cost_per_image = 858 * 24 * 8.0 / 1000000 / 6.8,
       updated_at            = NOW()
 WHERE source = 'lingjing'
   AND mode = 'video_generation'
   AND lower(replace(model_id, '.', '-')) LIKE 'doubao-seedance-1-5-pro%';
