-- Migration: 151_seed_lingjing_models
-- 注册京东云灵境平台全部官方模型到 model_pricings 表。
-- 数据来源：docs.jdcloud.com/cn/lingjing 各产品子页面（2026-06-03 抓取）
--
-- 关键决策：
-- 1) model_id = model_name（与 docs 上 params.model / params.model_name 字段值严格一致）
-- 2) provider 统一 = 'lingjing'，source = 'lingjing'
-- 3) 价格统一不在迁移中赋值：
--    - 已计价的 Seedream / Seedance / cinema-generate-2.0 由代码内置静态价（pricing_service.go）兜底
--    - 其余模型 pricing_status='unpriced'，等管理员在 Admin UI 中补价
-- 4) ON CONFLICT (model_id) DO NOTHING：保留管理员已自定义的任何记录（包括已计价老模型），
--    迁移仅负责"新增缺失行"而不覆盖现有任何字段。

INSERT INTO model_pricings (
    model_id, display_name, provider, mode, source, pricing_status, is_enabled, is_custom
)
VALUES
    -- 可灵系列 (Kling) =====================================================
    ('kling-v2-5-turbo',           '可灵 V2.5 Turbo',           'lingjing', 'video_generation', 'lingjing', 'unpriced', TRUE, TRUE),
    ('kling-video-o1',             '可灵 O1',                   'lingjing', 'video_generation', 'lingjing', 'unpriced', TRUE, TRUE),
    ('kling-v2-6',                 '可灵 V2.6',                 'lingjing', 'video_generation', 'lingjing', 'unpriced', TRUE, TRUE),
    ('Kling-V3',                   '可灵 V3',                   'lingjing', 'video_generation', 'lingjing', 'unpriced', TRUE, TRUE),
    ('kling-v2-1',                 '可灵 V2.1',                 'lingjing', 'video_generation', 'lingjing', 'unpriced', TRUE, TRUE),
    ('kling-v1-6',                 '可灵 V1.6',                 'lingjing', 'video_generation', 'lingjing', 'unpriced', TRUE, TRUE),
    ('kling-v2',                   '可灵 V2（图生图）',          'lingjing', 'image_generation', 'lingjing', 'unpriced', TRUE, TRUE),

    -- 海螺系列 (MiniMax Hailuo) ============================================
    ('MiniMax-Hailuo-02',          '海螺 02',                   'lingjing', 'video_generation', 'lingjing', 'unpriced', TRUE, TRUE),
    ('MiniMax-Hailuo-2.3',         '海螺 2.3',                  'lingjing', 'video_generation', 'lingjing', 'unpriced', TRUE, TRUE),
    ('MiniMax-Hailuo-2.3-Fast',    '海螺 2.3 极速',             'lingjing', 'video_generation', 'lingjing', 'unpriced', TRUE, TRUE),
    ('S2V-01',                     '海螺 S2V-01 主体参考',       'lingjing', 'video_generation', 'lingjing', 'unpriced', TRUE, TRUE),
    ('image-01',                   '海螺 image-01 图片生成',     'lingjing', 'image_generation', 'lingjing', 'unpriced', TRUE, TRUE),

    -- Vidu 系列 ============================================================
    ('viduq1',                     'Vidu Q1',                  'lingjing', 'video_generation', 'lingjing', 'unpriced', TRUE, TRUE),
    ('viduq2',                     'Vidu Q2',                  'lingjing', 'video_generation', 'lingjing', 'unpriced', TRUE, TRUE),
    ('viduq3-pro',                 'Vidu Q3-Pro',              'lingjing', 'video_generation', 'lingjing', 'unpriced', TRUE, TRUE),
    ('vidu2.0',                    'Vidu 2.0',                 'lingjing', 'video_generation', 'lingjing', 'unpriced', TRUE, TRUE),
    ('viduq2-pro',                 'Vidu Q2-Pro',              'lingjing', 'video_generation', 'lingjing', 'unpriced', TRUE, TRUE),

    -- 拍我 / Pixverse 系列 =================================================
    ('v5',                         'Paiwo V5',                 'lingjing', 'video_generation', 'lingjing', 'unpriced', TRUE, TRUE),
    ('v5.5',                       'Paiwo V5.5',               'lingjing', 'video_generation', 'lingjing', 'unpriced', TRUE, TRUE),
    ('v6',                         'Pixverse V6',              'lingjing', 'video_generation', 'lingjing', 'unpriced', TRUE, TRUE),

    -- 豆包系列 (Doubao) ====================================================
    -- 以下 4 个已在 pricing_service.go 有静态价格，迁移只补元信息（DO NOTHING 保护现有行）
    ('doubao-seedream-4-0-250828', '豆包 Seedream 4.0',         'lingjing', 'image_generation', 'lingjing', 'priced',   TRUE, TRUE),
    ('doubao-seedream-4-5-251128', '豆包 Seedream 4.5',         'lingjing', 'image_generation', 'lingjing', 'priced',   TRUE, TRUE),
    ('Doubao-Seedream-5.0-lite',   '豆包 Seedream 5.0 Lite',    'lingjing', 'image_generation', 'lingjing', 'priced',   TRUE, TRUE),
    ('Doubao-Seedance-1.5-pro',    '豆包 Seedance 1.5 Pro',     'lingjing', 'video_generation', 'lingjing', 'priced',   TRUE, TRUE),

    -- Happy Horse 系列 =====================================================
    ('HappyHorse-1.0',             'Happy Horse 1.0',          'lingjing', 'video_generation', 'lingjing', 'unpriced', TRUE, TRUE),

    -- 历史遗留（代码已有 apiId=754 映射，但 pricing_service.go 没有静态价）==
    ('cinema-generate-2.0',        '豆包 Seedance 2.0 参考生视频', 'lingjing', 'video_generation', 'lingjing', 'unpriced', TRUE, TRUE)

ON CONFLICT (model_id) DO NOTHING;
