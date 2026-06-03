# 京东云灵境平台模型清单

来源：docs.jdcloud.com/cn/lingjing（2026-06-03 抓取）

## 全部模型一览（按 model_name 唯一聚合）

| # | model_name | provider | mode | apiIds (按任务类型) | 备注 |
|---|---|---|---|---|---|
| 1 | `kling-v2-5-turbo` | lingjing | video | 551 (ttv) | 可灵 V2.5 Turbo，支持 std/pro |
| 2 | `kling-video-o1` | lingjing | video | 560 (ttv) / 561 (ptv) / 562 (rtv) | 可灵 O1 |
| 3 | `kling-v2-6` | lingjing | video | 563 (ttv) / 564 (ptv) | 可灵 V2.6，仅 pro，支持 sound |
| 4 | `Kling-V3` | lingjing | video | 565 (ttv) / 566 (ptv) / 567 (picture) | 可灵 V3，跨视频/图片 |
| 5 | `kling-v2-1` | lingjing | video | 550 (ptv) / 553 (picture) | 可灵 V2.1 |
| 6 | `kling-v1-6` | lingjing | video | 552 (rtv) | 可灵 V1.6 参考生视频 |
| 7 | `kling-v2` | lingjing | image | 554 (picture) | 可灵 V2 图生图 |
| 8 | `MiniMax-Hailuo-02` | lingjing | video | 458 (ttv) / 457 (ptv) | 海螺 02 |
| 9 | `MiniMax-Hailuo-2.3` | lingjing | video | 460 (ttv) / 461 (ptv) | 海螺 2.3 |
| 10 | `MiniMax-Hailuo-2.3-Fast` | lingjing | video | 462 (ptv) | 海螺 2.3 极速版 |
| 11 | `S2V-01` | lingjing | video | 459 (rtv) | 海螺主体参考视频 |
| 12 | `image-01` | lingjing | image | 456 (picture) | 海螺图片生成 |
| 13 | `viduq1` | lingjing | video | 7 (ttv) / 0 (ptv) / 4 (rtv) / 16 (picture) | Vidu Q1 |
| 14 | `viduq2` | lingjing | video | 20 (ttv) / 21 (rtv) / 22 (picture) | Vidu Q2 |
| 15 | `viduq3-pro` | lingjing | video | 23 (ttv) / 24 (ptv) | Vidu Q3-Pro |
| 16 | `vidu2.0` | lingjing | video | 2 (ptv) / 5 (rtv) | Vidu 2.0 |
| 17 | `viduq2-pro` | lingjing | video | 17 (ptv) / 19 (ptv 单图) | Vidu Q2-Pro |
| 18 | `v5` | lingjing | video | 400 (ttv) / 501 (ptv) / 502 (ptv API) / 503 (rtv) | Paiwo V5 |
| 19 | `v5.5` | lingjing | video | 401 (ttv) / 402 (ptv) | Paiwo V5.5 |
| 20 | `v6` | lingjing | video | 505 (ttv) / 504 (ptv) | Pixverse V6 |
| 21 | `doubao-seedream-4-0-250828` | lingjing | image | 700 (picture) | 豆包 Seedream 4.0 ⭐已计价 |
| 22 | `doubao-seedream-4-5-251128` | lingjing | image | 701 (picture) | 豆包 Seedream 4.5 ⭐已计价 |
| 23 | `Doubao-Seedream-5.0-lite` | lingjing | image | 707 (picture) | 豆包 Seedream 5.0 Lite ⭐已计价 |
| 24 | `Doubao-Seedance-1.5-pro` | lingjing | video | 750 (ttv) / 751 (ptv) | 豆包 Seedance 1.5 Pro ⭐已计价 |
| 25 | `HappyHorse-1.0` | lingjing | video | 200202 (ttv) / 200203 (ptv) / 200204 (rtv) | Happy Horse 1.0 |
| 26 | `cinema-generate-2.0` | lingjing | video | 754 | 已有遗留映射（Seedance 2.0 参考生视频）⭐已计价 |

## 不入模型表的接口（工具类）

- `703` — 数字人识别 + 数字人视频生成（用法不同，复合接口，按需后续接入）

## 价格说明

- ⭐ 标记的 6 个模型在 `pricing_service.go` 已有静态价格，迁移使用 `ON CONFLICT (model_id) DO NOTHING` **保留现状**
- 其余 20+ 模型先以 `pricing_status='unpriced'` 入库，等管理员后续在 Admin UI 中补价
- 价格因京东云灵境官方页要登录才能查看，迁移阶段不强行赋值
