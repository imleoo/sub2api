-- Migration: 152_update_lingjing_prices
-- 把京东云灵境 2026-06 官方刊例价（JoySpace 内部表格抄录）写入 model_pricings。
--
-- 关键约定（与 model_catalog_seed.go:194-198 现有 Seedance 处理一致）：
--   image_generation: output_cost_per_image = USD per image
--   video_generation: output_cost_per_image = USD per second （字段被复用为 per-second 单价）
--
-- 汇率：1 USD = 7.28 CNY（同 pricing_service.go 上游注释）
-- 代表性档位：5s + 720p + 单镜头 + 非音画同步（不同模型按可用最近档位换算后除以时长得到 ¥/s）
--   - 不在 JoySpace 表中的模型（v6 / HappyHorse-1.0 / cinema-generate-2.0 / S2V-01 等）保持 unpriced
--   - 同名同 model_id 但跨视频+图片两种用途的（viduq1/viduq2/kling-v2-1/kling-video-o1/Kling-V3）
--     按视频单价入库，图片用法暂用相同 model_id 共享价格
--
-- 同时插入 3 个新模型 + 6 个常见别名（用户实际 API 调用中观察到的非官方拼法）。
-- 所有别名/新增使用 ON CONFLICT(model_id) DO UPDATE 把价格落到位。

-- ============================================================================
-- 1. 更新已有 23 行的 output_cost_per_image + pricing_status
-- ============================================================================
UPDATE model_pricings SET
    output_cost_per_image = 0.04121,
    pricing_status        = 'priced',
    last_synced_at        = NOW(),
    updated_at            = NOW()
WHERE model_id = 'kling-v2-5-turbo';

UPDATE model_pricings SET output_cost_per_image = 0.08242, pricing_status = 'priced', last_synced_at = NOW(), updated_at = NOW() WHERE model_id = 'kling-video-o1';
UPDATE model_pricings SET output_cost_per_image = 0.06868, pricing_status = 'priced', last_synced_at = NOW(), updated_at = NOW() WHERE model_id = 'kling-v2-6';
UPDATE model_pricings SET output_cost_per_image = 0.08242, pricing_status = 'priced', last_synced_at = NOW(), updated_at = NOW() WHERE model_id = 'Kling-V3';
UPDATE model_pricings SET output_cost_per_image = 0.05495, pricing_status = 'priced', last_synced_at = NOW(), updated_at = NOW() WHERE model_id = 'kling-v2-1';
UPDATE model_pricings SET output_cost_per_image = 0.05495, pricing_status = 'priced', last_synced_at = NOW(), updated_at = NOW() WHERE model_id = 'kling-v1-6';
UPDATE model_pricings SET output_cost_per_image = 0.05495, pricing_status = 'priced', last_synced_at = NOW(), updated_at = NOW() WHERE model_id = 'kling-v2';

UPDATE model_pricings SET output_cost_per_image = 0.04579, pricing_status = 'priced', last_synced_at = NOW(), updated_at = NOW() WHERE model_id = 'MiniMax-Hailuo-02';
UPDATE model_pricings SET output_cost_per_image = 0.04579, pricing_status = 'priced', last_synced_at = NOW(), updated_at = NOW() WHERE model_id = 'MiniMax-Hailuo-2.3';
UPDATE model_pricings SET output_cost_per_image = 0.03091, pricing_status = 'priced', last_synced_at = NOW(), updated_at = NOW() WHERE model_id = 'MiniMax-Hailuo-2.3-Fast';
UPDATE model_pricings SET output_cost_per_image = 0.00412, pricing_status = 'priced', last_synced_at = NOW(), updated_at = NOW() WHERE model_id = 'image-01';

UPDATE model_pricings SET output_cost_per_image = 0.06868, pricing_status = 'priced', last_synced_at = NOW(), updated_at = NOW() WHERE model_id = 'viduq1';
UPDATE model_pricings SET output_cost_per_image = 0.02995, pricing_status = 'priced', last_synced_at = NOW(), updated_at = NOW() WHERE model_id = 'viduq2';
UPDATE model_pricings SET output_cost_per_image = 0.12912, pricing_status = 'priced', last_synced_at = NOW(), updated_at = NOW() WHERE model_id = 'viduq3-pro';
UPDATE model_pricings SET output_cost_per_image = 0.08585, pricing_status = 'priced', last_synced_at = NOW(), updated_at = NOW() WHERE model_id = 'vidu2.0';
UPDATE model_pricings SET output_cost_per_image = 0.05357, pricing_status = 'priced', last_synced_at = NOW(), updated_at = NOW() WHERE model_id = 'viduq2-pro';

UPDATE model_pricings SET output_cost_per_image = 0.03626, pricing_status = 'priced', last_synced_at = NOW(), updated_at = NOW() WHERE model_id = 'v5';
UPDATE model_pricings SET output_cost_per_image = 0.03654, pricing_status = 'priced', last_synced_at = NOW(), updated_at = NOW() WHERE model_id = 'v5.5';

UPDATE model_pricings SET output_cost_per_image = 0.02747, pricing_status = 'priced', last_synced_at = NOW(), updated_at = NOW() WHERE model_id = 'doubao-seedream-4-0-250828';
UPDATE model_pricings SET output_cost_per_image = 0.03434, pricing_status = 'priced', last_synced_at = NOW(), updated_at = NOW() WHERE model_id = 'doubao-seedream-4-5-251128';
UPDATE model_pricings SET output_cost_per_image = 0.01099, pricing_status = 'priced', last_synced_at = NOW(), updated_at = NOW() WHERE model_id = 'Doubao-Seedance-1.5-pro';

-- ============================================================================
-- 2. 新增 3 个 JoySpace 表里有但官方 docs 暂未列入的模型
--    （viduq2-turbo / v5.6 / OmniHuman1.5）
--    apiId 暂未确认的两个不进 Go catalog 但价格先落库，等管理员补 apiId
-- ============================================================================
INSERT INTO model_pricings (
    model_id, display_name, provider, mode, source, pricing_status, is_enabled, is_custom, output_cost_per_image
) VALUES
    ('viduq2-turbo',   'Vidu Q2 Turbo',                   'lingjing', 'video_generation', 'lingjing', 'priced', TRUE, TRUE, 0.03228),
    ('v5.6',           'Paiwo V5.6',                      'lingjing', 'video_generation', 'lingjing', 'priced', TRUE, TRUE, 0.02747),
    ('OmniHuman1.5',   '豆包 OmniHuman 1.5 数字人',         'lingjing', 'video_generation', 'lingjing', 'priced', TRUE, TRUE, 0.13736)
ON CONFLICT (model_id) DO UPDATE SET
    output_cost_per_image = EXCLUDED.output_cost_per_image,
    pricing_status        = EXCLUDED.pricing_status,
    display_name          = COALESCE(model_pricings.display_name, EXCLUDED.display_name),
    last_synced_at        = NOW(),
    updated_at            = NOW();

-- ============================================================================
-- 3. 用户实际配置中常见的别名 / 短名（与 JoySpace 表里写法一致）
--    这些与 docs 官方 model_name 不同，但已被生产环境用到，独立入库以便 billing 命中
-- ============================================================================
INSERT INTO model_pricings (
    model_id, display_name, provider, mode, source, pricing_status, is_enabled, is_custom, output_cost_per_image
) VALUES
    ('kling-v3',                 'Kling V3（别名）',             'lingjing', 'video_generation', 'lingjing', 'priced', TRUE, TRUE, 0.08242),
    ('kling-O1',                 'Kling O1（别名）',             'lingjing', 'video_generation', 'lingjing', 'priced', TRUE, TRUE, 0.08242),
    ('kling-2.6',                'Kling 2.6（别名）',            'lingjing', 'video_generation', 'lingjing', 'priced', TRUE, TRUE, 0.06868),
    ('seedream4.0',              'Seedream 4.0（短名）',         'lingjing', 'image_generation', 'lingjing', 'priced', TRUE, TRUE, 0.02747),
    ('seedream4.5',              'Seedream 4.5（短名）',         'lingjing', 'image_generation', 'lingjing', 'priced', TRUE, TRUE, 0.03434),
    ('doubao-seedance-1-5-pro',  '豆包 Seedance 1.5 Pro（短名）', 'lingjing', 'video_generation', 'lingjing', 'priced', TRUE, TRUE, 0.01099)
ON CONFLICT (model_id) DO UPDATE SET
    output_cost_per_image = EXCLUDED.output_cost_per_image,
    pricing_status        = EXCLUDED.pricing_status,
    display_name          = COALESCE(model_pricings.display_name, EXCLUDED.display_name),
    last_synced_at        = NOW(),
    updated_at            = NOW();
