=== kelingttv ===
---

## 1. 快速开始：Curl 调用示例

通过该统一端点提交可灵视频生成任务。请根据所需的模型版本从下表选择对应的 `apiId` 与 `model_name`。

```bash
curl -X POST "https://model.jdcloud.com/joycreator/openApi/submitTask" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${your_app_key}" \
  -H "x-jdcloud-request-id: ${request-id or trace-id}" \
  -d '{
      "apiId": "[填入 apiId]",
      "params": {
        "prompt": "在此输入您的视频创意描述",
        "model_name": "[填入模型名称]",
        "duration": "5",
        "mode": "pro",
        "aspect_ratio": "16:9",
        "sound": "off",
        "multi_shot": "false",
        "shot_type": "intelligence"
      }
  }'

```

> **注意**：`multi_shot` 与 `shot_type` 字段目前仅在 **apiId: 565 (Kling-V3)** 中生效。

---

## 2. 模型版本与参数特征映射

可灵系列不同版本的接口 ID 及参数支持对照表：

| 模型版本 | 接口 ID (apiId) | 模型名称 (model_name) | 特有参数/模式支持 |
| --- | --- | --- | --- |
| **Kling-V2.5-Turbo** | `551` | `kling-v2-5-turbo` | 支持 `std` (标准) 与 `pro` (专业) 模式 |
| **Kling-O1** | `560` | `kling-video-o1` | 支持 `pro` 与 `std` 模式 |
| **Kling-V2.6** | `563` | `kling-v2-6` | 仅支持 `pro` 模式；支持同步音频 `sound` |
| **Kling-V3** | `565` | `Kling-V3` | 支持多镜头 `multi_shot`、分镜方式 `shot_type` 及 3-15s 时长 |

---

## 3. 请求参数详细说明 (params)

配置生成任务的具体属性：

| 字段名 | 类型 | 必填 | 详细说明 |
| --- | --- | --- | --- |
| **prompt** | string | 是 | 视频生成提示词 |
| **model_name** | string | 是 | 必须与上方对照表中的名称严格一致 |
| **duration** | string | 是 | **V2.x/O1**: `"5"`, `"10"`；**V3**: 可选 `"3"` 至 `"15"` 之间的字符串 |
| **mode** | string | 是 | 视频模式：`"pro"` (专业) 或 `"std"` (标准) |
| **aspect_ratio** | string | 是 | 宽高比：`"16:9"`, `"9:16"`, `"1:1"` |
| **shot_type** | string | 条件必填 | **仅 V3 支持**：分镜方式，可选值：`"intelligence"` |
| **sound** | string | 否 | 同步音频（仅 **563/565** 支持）：`"on"`, `"off"` |
| **multi_shot** | string | 否 | **仅 V3 支持**：生成多镜头，可选 `true`, `false` |

> **提示**：若涉及图生参考，图片 URL 地址必须保证公网可访问。

---

## 4. 输出响应说明 (JSON)

接口提交成功后，将返回统一的任务回执：

```json
{
	"apiKey": "${your_api_key}",
	"appId": "${your_app_id}",
	"error": "INVALID_API_KEY",
	"errorParamName": "${some one param}",
	"genTaskId": "任务ID:用于查询任务状态",
	"requestId": "${your_request_id}",
	"success": true
}

``` 

=== kelingptv ===

## 1. 快速开始：Curl 调用示例

通过该统一端点提交可灵图生视频任务。请根据所需的模型版本从下表选择对应的 `apiId` 与 `model_name` 填入请求体。

```bash
curl -X POST "https://model.jdcloud.com/joycreator/openApi/submitTask" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${your_app_key}" \
  -H "x-jdcloud-request-id: ${request-id or trace-id}" \
  -d '{
      "apiId": "[填入对应 apiId]",
      "params": {
        "image": "首帧图片URL",
        "image_tail": "可选：尾帧图片URL",
        "prompt": "视频内容的描述词",
        "model_name": "[填入模型名称]",
        "duration": "5",
        "mode": "pro",
        "sound": "off",
        "multi_shot": "false",
        "shot_type": "intelligence"
      }
  }'

```

> **注意**：`multi_shot` 与 `shot_type` 仅在 **Kling-V3 (566)** 中生效。

---

## 2. 模型版本与参数特征映射

可灵系列图生视频模型在参数结构上存在差异，请注意 `apiId` 的对应关系：

| 模型版本 | 接口 ID (apiId) | 模型名称 (model_name) | 特有参数/模式支持 |
| --- | --- | --- | --- |
| **Kling-V2.1** | `550` | `kling-v2-1` | 支持首尾帧控制 (`image`, `image_tail`) |
| **Kling-V2.6** | `564` | `kling-v2-6` | 仅支持 `pro` 模式；支持同步音频 `sound` |
| **Kling-O1** | `561` | `kling-video-o1` | 使用 `image_urls` (数组) 代替单图字段 |
| **Kling-V3** | `566` | `Kling-V3` | 支持首尾帧、多镜头 `multi_shot` 及 3-15s 时长 |

---

## 3. 请求参数详细说明 (params)

配置生成任务的具体属性：

| 字段名 | 类型 | 必填 | 详细说明 |
| --- | --- | --- | --- |
| **prompt** | string | 是 | 视频内容描述词 |
| **model_name** | string | 是 | 必须与上方对照表中的名称严格一致 |
| **duration** | string/int | 是 | **V2.x/O1**: `5`, `10`；**V3**: `"3"` 至 `"15"` (字符串) |
| **mode** | string | 是 | 视频模式：`"pro"` (专业) 或 `"std"` (标准) |
| **shot_type** | string | 条件必填 | **仅 V3 (566) 支持**：分镜方式，可选值：`"intelligence"` |
| **image** | string | 条件必填 | 首帧图片 URL (适用于 550, 564, 566) |
| **image_tail** | string | 否 | 尾帧图片 URL (适用于 550, 564, 566) |
| **image_urls** | stringArray | 条件必填 | 仅用于 **Kling-O1 (561)** 的参考图片列表 |
| **sound** | string | 否 | 同步音频 (支持 564, 566)：`"on"`, `"off"` |
| **multi_shot** | string | 否 | **仅 V3 (566) 支持**：生成多镜头，可选 `true`, `false` |

---

## 4. 业务逻辑与约束

* **图片访问**：所有图片 URL 地址必须保证公网可访问。
* **V2.6 限制**：Kling-V2.6 图生视频 (`564`) 仅支持 `"pro"` 模式。
* **V3 灵活性**：Kling-V3 (`566`) 提供最广泛的时长选择（3-15s）及音频同步支持。
* **O1 结构**：Kling-O1 (`561`) 采用数组结构提交参考图，且支持手动调节 `aspect_ratio`。

---

## 5. 输出响应说明 (JSON)

接口提交成功后，返回统一的任务回执：

```json
{
	"apiKey": "${your_api_key}",
	"appId": "${your_app_id}",
	"error": "INVALID_API_KEY",
	"errorParamName": "${some one param}",
	"genTaskId": "任务ID:用于查询任务状态",
	"requestId": "${your_request_id}",
	"success": true
}

```


=== kelingrtv ===

## 1. 快速开始：Curl 调用示例

通过该统一端点提交参考生视频任务。请根据所选模型版本配置对应的 `apiId` 与参考参数（`image_references` 或 `ref_video`/`image_list`）。

```bash
curl -X POST "https://model.jdcloud.com/joycreator/openApi/submitTask" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${your_app_key}" \
  -H "x-jdcloud-request-id: ${request-id or trace-id}" \
  -d '{
      "apiId":"[对应接口ID]",
      "params":{
        "prompt": "在此输入您的视频创意描述",
        "model_name": "[对应模型名称]",
        "duration": "5",
        "mode": "pro",
        "aspect_ratio": "16:9",
        "image_references": [],
        "ref_video": "视频URL",
        "image_list": []
      }
  }'

```

---

## 2. 模型版本与参数特征映射

可灵参考生视频支持“多图参考”或“视频+图片参考”两种主流模式：

| 模型版本 | 接口 ID (apiId) | 模型名称 (model_name) | 核心参考参数 | 支持时长 (duration) |
| --- | --- | --- | --- | --- |
| **Kling-V1.6** | `552` | `kling-v1-6` | `image_references` (多图) | `"5"`, `"10"` |
| **Kling-O1** | `562` | `kling-video-o1` | `ref_video`, `image_list` | `"3"`, `"5"`, `"8"`, `"10"` |

---

## 3. 请求参数详细说明 (params)

配置生成任务的具体属性：

| 字段名 | 适用模型 | 类型 | 必填 | 详细说明 |
| --- | --- | --- | --- | --- |
| **prompt** | 全部 | string | 是 | 视频生成提示词 |
| **model_name** | 全部 | string | 是 | 必须与上方对照表中的名称严格一致 |
| **duration** | 全部 | string | 是 | 视频时长（注意 O1 模型支持更多档位） |
| **mode** | 全部 | string | 是 | 视频模式：`"pro"` (专业) 或 `"std"` (标准) |
| **aspect_ratio** | 全部 | string | 是 | 宽高比：`"16:9"`, `"9:16"`, `"1:1"` |
| **image_references** | V1.6 | objectArray | 否 | 多张参考图片的对象数组 |
| **ref_video** | O1 | string | 否 | 参考视频 URL |
| **image_list** | O1 | objectArray | 否 | 参考图片列表对象数组 |

> **重要提示**：所有上传的图片和视频 URL 地址必须保证公网可访问。

---

## 4. 输出响应说明 (JSON)

接口提交成功后，将返回统一的任务回执：

```json
{
	"apiKey": "${your_api_key}",
	"appId": "${your_app_id}",
	"error": "INVALID_API_KEY",
	"errorParamName": "${some one param}",
	"genTaskId": "任务ID:用于查询任务状态",
	"requestId": "${your_request_id}",
	"success": true
}

```



=== kelingpicture ===
## 1. 快速开始：Curl 调用示例

通过该统一端点提交可灵图片生成任务。请根据您选择的模型版本传入对应的必填参数，**请勿将不同版本的专有参数混用**。

```bash
curl -X POST "https://model.jdcloud.com/joycreator/openApi/submitTask" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${your_app_key}" \
  -H "x-jdcloud-request-id: ${request-id or trace-id}" \
  -d '{
      "apiId":"[填写对应接口ID，例如 V3 为 567]",
      "params":{
        "// V3 模型必填及专有参数": "",
        "text": "在此输入您的图片描述词 (V3使用，最大2500字符)",
        "model": "Kling-V3",
        "resolution": "1k",
        "image_urls": ["https://example.com/image.jpg"],
        "image_reference": "subject",

        "// V2 / V2.1 模型必填及专有参数": "",
        "prompt": "在此输入您的图片描述词 (V2/V2.1使用)",
        "model_name": "[对应V2模型名称，如 kling-v2-1]",
        "taskNum": 1,
        "image": "https://example.com/image.jpg",

        "// 通用参数": "",
        "aspect_ratio": "16:9"
      }
  }'
```
*(注：以上请求体仅为展示所有可用字段的聚合示例。实际调用时，请严格对照下方参数说明表，剔除不属于该版本的参数。)*

---

## 2. 模型版本与参数特征映射

根据您的生成模式（文生图或图生图）和版本需求选择合适的配置：

| 任务类别 | 模型版本 | 接口 ID (apiId) | 模型名称标识 | 核心特征 |
| --- | --- | --- | --- | --- |
| **文生图 / 参考生图** | **Kling-V3** | `567` | `model`: `Kling-V3` | 支持 1k/2k 分辨率，支持多比例生成，支持图片参考特征控制（主体/人脸） |
| **文生图** | Kling-V2.1 | `553` | `model_name`: `kling-v2-1` | 纯文本驱动生成 |
| **图生图 (参考生图)** | Kling-V2 | `554` | `model_name`: `kling-v2` | 支持 `image` 参数作为参考 |

---

## 3. 请求参数详细说明 (params)

无特殊说明则参数必传。

| 字段名 | 适用模型 | 类型 | 必填 | 详细说明 |
| --- | --- | --- | --- | --- |
| **text** | V3 | string | 是 | 提示词，最大长度2500字符 |
| **prompt** | V2.1 / V2 | string | 是 | 图片生成正向提示词 |
| **model** | V3 | string | 是 | 模型名称，固定值: `Kling-V3` |
| **model_name** | V2.1 / V2 | string | 是 | 必须与上方对照表中的名称严格一致 |
| **resolution** | V3 | string | 是 | 清晰度：`"1k"`, `"2k"`，默认 `1k` |
| **aspect_ratio** | 全部 | string | V3否 / V2是 | 图片纵横比。V3支持：`"21:9"`, `"16:9"`, `"9:16"`, `"4:3"`, `"3:4"`, `"3:2"`, `"2:3"`, `"1:1"`。V2支持：`"16:9"`, `"9:16"`, `"1:1"`, `"4:3"`, `"3:4"`。默认均为 `16:9`。 |
| **taskNum** | V2.1 / V2 | int | 是 | 单次生成的图片数量，范围：`1` ~ `4` |
| **image_urls** | V3 | stringArray | 否 | 图片URL列表，支持上传图片，非必传。传值时为图生图模式。文件限制：大小10MB，格式.jpg/.jpeg/.png |
| **image_reference**| V3 | string | 否 | 图片参考类型：`"subject"` (主体), `"face"` (人脸)，默认 `subject`（仅图生图模式有效） |
| **image** | V2 | string | 否 | 参考图片的公网可访问 URL |

> **注意**：所有包含参考图片的参数（`image_urls`, `image`）均须保证 URL 在公网环境下可访问。

---

## 4. 输出响应说明 (JSON)

接口提交成功后，将返回统一的任务回执：

```json
{
	"apiKey": "${your_api_key}",
	"appId": "${your_app_id}",
	"error": "INVALID_API_KEY",
	"errorParamName": "${some one param}",
	"genTaskId": "任务ID:用于查询任务状态",
	"requestId": "${your_request_id}",
	"success": true
}
```

=== hailuottv ===

## 1. 快速开始：Curl 调用示例

通过该接口可以向服务器提交视频生成任务。请根据下表选择对应的 `apiId` 与 `model` 字段。

```bash
curl -X POST "https://model.jdcloud.com/joycreator/openApi/submitTask" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${your_app_key}" \
  -H "x-jdcloud-request-id: ${request-id or trace-id}" \
  -d '{
      "apiId":"[填入 apiId]",
      "params":{
        "prompt": "在此输入您的视频创意描述",
        "model": "[填入模型名称]",
        "duration": 6,
        "resolution": "1080P"
      }
  }'

```

---

## 2. 模型版本与鉴权参数映射

不同版本的海螺模型对应不同的 API 标识符：

| 模型版本 | 接口标识符 (apiId) | 模型名称 (model) |
| --- | --- | --- |
| **Hailuo-02** | `458` | `MiniMax-Hailuo-02` |
| **Hailuo-2.3**  | `460` | `MiniMax-Hailuo-2.3` |

---

## 3. 请求参数结构说明

在 `params` 字段中，需配置以下视频生成属性：

| 字段名 | 类型 | 详细说明 |
| --- | --- | --- |
| **prompt** | string | 视频生成提示词 |
| **model** | string | 模型名称，需与版本对应 |
| **duration** | int | 视频时长，可选值范围：`6`, `10`（单位：秒） |
| **resolution** | string | 分辨率，可选值：`1080P`, `768P` |

> **开发提示**：若涉及图片参考，图片 URL 地址必须保证在公网环境下可访问。

---

## 4. 业务逻辑与参数约束 (Duration vs Resolution)

视频的分辨率受限于选定的时长，配置时请遵循以下约束：

| 视频时长 (duration) | 允许的分辨率 (resolution) 取值范围 |
| --- | --- |
| **6 秒** | 可选 `768P` 或 `1080P` |
| **10 秒** | 仅支持 `768P` |

---

## 5. 响应报文与任务回执

接口调用后将返回任务回执，您需记录 `genTaskId` 以便后续查询视频生成状态。

```json
{
	"apiKey": "${your_api_key}",
	"appId": "${your_app_id}",
	"error": "INVALID_API_KEY",
	"errorParamName": "${some one param}",
	"genTaskId": "任务ID:用于查询任务状态",
	"requestId": "${your_request_id}",
	"success": true
}

```



=== hailuoptv ===

## 1. 快速开始：Curl 调用示例

通过该接口可提交图生视频任务。请根据您的业务需求选择对应的 `apiId` 与 `model` 参数。

```bash
curl -X POST "https://model.jdcloud.com/joycreator/openApi/submitTask" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${your_app_key}" \
  -H "x-jdcloud-request-id: ${request-id or trace-id}" \
  -d '{
      "apiId":"[填入 apiId]",
      "params":{
        "first_frame_image": "https://example.com/image1.jpg",
        "last_frame_image": "https://example.com/image2.jpg", 
        "prompt": "在此输入您的视频动态描述",
        "model": "[填入模型名称]",
        "duration": 6,
        "resolution": "1080P"
      }
  }'

```

> **注意**：`last_frame_image` 仅在 **Hailuo-02** 模型下生效。

---

## 2. 模型版本与参数特征映射

各版本模型在功能支持和接口 ID 上存在细微差异：

| 模型版本 | 接口标识 (apiId) | 模型名称 (model) | 特有参数支持 |
| --- | --- | --- | --- |
| **Hailuo-02** | `457` | `MiniMax-Hailuo-02` | 支持首帧+尾帧控制 (`last_frame_image`) |
| **Hailuo-2.3** | `461` | `MiniMax-Hailuo-2.3` | 仅支持首帧控制 |
| **Hailuo-2.3-Fast** | `462` | `MiniMax-Hailuo-2.3-Fast` | 仅支持首帧控制，侧重生成速度 |

---

## 3. 请求参数结构说明

在 `params` 字典中配置以下字段：

| 字段名 | 类型 | 必填 | 详细说明 |
| --- | --- | --- | --- |
| **first_frame_image** | string | 是 | 视频起始帧图片的公网可访问 URL |
| **last_frame_image** | string | 否 | 视频结束帧图片 URL（仅 **Hailuo-02** 支持） |
| **prompt** | string | 是 | 描述视频运动、风格或内容的提示词 |
| **model** | string | 是 | 模型名称，需与版本对应 |
| **duration** | int | 是 | 视频时长：`6` 或 `10` 秒 |
| **resolution** | string | 是 | 视频分辨率：`1080P` 或 `768P` |

---

## 4. 业务逻辑约束 (时长与分辨率)

为了保证生成质量，分辨率的选择受限于视频时长限制：

| 视频时长 (duration) | 允许的分辨率 (resolution) 取值 |
| --- | --- |
| **6 秒** | `768P` (768p) 或 `1080P` (1080p) |
| **10 秒** | 仅支持 `768P` (768p) |

---

## 5. 输出响应说明

接口调用成功后返回任务回执，您需通过 `genTaskId` 查询任务状态。

```json
{
	"apiKey": "${your_api_key}",
	"appId": "${your_app_id}",
	"error": "INVALID_API_KEY",
	"errorParamName": "${some one param}",
	"genTaskId": "任务ID:用于查询任务状态",
	"requestId": "${your_request_id}",
	"success": true
}

```


=== hailuortv ===

## 1. 快速开始：Curl 调用示例

通过该接口可以提交基于主体参考的视频生成任务。

```bash
curl -X POST "https://model.jdcloud.com/joycreator/openApi/submitTask" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${your_app_key}" \
  -H "x-jdcloud-request-id: ${request-id or trace-id}" \
  -d '{
      "apiId":"459",
      "params":{
        "subject_reference": {},
        "prompt": "在此输入您的视频创意描述",
        "model": "S2V-01"
      }
  }'

```

---

## 2. 请求参数详细说明 (params)

配置生成任务的具体属性：

| 字段名 | 类型 | 详细说明 |
| --- | --- | --- |
| **subject_reference** | object | 主体参考对象 |
| **prompt** | string | 视频生成提示词 |
| **model** | string | 固定值为 `S2V-01` |

> **注意**：图片 URL 地址必须保证在公网环境下可访问。

---

## 3. 输出响应说明 (JSON)

接口调用成功后返回任务回执，您需通过 `genTaskId` 查询任务状态。

```json
{
	"apiKey": "${your_api_key}",
	"appId": "${your_app_id}",
	"error": "INVALID_API_KEY",
	"errorParamName": "${some one param}",
	"genTaskId": "任务ID:用于查询任务状态",
	"requestId": "${your_request_id}",
	"success": true
}

```


=== hailuottp ===
## 1. 快速开始：Curl 调用示例

通过该接口可以向服务器提交图片生成任务。

```bash
curl -X POST "https://model.jdcloud.com/joycreator/openApi/submitTask" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${your_app_key}" \
  -H "x-jdcloud-request-id: ${request-id or trace-id}" \
  -d '{
      "apiId":"456",
      "params":{
        "subject_reference": ["https://example.com/image.jpg"],
        "prompt": "在此输入您的图片描述词",
        "model": "image-01",
        "aspect_ratio": "16:9"
      }
  }'

```

---

## 2. 请求参数详细说明 (params)

配置图片生成的具体属性：

| 字段名 | 类型 | 详细说明 |
| --- | --- | --- |
| **subject_reference** | stringArray | 主体参考图的 URL 数组 |
| **prompt** | string | 图片生成提示词 |
| **model** | string | 固定值为 `image-01` |
| **aspect_ratio** | string | 生成图片的比例，可选值：`16:9`, `9:16`, `1:1`, `4:3`, `3:4` |

> **注意**：所有参考图片 URL 地址必须保证公网可访问。

---

## 3. 输出响应说明 (JSON)

接口提交成功后，将返回包含任务 ID 的响应报文，用于后续查询生成结果：

```json
{
	"apiKey": "${your_api_key}",
	"appId": "${your_app_id}",
	"error": "INVALID_API_KEY",
	"errorParamName": "${some one param}",
	"genTaskId": "任务ID:用于查询任务状态",
	"requestId": "${your_request_id}",
	"success": true
}

```



=== viduttv ===
## 1. 快速开始：Curl 调用示例

通过统一端点提交 Vidu 文生视频生成任务。请根据所需模型版本选择对应的 `apiId` 与 `model`。

*(注：以下示例包含了所有可能用到的参数，实际调用时请对照下方参数表，选择当前模型支持的参数)*

```bash
curl -X POST "https://model.jdcloud.com/joycreator/openApi/submitTask" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${your_app_key}" \
  -H "x-jdcloud-request-id: ${request-id or trace-id}" \
  -d '{
      "apiId":"[对应接口ID，例如 23]",
      "params":{
        "prompt": "在此输入您的视频创意描述",
        "model": "[对应模型名称，例如 viduq3-pro]",
        "duration": 5,
        "resolution": "1080p",
        "aspect_ratio": "16:9",
        "style": "general",
        "bgm": true,
        "audio": true,
        "movement_amplitude": "auto"
      }
  }'
```

---

## 2. 模型版本与参数对照表

Vidu 系列不同版本的接口 ID 及功能支持如下：

| 模型版本 | 接口 ID (apiId) | 模型名称 (model) | 支持时长 (duration) | 支持比例 (aspect_ratio) | 特有参数 / 备注 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Vidu Q1** | `7` | `viduq1` | `5` | `16:9`, `9:16`, `1:1` | 支持运动幅度 `movement_amplitude` |
| **Vidu Q2** | `20` | `viduq2` | `5`, `8`, `10` | `16:9`, `9:16`, `4:3`, `3:4`, `1:1` | 支持更多比例和时长 |
| **Vidu Q3-Pro** | `23` | `viduq3-pro` | `1` ~ `16` 的整数 | `16:9`, `9:16`, `4:3`, `3:4`, `1:1` | 支持同步音频 `audio` |

---

## 3. 详细参数说明 (params)

配置生成任务的具体属性：

| 字段名 | 类型 | 必填 | 详细说明 |
| :--- | :--- | :--- | :--- |
| **prompt** | string | 是 | 视频生成提示词。 |
| **model** | string | 是 | 必须与上方对照表中的名称严格一致（如 `viduq3-pro`）。 |
| **duration** | int | 是 | 视频时长（秒）。**Q3-Pro 支持 1 至 16 秒**；Q1 为 `5`；Q2 支持 `5`, `8`, `10`。 |
| **resolution** | string | 是 | 分辨率。Q1 仅限 `"1080p"`；Q2 与 Q3-Pro 可选 `"540p"`, `"720p"`, `"1080p"`。 |
| **aspect_ratio** | string | 是 | 视频比例。Q1 支持 3 种；Q2 与 Q3-Pro 支持 5 种（详见上方对照表）。 |
| **style** | string | 否 | 风格：`"general"` (通用), `"anime"` (动漫)。 |
| **bgm** | boolean | 否 | 是否开启背景音乐：`true`, `false`。 |
| **audio** | boolean | 否 | 是否生成同步音频：`true`, `false` **(仅 Vidu Q3-Pro 支持)**。 |
| **movement_amplitude** | string | 否 | 运动幅度：`"auto"`, `"small"`, `"medium"`, `"large"` **(仅 Vidu Q1 支持)**。 |

> **⚠️ 注意**：如果提示词中包含或关联了外部图片/视频素材的引用，URL 地址必须保证公网可访问。

---

## 4. 输出响应说明 (JSON)

接口提交后将返回任务回执。成功时返回 `SUCCESS` 及任务 ID；失败时将返回相应的错误码及出错的参数名。

```json
{
  "apiKey": "${your_api_key}",
  "appId": "${your_app_id}",
  "error": "SUCCESS 或 错误代码(如 INVALID_API_KEY)",
  "errorParamName": "报错的参数名称(仅在出错时返回)",
  "genTaskId": "任务ID:用于查询任务状态",
  "requestId": "${your_request_id}",
  "success": true
}
```

=== viduptv ===
## 1. 快速开始：接口调用示例

通过统一的 API 端点提交 Vidu 图生视频任务。请根据所选模型配置对应的 `apiId` 与 `params`。

*(注：以下示例包含了所有可能用到的参数，实际调用时请对照下方参数表，剔除当前模型不支持的特有参数)*

```bash
curl -X POST "https://model.jdcloud.com/joycreator/openApi/submitTask" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${your_app_key}" \
  -H "x-jdcloud-request-id: ${request-id or trace-id}" \
  -d '{
      "apiId":"[对应接口ID，例如 24]",
      "params":{
        "images": ["图片URL1"],
        "prompt": "视频内容的动态描述",
        "model": "[对应模型名称，例如 viduq3-pro]",
        "duration": 4,
        "resolution": "1080p",
        "movement_amplitude": "auto",
        "taskNum": 1,
        "bgm": true,
        "audio": true
      }
  }'
```

---

## 2. 模型版本与参数特征映射

Vidu 系列图生视频接口根据模型能力划分为以下版本，请严格对应 `apiId` 与 `model` 字段：

| 模型版本 | 接口 ID (apiId) | 模型名称 (model) | 支持时长 (duration) | 支持分辨率 (resolution) | 特有参数 / 备注 |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Vidu 2.0** | `2` | `vidu2.0` | `4`, `8` | `360p`, `720p`, `1080p` | 支持背景音乐 `bgm` |
| **Vidu Q1** | `0` | `viduq1` | `5` | `1080p` | 支持背景音乐 `bgm` |
| **Vidu Q2-pro** | `17` | `viduq2-pro` | `4`, `8` | `720p`, `1080p` | 支持多任务生成 `taskNum` (1-4) |
| **Vidu Q2-pro (单图)** | `19` | `viduq2-pro` | `4`, `8` | `720p`, `1080p` | 侧重单张图片生成 |
| **Vidu Q3-Pro** | `24` | `viduq3-pro` | `1` ~ `16` 的整数 | `540p`, `720p`, `1080p` | 支持同步音频 `audio` |

---

## 3. 核心参数详细说明 (params)

| 字段名 | 类型 | 必填 | 详细说明 |
| :--- | :--- | :--- | :--- |
| **images** | stringArray | 是 | 参考图片 URL 列表。即便只有单图，也需使用数组格式，如 `["url"]`。 |
| **prompt** | string | 是 | 描述视频的动态变化及视觉细节。 |
| **model** | string | 是 | 必须与上方对照表中的模型名称严格一致（如 `viduq3-pro`）。 |
| **duration** | int | 是 | 视频时长（秒）。**Q3-Pro 支持 1 至 16 秒**；其余模型仅支持固定的 `4`, `5`, `8`。 |
| **resolution** | string | 是 | 视频分辨率：可选 `"1080p"`, `"720p"`, `"540p"`, `"360p"` (具体支持列表依模型而定)。 |
| **movement_amplitude** | string | 否 | 运动幅度：`"auto"`, `"small"`, `"medium"`, `"large"`。 |
| **bgm** | boolean | 否 | 是否生成背景音乐：`true`, `false` **(仅 Vidu 2.0 / Q1 支持)**。 |
| **audio** | boolean | 否 | 是否生成同步音频：`true`, `false` **(仅 Vidu Q3-Pro 支持)**。 |
| **taskNum** | int | 否 | 生成数量：`1` ~ `4` **(仅接口 17 且模型为 Q2-pro 时支持)**。 |

> **⚠️ 安全提示**：所有图片 URL 地址必须保证公网可访问。

---

## 4. 输出响应说明 (JSON)

接口提交后将返回任务回执。成功时返回 `SUCCESS` 及任务 ID；失败时将返回相应的错误码（如 `INVALID_API_KEY`）及出错的参数名。

```json
{
  "apiKey": "${your_api_key}",
  "appId": "${your_app_id}",
  "error": "SUCCESS 或 错误代码(如 INVALID_API_KEY)",
  "errorParamName": "报错的参数名称(仅在出错时返回)",
  "genTaskId": "任务ID:用于查询任务状态",
  "requestId": "${your_request_id}",
  "success": true
}
```



=== vidurtv ===

## 1. 快速开始：Curl 调用示例

通过该统一端点提交任务，根据需要参考的精细度及模型版本选择对应的 `apiId`。

```bash
curl -X POST "https://model.jdcloud.com/joycreator/openApi/submitTask" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${your_app_key}" \
  -H "x-jdcloud-request-id: ${request-id or trace-id}" \
  -d '{
      "apiId":"[对应接口ID]",
      "params":{
        "image_references": [
          {
            "to": "subject",
            "url": "参考图URL"
          }
        ],
        "prompt": "视频内容的详细描述",
        "model": "[对应模型名称]",
        "duration": 5,
        "resolution": "1080p",
        "aspect_ratio": "16:9",
        "bgm": true
      }
  }'

```

---

## 2. 模型版本与参数对照表

Vidu 参考生视频支持多维度控制，不同模型的能力分布如下：

| 模型版本 | 接口 ID | 模型名称 (model) | 支持时长 (duration) | 比例 (aspect_ratio) | 分辨率支持 |
| --- | --- | --- | --- | --- | --- |
| **Vidu 2.0** | `5` | `vidu2.0` | `4` | `16:9`, `9:16`, `1:1` | `360p`, `720p` |
| **Vidu Q1** | `4` | `viduq1` | `5` | `16:9`, `9:16`, `1:1` | `1080p` |
| **Vidu Q2** | `21` | `viduq2` | `5`, `8`, `10` | `16:9`, `9:16`, `4:3`, `3:4`, `1:1` | `540p`, `720p`, `1080p` |

---

## 3. 核心参数详细说明 (params)

| 字段名 | 类型 | 必填 | 详细说明 |
| --- | --- | --- | --- |
| **image_references** | objectArray | 是 | 参考图数组。包含 `url` (图片地址) 和 `to` (参考类型，如 `"subject"`) |
| **prompt** | string | 是 | 视频生成提示词，建议描述动态变化 |
| **model** | string | 是 | 必须与上方对照表中的模型名称严格一致 |
| **duration** | int | 是 | 视频时长。Q2 模型支持最长 `10` 秒 |
| **resolution** | string | 是 | 分辨率：`"1080p"`, `"720p"`, `"540p"`, `"360p"` |
| **aspect_ratio** | string | 是 | 宽高比。Q2 支持最全面的比例选择 |
| **movement_amplitude** | string | 否 | **仅 2.0/Q1 支持**：`"auto"`, `"small"`, `"medium"`, `"large"` |
| **bgm** | boolean | 否 | 是否开启背景音乐：`true`, `false` |

> **开发建议**：参考图 URL 必须保证公网可访问。为了获得最佳一致性，建议参考图主体清晰。

---

## 4. 输出响应说明 (JSON)

接口提交成功后，将返回统一的任务回执：

```json
{
	"apiKey": "${your_api_key}",
	"appId": "${your_app_id}",
	"error": "SUCCESS",
	"genTaskId": "任务ID:用于查询任务状态",
	"requestId": "${your_request_id}",
	"success": true
}

```



=== vidurpicture ===

## 1. 快速开始：Curl 调用示例

通过该统一端点提交任务。Vidu 的生图接口统一使用 `images` 数组来处理参考图（如为纯文生图，该数组可为空或不传，具体视模型要求而定）。

```bash
curl -X POST "https://model.jdcloud.com/joycreator/openApi/submitTask" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${your_app_key}" \
  -H "x-jdcloud-request-id: ${request-id or trace-id}" \
  -d '{
      "apiId":"[对应接口ID]",
      "params":{
        "prompt": "在此输入您的图片创意描述",
        "model": "[对应模型名称]",
        "aspect_ratio": "16:9",
        "taskNum": 1,
        "images": ["可选：参考图URL"],
        "resolution": "1080p"
      }
  }'

```

---

## 2. 模型版本与参数对照表

根据您的画质需求和参考维度选择对应的配置：

| 模型版本 | 接口 ID | 模型名称 (model) | 支持比例 (aspect_ratio) | 分辨率 (resolution) |
| --- | --- | --- | --- | --- |
| **Vidu Q1** | `16` | `viduq1` | `16:9`, `9:16`, `1:1` | 默认高质量输出 |
| **Vidu Q2** | `22` | `viduq2` | `16:9`, `9:16`, `4:3`, `3:4`, `1:1` | `1080p`, `2K`, `4K` |

---

## 3. 详细参数说明 (params)

| 字段名 | 类型 | 必填 | 详细说明 |
| --- | --- | --- | --- |
| **prompt** | string | 是 | 图片生成的提示词 |
| **model** | string | 是 | 必须与上方对照表中的名称严格一致 |
| **aspect_ratio** | string | 是 | 比例。Q2 模型提供了更丰富的传统摄影比例（如 4:3） |
| **taskNum** | int | 是 | 单次生成的图片数量，范围：`1` ~ `4` |
| **images** | stringArray | 否 | 参考图片 URL 数组。格式为 `["http://..."]` |
| **resolution** | string | 否 | **仅 Q2 支持**：可指定 `"1080p"`, `"2K"`, `"4K"` |

> **开发要点**：
> 1. Vidu Q2 支持最高 **4K** 级别的超清图片输出。
> 2. 所有上传的参考图片 URL 必须保证在公网环境下可免密访问。
> 
> 

---

## 4. 输出响应说明 (JSON)

接口提交成功后，将返回统一的任务回执：

```json
{
	"apiKey": "${your_api_key}",
	"appId": "${your_app_id}",
	"error": "INVALID_API_KEY",
	"errorParamName": "${some one param}",
	"genTaskId": "任务ID:用于查询任务状态",
	"requestId": "${your_request_id}",
	"success": true
}

```


=== paiwottv ===

## 1. 快速开始：Curl 调用示例

使用统一端点提交任务，根据所需功能（如是否需要同步音频或多镜头）选择对应的 `apiId`。对于 V6 模型，其对应的 `apiId` 为 `505`。

```bash
curl -X POST "https://model.jdcloud.com/joycreator/openApi/submitTask" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${your_app_key}" \
  -H "x-jdcloud-request-id: ${request-id or trace-id}" \
  -d '{
      "apiId":"[对应接口ID]",
      "params":{
        "prompt": "描述您的视频画面创意",
        "model": "[对应模型名称]",
        "duration": 5,
        "quality": "1080p",
        "aspect_ratio": "16:9",
        "generate_audio_switch": true,
        "generate_multi_clip_switch": false
      }
  }'

```

---

## 2. 模型版本与参数对照表

不同版本的核心差异如下：

| 模型版本 | 接口 ID | 模型名称 (model) | 时长 (duration) | 分辨率 (quality) | 支持比例 (aspect_ratio) | 特有功能 |
| --- | --- | --- | --- | --- | --- | --- |
| **Pixverse V6** | `505` | `v6` | `1`~`15` | `360p` ~ `1080p` | `16:9`, `4:3`, `3:4`, `1:1`, `9:16`, `2:3`, `3:2`, `21:9` | 同步音频、多镜头 |
| **Paiwo V5** | `400` | `v5` | `5`, `8` | `360p` ~ `1080p` | `16:9`, `9:16`, `4:3`, `3:4`, `1:1` | 经典通用版本 |
| **Paiwo V5.5** | `401` | `v5.5` | `5`, `8` | `540p`, `720p`, `1080p` | `16:9` | 同步音频、多镜头控制 |

---

## 3. 核心参数详细说明 (params)

| 字段名 | 类型 | 必填 | 详细说明 |
| --- | --- | --- | --- |
| **prompt** | string | 是 | 视频提示词，描述场景、主体及动作。 |
| **model** | string | 是 | 必须与上方对照表一致，填写 `v5`、`v5.5` 或 `v6`。 |
| **duration** | int | 是 | 视频参数：Paiwo 系列可选 `5` 或 `8` 秒；V6 支持 `[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15]`。 |
| **quality** | string | 是 | 分辨率：可选 `["360p","540p","720p","1080p"]`（推荐使用 `"1080p"` 以获得最高清晰度）。 |
| **aspect_ratio** | string/int | 是 | 比例：V5.5 目前仅支持 `"16:9"`；V5 支持多比例切换；V6 支持 `["16:9","4:3","3:4","1:1","9:16","2:3","3:2","21:9"]`。 |
| **generate_audio_switch** | boolean | 否 | **V5.5 及 V6 支持**：同步音频开关，控制是否同步生成视频配音/音效。 |
| **generate_multi_clip_switch** | boolean | 否 | **V5.5 及 V6 支持**：多镜头开关，控制是否开启多镜头/剪辑模式。 |

*注意：在 Pixverse V6 接口说明中还包含一项提示，即“图片URL地址必须公网可访问”，若在使用过程中需要扩展传入图片参数，需遵循此项规定。*

---

## 4. 输出响应说明 (JSON)

接口提交成功后，将返回统一的任务回执：

```json
{
	"apiKey": "${your_api_key}",
	"appId": "${your_app_id}",
	"error": "SUCCESS",
	"errorParamName": "${some one param}",
	"genTaskId": "任务ID:用于后续查询生成进度与结果",
	"requestId": "${your_request_id}",
	"success": true
}

```

=== paiwoptv ===

## 1. 快速开始：Curl 调用示例

通过统一端点提交任务，根据所需版本（V5、V5.5 或 V6）以及控制方式（单图或首尾帧）选择对应的 `apiId`。

```bash
curl -X POST "https://model.jdcloud.com/joycreator/openApi/submitTask" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${your_app_key}" \
  -H "x-jdcloud-request-id: ${request-id or trace-id}" \
  -d '{
      "apiId":"[对应接口ID]",
      "params":{
        "first_frame_img": "首帧图片URL",
        "last_frame_img": "可选：尾帧图片URL",
        "prompt": "在此描述视频中的动作或变化",
        "model": "[对应模型名称]",
        "duration": 5,
        "quality": "1080p",
        "generate_audio_switch": true,
        "generate_multi_clip_switch": false
      }
  }'
```
*(注：为体现 V6 新增参数，示例中补充了 `generate_multi_clip_switch`)*

---

## 2. 模型版本与参数对照表

| 模型版本 | 接口 ID | 控制类型 | 模型名称 (model) | 时长 (duration) | 最高分辨率 | 特有功能 |
| --- | --- | --- | --- | --- | --- | --- |
| **Pixverse V6** | `504` | 首尾帧/单图 | `v6` | `1`~`15` | `1080p` | 同步音频、多镜头控制 |
| **Paiwo V5.5** | `402` | 首尾帧/单图 | `v5.5` | `5`, `8` | `1080p` | 同步音频、多镜头控制 |
| **Paiwo V5** | `501` | 首尾帧/单图 | `v5` | `5`, `8` | `1080p` | 经典稳定版本 |
| **Paiwo V5** | `502` | 仅单图 (API专版) | `v5` | `5`, `8` | `1080p` | 使用 `img_id` 传参 |

---

## 3. 详细参数说明 (params)

| 字段名 | 类型 | 必填 | 详细说明 |
| --- | --- | --- | --- |
| **first_frame_img** | string | 是 | 视频起始帧图片的公网可访问 URL。 |
| **last_frame_img** | string | 否 | 视频结束帧图片的 URL。若不传则为单图生成模式。**注意：V6 开启多镜头时（即 `generate_multi_clip_switch` 为 true 或 false 时），此项的允许取值范围为 null。** |
| **img_id** | int | 否 | **仅接口 502 使用**：内部图片资源 ID（通常用于特定集成场景）。 |
| **prompt** | string | 是 | 描述视频中的动态效果、光影变化或动作逻辑。 |
| **model** | string | 是 | 必须填入 `v5`、`v5.5` 或 `v6`。 |
| **duration** | int | 是 | 视频时长：Paiwo 系列可选 `5` 或 `8` 秒；**V6 系列可选 `1` 到 `15` 之间的整数**。 |
| **quality** | string | 是 | 分辨率：可选 `"360p"`, `"540p"`, `"720p"`, `"1080p"`。 |
| **generate_audio_switch** | boolean | 否 | **仅 V5.5 / V6 支持**：是否自动为生成的视频匹配音效/配音。 |
| **generate_multi_clip_switch** | boolean | 否 | **仅 V5.5 / V6 支持**：是否开启自动剪辑/多镜头模式。 |

---

## 4. 输出响应说明 (JSON)

接口提交成功后，将返回统一的任务回执：

```json
{
	"apiKey": "${your_api_key}",
	"appId": "${your_app_id}",
	"error": "SUCCESS",
	"errorParamName": "${some one param}",
	"genTaskId": "T123456789",
	"requestId": "${your_request_id}",
	"success": true
}
```


=== paiwortv ===

## 1. 快速开始 (Curl 示例)

```bash
curl -X POST "https://model.jdcloud.com/joycreator/openApi/submitTask" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${your_app_key}" \
  -H "x-jdcloud-request-id: ${request-id or trace-id}" \
  -d '{
      "apiId":"503",
      "params":{
        "image_references": [
          {
            "to": "subject",
            "url": "https://example.com/your-image.jpg"
          }
        ],
        "prompt": "一个小男孩在草地上奔跑，阳光明媚，电影质感",
        "model": "v5",
        "duration": 5,
        "quality": "1080p",
        "aspect_ratio": "16:9"
      }
  }'

```

---

## 3. 参数详细说明 (params)

| 参数序号 | 字段名 | 类型 | 必填 | 字段说明与允许值 |
| --- | --- | --- | --- | --- |
| 0 | **image_references** | objectArray | 是 | 参考图配置，包含 `url` (图片地址) 和 `to` (参考维度) |
| 1 | **prompt** | string | 是 | 视频内容的文本描述，建议包含动作和环境细节 |
| 2 | **model** | string | 是 | 必须固定为：`v5` |
| 3 | **duration** | int | 是 | 视频时长。可选值：`5`, `8` (单位：秒) |
| 4 | **quality** | string | 是 | 分辨率。可选：`"360p"`, `"540p"`, `"720p"`, `"1080p"` |
| 5 | **aspect_ratio** | string/int | 是 | 宽高比。可选：`"16:9"`, `"9:16"`, `"4:3"`, `"3:4"`, `"1:1"` |

> **注意**：
> 1. `image_references` 中的图片 URL 必须保证在公网环境下可免密直接访问。
> 2. `to` 字段通常取值为 `"subject"` (主体参考) 或根据具体需求定义的维度。
> 
> 

---

## 4. 输出响应说明 (JSON)

接口提交成功后，将返回统一的任务回执：

```json
{
	"apiKey": "${your_api_key}",
	"appId": "${your_app_id}",
	"error": "SUCCESS",
	"genTaskId": "任务ID:用于查询任务状态",
	"requestId": "${your_request_id}",
	"success": true
}

```



=== doubaopic ===
## 1. 快速开始：Curl 调用示例

通过统一端点提交任务。Seedream 的生图接口支持通过 `image` 数组处理参考图（纯文生图时该数组可为空）。部分较新模型（如 5.0 Lite）还支持额外的特性控制参数。

```bash
curl -X POST "https://model.jdcloud.com/joycreator/openApi/submitTask" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${your_app_key}" \
  -H "x-jdcloud-request-id: ${request-id or trace-id}" \
  -d '{
      "apiId":"[对应接口ID]",
      "params":{
        "prompt": "在此输入您的图片创意描述",
        "model": "[对应模型名称]",
        "size": "2048x2048",
        "taskNum": 1,
        "image": ["可选：参考图URL"],
        "type": false 
      }
  }'
```

> **提示**：`type` 参数为联网搜索控制，目前仅特定模型（如 Seedream 5.0 Lite）支持，具体请参考下方参数说明。

-----

## 2. 模型版本与参数对照表

根据您的版本需求和生成维度选择对应的配置：

| 模型版本 | 接口 ID | 模型名称 (model) | 支持尺寸 (size) | 生成数量 (taskNum) |
| --- | --- | --- | --- | --- |
| **Seedream 4.0** | `700` | `doubao-seedream-4-0-250828` | 见下文详细尺寸列表 | 1-4 张 |
| **Seedream 4.5** | `701` | `doubao-seedream-4-5-251128` | 见下文详细尺寸列表 | 1-4 张 |
| **Seedream 5.0 Lite** | `707` | `Doubao-Seedream-5.0-lite` | 见下文详细尺寸列表 | 1-4 张 |

-----

## 3. 详细参数说明 (params)

| 字段名 | 类型 | 必填 | 详细说明 |
| --- | --- | --- | --- |
| **prompt** | string | 是 | 图片生成的文字描述。 |
| **model** | string | 是 | 必须填入对应的模型名称。 |
| **size** | string | 是 | 支持：`2048x2048`, `2304x1728`, `1728x2304`, `2560x1440`, `1440x2560`, `2496x1664`, `1664x2496`, `3024x1296`。 |
| **taskNum** | int | 是 | 单次任务生成的图片张数，范围 1-4。 |
| **image** | stringArray | 否 | 参考图 URL 数组。用于图生图或提供视觉特征参考。<br>**注意：图片 URL 地址必须公网可访问。** |
| **type** | boolean | 否 | 联网搜索开关。支持 `true` 或 `false`。*(注：目前主要适用于 Doubao-Seedream-5.0-lite)* |

### 参数制约关系说明

在某些特定参数组合下，字段之间存在制约关系（主要针对引入 `type` 联网搜索的新模型）：

| 主控制参数 | 被控制参数 | 主参数取值 | 被控制参数允许取值范围 |
| --- | --- | --- | --- |
| **image** | **type** | `null` | `null` (即未传入参考图时，联网搜索参数也不应生效或应置空) |

-----

## 4. 输出说明

任务提交成功后，系统返回以下格式的响应结果：

```json
{
  "apiKey":"${your_api_key}",
  "appId":"${your_app_id}",
  "error":"INVALID_API_KEY",
  "errorParamName":"${some one param}",
  "genTaskId":"任务ID:用于查询任务状态",
  "requestId":"${your_request_id}",
  "success":true
}
```

=== doubaottv ===
## 1. 快速开始：Curl 调用示例

通过该统一端点提交字节跳动 Seedance 视频生成任务。请根据下表填入对应的 `apiId` 与 `model_name`。

```bash
curl -X POST "https://model.jdcloud.com/joycreator/openApi/submitTask" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${your_app_key}" \
  -H "x-jdcloud-request-id: ${request-id or trace-id}" \
  -d '{
      "apiId": "750",
      "params": {
        "prompt": "在此输入您的视频创意描述",
        "model_name": "Doubao-Seedance-1.5-pro",
        "duration": "5",
        "mode": "720p",
        "aspect_ratio": "16:9",
        "generate_audio": true
      }
  }'

```

> **注意**：`generate_audio` 字段为布尔值（`true`/`false`），用于控制是否同步生成音频。

---

## 2. 模型版本与参数特征映射

字节系列文生视频模型版本及特有支持如下：

| 模型版本 | 接口 ID (apiId) | 模型名称 (model_name) | 特有参数/模式支持 |
| --- | --- | --- | --- |
| **Seedance 1.5 Pro** | `750` | `Doubao-Seedance-1.5-pro` | 支持多分辨率 (`mode`)、多种宽高比及同步音频 |

---

## 3. 请求参数详细说明 (params)

配置生成任务的具体属性：

| 字段名 | 类型 | 必填 | 详细说明 |
| --- | --- | --- | --- |
| **prompt** | string | 是 | 视频生成提示词。 |
| **model_name** | string | 是 | 必须固定为 `Doubao-Seedance-1.5-pro`。 |
| **duration** | string | 是 | 视频时长，可选值：`"5"`, `"10"`, `"12"`。 |
| **mode** | string | 是 | 视频分辨率：`"480p"`, `"720p"`, `"1080p"`。 |
| **aspect_ratio** | string | 是 | 宽高比：`"16:9"`, `"9:16"`, `"4:3"`, `"1:1"`, `"4:4"`, `"21:9"`。 |
| **generate_audio** | boolean | 否 | 是否同步生成音频：`true` (开启), `false` (关闭)。 |

> **提示**：若涉及图片参考（如后续扩展图生视频），图片 URL 地址必须保证公网可访问。

---

## 4. 输出响应说明 (JSON)

接口提交成功后，将返回统一的任务回执：

```json
{
	"apiKey": "${your_api_key}",
	"appId": "${your_app_id}",
	"error": "INVALID_API_KEY",
	"errorParamName": "${some one param}",
	"genTaskId": "任务ID:用于查询任务状态",
	"requestId": "${your_request_id}",
	"success": true
}

```



=== doubaoptv ===
## 1. 快速开始：Curl 调用示例

通过该统一端点提交字节跳动 Seedance 图生视频任务。请根据下表填入对应的 `apiId` 与 `model_name`。

```bash
curl -X POST "https://model.jdcloud.com/joycreator/openApi/submitTask" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${your_app_key}" \
  -H "x-jdcloud-request-id: ${request-id or trace-id}" \
  -d '{
      "apiId": "751",
      "params": {
        "image_urls": "图片URL1"
        "prompt": "在此输入您的视频创意描述",
        "model_name": "Doubao-Seedance-1.5-pro",
        "duration": "5",
        "mode": "720p",
        "aspect_ratio": "16:9",
        "generate_audio": true
      }
  }'

```

> **注意**：`image_urls` 为字符串数组格式；`generate_audio` 为布尔值。

---

## 2. 模型版本与参数特征映射

字节系列图生视频模型版本及特有支持如下：

| 模型版本 | 接口 ID (apiId) | 模型名称 (model_name) | 特有参数/模式支持 |
| --- | --- | --- | --- |
| **Seedance 1.5 Pro** | `751` | `Doubao-Seedance-1.5-pro` | 使用 `image_urls` 数组提交参考图 |

---

## 3. 请求参数详细说明 (params)

配置生成任务的具体属性：

| 字段名 | 类型 | 必填 | 详细说明 |
| --- | --- | --- | --- |
| **image_urls** | stringArray | 是 | 参考图片列表，传入图片 URL 地址。 |
| **prompt** | string | 是 | 视频生成提示词。 |
| **model_name** | string | 是 | 必须固定为 `Doubao-Seedance-1.5-pro`。 |
| **duration** | string | 是 | 视频时长，可选值：`"5"`, `"10"`, `"12"`。 |
| **mode** | string | 是 | 视频分辨率：`"480p"`, `"720p"`, `"1080p"`。 |
| **aspect_ratio** | string | 是 | 宽高比：`"16:9"`, `"9:16"`, `"4:3"`, `"1:1"`, `"4:4"`, `"21:9"`。 |
| **generate_audio** | boolean | 否 | 是否同步生成音频：`true` (开启), `false` (关闭)。 |

> **重要提示**：图片 URL 地址必须保证公网可访问。

---

## 4. 输出响应说明 (JSON)

接口提交成功后，将返回统一的任务回执：

```json
{
	"apiKey": "${your_api_key}",
	"appId": "${your_app_id}",
	"error": "INVALID_API_KEY",
	"errorParamName": "${some one param}",
	"genTaskId": "任务ID:用于查询任务状态",
	"requestId": "${your_request_id}",
	"success": true
}

```


=== bddighuman ===
# 接口一：数字人识别接口

## 接口说明

用于识别图片中的数字人，生成遮罩图片(maskUrl)，供数字人视频生成接口使用。

## 请求示例

```bash
curl --location 'https://model.jdcloud.com/joycreator/openApi/detection' \
  --header 'Authorization: Bearer ${your_app_key}' \
  --form 'file=@"/path/to/file"' \
  --form 'apiId="703"' \
  --form 'url="https://example.com/path/to/image.jpg"'
```

## 参数说明

| 参数名   | 类型     | 必填 | 说明                |
|-------|--------|----|-------------------|
| file  | file   | 否  | 上传的图片文件（与url二选一）  |
| url   | string | 否  | 图片URL地址（与file二选一） |
| apiId | string | 是  | API ID，固定值："703"  |

## 返回示例

```json
{
  "requestId": "s-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "error": null,
  "result": {
    "url": "https://example.com/path/to/image.jpg",
    "taskId": "task-xxxxxxxxxxxxxx"
  }
}
```

---

# 接口二：识别任务查询接口

## 接口说明

用于查询数字人识别任务的执行结果，获取生成的maskUrl和roleUrl。

## 请求示例

```bash
curl --location 'https://model.jdcloud.com/joycreator/openApi/task?taskId=task-xxxxxxxxxxxxxx' \
  --header 'Authorization: Bearer ${your_app_key}'
```

## 参数说明

| 参数名    | 类型     | 必填 | 说明             |
|--------|--------|----|----------------|
| taskId | string | 是  | 数字人识别接口返回的任务ID |

## 返回示例

```json
{
  "requestId": "s-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "error": null,
  "result": {
    "content": [
      {
        "id": "otid-xxxxxxxxxxxx",
        "maskUrl": "https://example.com/path/to/mask.png",
        "roleUrl": "https://example.com/path/to/role.png"
      }
    ],
    "status": 1
  }
}
```

## 返回字段说明

| 字段名               | 类型     | 说明                               |
|-------------------|--------|----------------------------------|
| status            | int    | 任务状态：0-处理中，1-处理成功                |
| content           | array  | 任务结果列表                           |
| content[].id      | string | 结果ID                             |
| content[].maskUrl | string | 数字人遮罩图片URL，用于数字人视频生成接口的maskUrl参数 |
| content[].roleUrl | string | 数字人角色图片URL                       |

---

# 接口三：数字人视频生成接口

## 接口说明

用于生成数字人视频，需配合数字人识别接口使用。

## 请求示例

```bash
curl -X POST "https://model.jdcloud.com/joycreator/openApi/submitTask" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${your_app_key}" \
  -H "x-jdcloud-request-id: ${request-id or trace-id}" \
  -d '{
      "apiId":"703",
      "params":{
        "imageUrl": "",
        "maskUrl": "",
        "audioUrl": "",
        "promptText": "",
        "model": "703",
        "resolution": ""
      }
  }'
```

## 参数说明

| 参数序号 | 字段名        | 字段类型   | 字段说明                                                                                               |
|------|------------|--------|----------------------------------------------------------------------------------------------------|
| 0    | imageUrl   | string | 数字人主图                                                                                              |
| 1    | maskUrl    | string | 数字人遮罩图片URL，通过接口二获取                                                                                 |
| 2    | audioUrl   | strin

=== t2vhappy_horse ===
## 1. 快速开始：Curl 调用示例

```bash
curl -X POST "https://model.jdcloud.com/joycreator/openApi/submitTask" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${your_app_key}" \
  -H "x-jdcloud-request-id: ${request-id or trace-id}" \
  -d '{
      "apiId":"200202",
      "params":{
        "prompt": "",
        "model_name": "HappyHorse-1.0",
        "duration": "",
        "resolution": "",
        "aspect_ratio": "",
        "watermark": "",
        "seed": ""
      }
  }'

```

---

## 2. 模型版本与参数对照表

| 模型版本 | 接口 ID | 模型名称 (model_name) |
| --- | --- | --- |
| **HappyHorse 1.0** | `200202` | `HappyHorse-1.0` |

---

## 3. 详细参数说明 (params)

| 字段名 | 类型 | 详细说明 |
| --- | --- | --- |
| **prompt** | string | :[] |
| **model_name** | string | 模型:HappyHorse-1.0 |
| **duration** | string | 视频参数:[3,4,5,6,7,8,9,10,11,12,13,14,15] |
| **resolution** | string | 清晰度:["720P","1080P"] |
| **aspect_ratio** | string | 宽高比:["16:9","9:16","4:3","1:1","3:4","4:5","5:4","9:21","21:9"] |
| **watermark** | boolean | 水印:[true,false] |
| **seed** | int | 随机种子:[] |

> **注意**：图片URL地址必须公网可访问

---

## 4. 输出说明

```json
{
  "appId":"${your_app_id}",
  "error":"INVALID_API_KEY",
  "errorParamName":"${some one param}",
  "genTaskId":"任务ID:用于查询任务状态",
  "requestId":"${your_request_id}",
  "success":true
}

```

=== i2vhappy_horse ===
## 1. 快速开始：Curl 调用示例

```bash
curl -X POST "https://model.jdcloud.com/joycreator/openApi/submitTask" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${your_app_key}" \
  -H "x-jdcloud-request-id: ${request-id or trace-id}" \
  -d '{
      "apiId":"200203",
      "params":{
        "image_urls": "",
        "prompt": "",
        "model_name": "HappyHorse-1.0",
        "duration": "",
        "resolution": "",
        "watermark": "",
        "seed": ""
      }
  }'

```

---

## 2. 模型版本与参数对照表

| 模型版本 | 接口 ID | 模型名称 (model_name) |
| --- | --- | --- |
| **HappyHorse 1.0** | `200203` | `HappyHorse-1.0` |

---

## 3. 详细参数说明 (params)

| 字段名 | 类型 | 详细说明 |
| --- | --- | --- |
| **image_urls** | string | :[] |
| **prompt** | string | null:[] |
| **model_name** | string | 模型:HappyHorse-1.0 |
| **duration** | string | 时长:[3,4,5,6,7,8,9,10,11,12,13,14,15] |
| **resolution** | string | 分辨率:["720P","1080P"] |
| **watermark** | boolean | 水印:[true,false] |
| **seed** | int | 随机种子:[] |

> **注意**：图片URL地址必须公网可访问

---

## 4. 输出说明

```json
{
  "appId":"${your_app_id}",
  "error":"INVALID_API_KEY",
  "errorParamName":"${some one param}",
  "genTaskId":"任务ID:用于查询任务状态",
  "requestId":"${your_request_id}",
  "success":true
}

```

=== r2vhappy_horse ===
## 1. 快速开始：Curl 调用示例

```bash
curl -X POST "https://model.jdcloud.com/joycreator/openApi/submitTask" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${your_app_key}" \
  -H "x-jdcloud-request-id: ${request-id or trace-id}" \
  -d '{
      "apiId":"200204",
      "params":{
        "multi_model_url": "",
        "prompt": "",
        "model_name": "HappyHorse-1.0",
        "duration": "",
        "resolution": "",
        "ratio": "",
        "watermark": "",
        "seed": ""
      }
  }'

```

---

## 2. 模型版本与参数对照表

| 模型版本 | 接口 ID | 模型名称 (model_name) |
| --- | --- | --- |
| **HappyHorse 1.0** | `200204` | `HappyHorse-1.0` |

---

## 3. 详细参数说明 (params)

| 字段名 | 类型 | 详细说明 |
| --- | --- | --- |
| **multi_model_url** | objectArray | null:[] |
| **prompt** | string | null:[] |
| **model_name** | string | 模型:HappyHorse-1.0 |
| **duration** | string | 时长:[3,4,5,6,7,8,9,10,11,12,13,14,15] |
| **resolution** | string | 分辨率:["720P","1080P"] |
| **ratio** | string | 宽高比:["16:9","9:16","1:1","4:3","3:4","4:5","5:4","9:21","21:9"] |
| **watermark** | boolean | 水印:[true,false] |
| **seed** | int | 随机种子:[] |

> **注意**：图片URL地址必须公网可访问

---

## 4. 输出说明

```json
{
  "appId":"${your_app_id}",
  "error":"INVALID_API_KEY",
  "errorParamName":"${some one param}",
  "genTaskId":"任务ID:用于查询任务状态",
  "requestId":"${your_request_id}",
  "success":true
}

```

