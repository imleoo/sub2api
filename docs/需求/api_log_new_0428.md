# AI 调用日志批量查询接口文档

## 一、基础信息
- 接口地址：https://XXX/api/v1/logs　　<!-- [改] 路径从 /api/logs 改为 /api/v1/logs，与现有接口规范一致 -->
- 请求方式：POST
- 接口描述：查询当前 API Key 所属用户的调用明细数据　　<!-- [改] 补充了"当前 API Key 所属用户"的范围说明 -->
- 编码格式：UTF-8
- 响应格式：application/json

## 二、请求头（Headers）
| Header | 必填 | 说明 |
|--------|------|------|
| Authorization | 是 | Bearer {api_key}，使用用户在系统中创建的 API Key |　　<!-- [新增] 认证 header，原文档缺失 -->
| Content-Type | 是 | application/json |　　<!-- [新增] POST body 必须声明 -->
| Accept | 否 | application/json |

> **说明**：认证使用用户自己在分组下创建的 API Key（即调用 AI 模型时使用的同一个 key）。系统通过 key 识别用户身份，查询结果仅返回该用户自己的数据。　　<!-- [新增] 认证机制说明 -->

## 三、请求体（Body JSON）　　<!-- [改] 原标题为"请求查询参数（Query）"，实际 curl 示例是 JSON body，统一为 Body JSON -->

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| beginTime | Long | 是 | 开始时间，UTC 时间戳（秒） |
| endTime | Long | 是 | 结束时间，UTC 时间戳（秒） |
| model | String | 否 | 模型名称过滤 |
| apiKey | String | 否 | 按指定 API Key 过滤（仅限当前用户名下的 key） |　　<!-- [改] 补充了"仅限当前用户名下"的限制说明 -->
| page | Integer | 否 | 页码，默认 1 |　　<!-- [新增] 分页参数，防止大数据量查询 -->
| pageSize | Integer | 否 | 每页条数，默认 20，最大 100 |　　<!-- [新增] 分页参数 -->

## 四、响应数据结构

```json
{
    "code": 200,
    "message": "success",
    "data": {
        "logs": [],
        "total": 0,
        "page": 1,
        "pageSize": 20
    }
}
```
<!-- [改] 原响应只有 {"logs": []}，补充了系统标准的 code/message/data 包装结构，以及 total/page/pageSize 分页字段 -->

### logs 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| request_id | string | 调用唯一请求 ID |
| request_time | string | 请求调用时间，UTC 时间戳（秒） |
| model | string | 调用模型名称 |
| input_token | number | 输入 token 数 |
| output_token | number | 输出 token 数 |
| cache_write_token | number | 缓存写入 token 数（对应 cache_creation_tokens） |
| cache_read_token | number | 缓存读取 token 数 |
| api_key | string | 本次调用使用的 API Key（脱敏展示，如 sk-***xxxx） |　　<!-- [改] 补充脱敏说明，原文无此说明 -->
| amount | number | 本次消耗费用（美元，对应 actual_cost） |　　<!-- [改] 补充了对应数据库字段 actual_cost -->

<!-- [删除] thought_token 字段：当前系统无此字段，已移除 -->
> **注意**：`thought_token`（推理思考 token）当前系统不记录该字段，已移除。

## 五、错误码

| 错误码 | 描述 |
|--------|------|
| 200 | 成功 |
| 400 | 请求参数错误（如时间范围缺失） |
| 401 | 认证失败（API Key 无效或缺失） |　　<!-- [新增] 原文档缺少 401 -->
| 404 | 未找到相关数据 |
| 500 | 服务器内部错误 |

## 六、示例 CURL 请求

```bash
curl "https://XXX/api/v1/logs" \
  -H "Authorization: Bearer sk-your-api-key" \　　# [改] 补充认证 header
  -H "Content-Type: application/json" \　　# [新增]
  -H "Accept: application/json" \
  -X POST \
  -d '{
      "beginTime": 1745000000,
      "endTime": 1745086400,
      "model": "claude-3-5-sonnet",　　
      "apiKey": "sk-xxxxx",
      "page": 1,　　
      "pageSize": 20　　
    }'
```