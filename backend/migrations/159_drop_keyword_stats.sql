-- 移除提示词词云分析插件（promptanalytics）遗留的 keyword_stats 表
-- 该插件已从代码中完全删除，此处清理其数据表与索引。
-- 索引随表 DROP 一并删除，无需单独 DROP INDEX。

DROP TABLE IF EXISTS keyword_stats;
