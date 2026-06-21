package service

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	lingjingImagePollInterval = 2 * time.Second
	lingjingImagePollMaxWait  = 60 * time.Second
)

// LingjingVideoRequest 视频生成任务请求参数。
type LingjingVideoRequest struct {
	Prompt        string   `json:"prompt"`
	TaskType      string   `json:"task_type"` // "text2video" / "image2video"
	Duration      string   `json:"duration"`  // "5" / "10" / "12"
	Mode          string   `json:"mode"`      // "480p" / "720p" / "1080p"
	AspectRatio   string   `json:"aspect_ratio"`
	GenerateAudio bool     `json:"generate_audio"`
	ImageURLs     []string `json:"image_urls,omitempty"` // 图生视频时必填
}

// LingjingImageRequest 生图任务请求参数。
type LingjingImageRequest struct {
	Prompt    string   `json:"prompt"`
	Model     string   `json:"model"`
	Size      string   `json:"size"`
	TaskNum   int      `json:"task_num"`
	Image     []string `json:"image,omitempty"` // 可选参考图
	TypeParam *bool    `json:"type,omitempty"`  // 联网搜索（5.0 Lite）
}

// LingjingImageResult 生图成功结果。
type LingjingImageResult struct {
	URLs      []string
	TaskCount int
}

// LingjingGatewayService 灵境平台网关服务。
type LingjingGatewayService struct {
	client            *LingjingClient
	taskRepo          LingjingTaskRepository
	accountRepo       AccountRepository
	schedulerSnapshot *SchedulerSnapshotService
}

// NewLingjingGatewayService 创建 LingjingGatewayService。
func NewLingjingGatewayService(
	client *LingjingClient,
	taskRepo LingjingTaskRepository,
	accountRepo AccountRepository,
	schedulerSnapshot *SchedulerSnapshotService,
) *LingjingGatewayService {
	return &LingjingGatewayService{
		client:            client,
		taskRepo:          taskRepo,
		accountRepo:       accountRepo,
		schedulerSnapshot: schedulerSnapshot,
	}
}

// SelectAccount 从调度快照中随机选取一个可用的灵境账号。
func (s *LingjingGatewayService) SelectAccount(ctx context.Context, groupID *int64, excludeIDs []int64) (*Account, error) {
	var accounts []Account
	var err error
	if s.schedulerSnapshot != nil {
		accounts, _, err = s.schedulerSnapshot.ListSchedulableAccounts(ctx, groupID, PlatformLingjing, false)
	} else {
		if groupID != nil {
			accounts, err = s.accountRepo.ListSchedulableByGroupIDAndPlatform(ctx, *groupID, PlatformLingjing)
		} else {
			accounts, err = s.accountRepo.ListSchedulableByPlatform(ctx, PlatformLingjing)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("lingjing: select account: %w", err)
	}
	exclude := make(map[int64]bool, len(excludeIDs))
	for _, id := range excludeIDs {
		exclude[id] = true
	}
	for i := range accounts {
		if !exclude[accounts[i].ID] {
			return &accounts[i], nil
		}
	}
	return nil, fmt.Errorf("lingjing: no available account")
}

// GetAPIKey 从账号凭证中取京东云 API Key。
func (s *LingjingGatewayService) GetAPIKey(account *Account) (string, error) {
	key := account.GetLingjingAPIKey()
	if key == "" {
		return "", fmt.Errorf("lingjing account %d: missing api_key credential", account.ID)
	}
	return key, nil
}

// ForwardSeedreamImage 同步执行生图请求。
// 内部提交任务后轮询直到完成（最长 60s），返回图片 URL 列表。
func (s *LingjingGatewayService) ForwardSeedreamImage(ctx context.Context, account *Account, req *LingjingImageRequest) (*LingjingImageResult, error) {
	apiKey, err := s.GetAPIKey(account)
	if err != nil {
		return nil, err
	}

	apiID := lingjingModelToAPIID(req.Model)
	params := map[string]any{
		"prompt":  req.Prompt,
		"model":   req.Model,
		"size":    req.Size,
		"taskNum": req.TaskNum,
	}
	if len(req.Image) > 0 {
		params["image"] = req.Image
	}
	if req.TypeParam != nil {
		params["type"] = *req.TypeParam
	}

	submitResp, err := s.client.SubmitTask(ctx, apiKey, LingjingSubmitRequest{
		APIID:  apiID,
		Params: params,
	})
	if err != nil {
		return nil, fmt.Errorf("lingjing image submit: %w", err)
	}

	queryResp, err := s.client.PollUntilDone(ctx, apiKey, submitResp.Result.Result.GenTaskID, lingjingImagePollInterval, lingjingImagePollMaxWait)
	if err != nil {
		return nil, fmt.Errorf("lingjing image poll: %w", err)
	}
	if queryResp.IsFailed() {
		return nil, fmt.Errorf("lingjing image task failed: %s", queryResp.Result.Result.Error)
	}

	urls := queryResp.SuccessURLs()
	if len(urls) == 0 {
		return nil, fmt.Errorf("lingjing image: no result URLs in successful task")
	}
	return &LingjingImageResult{URLs: urls, TaskCount: req.TaskNum}, nil
}

// SubmitVideoTask 提交视频生成任务（异步），立即返回已持久化的 LingjingTask。
func (s *LingjingGatewayService) SubmitVideoTask(ctx context.Context, account *Account, userID, apiKeyID int64, groupID *int64, req *LingjingVideoRequest) (*LingjingTask, error) {
	apiKey, err := s.GetAPIKey(account)
	if err != nil {
		return nil, err
	}

	apiID, modelName := lingjingVideoAPIInfo(req.TaskType)
	params := map[string]any{
		"prompt":         req.Prompt,
		"model_name":     modelName,
		"duration":       req.Duration,
		"mode":           req.Mode,
		"aspect_ratio":   req.AspectRatio,
		"generate_audio": req.GenerateAudio,
	}
	if req.TaskType == LingjingTaskTypeImage2Video {
		if len(req.ImageURLs) == 0 {
			return nil, fmt.Errorf("lingjing image2video: image_urls required")
		}
		params["image_urls"] = req.ImageURLs
	}

	submitResp, err := s.client.SubmitTask(ctx, apiKey, LingjingSubmitRequest{
		APIID:  apiID,
		Params: params,
	})
	if err != nil {
		return nil, fmt.Errorf("lingjing video submit: %w", err)
	}

	now := time.Now()
	task := &LingjingTask{
		GenTaskID:   submitResp.Result.Result.GenTaskID,
		TaskType:    req.TaskType,
		Status:      LingjingTaskStatusPending,
		RequestBody: params,
		UserID:      userID,
		APIKeyID:    apiKeyID,
		AccountID:   account.ID,
		GroupID:     groupID,
		Model:       modelName,
		Duration:    req.Duration,
		Mode:        req.Mode,
		StartedAt:   &now,
	}
	if err := s.taskRepo.Create(ctx, task); err != nil {
		return nil, fmt.Errorf("lingjing video: save task: %w", err)
	}
	return task, nil
}

// GetTaskStatus 查询任务状态（含权限校验）。
func (s *LingjingGatewayService) GetTaskStatus(ctx context.Context, taskID, apiKeyID int64) (*LingjingTask, error) {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.APIKeyID != apiKeyID {
		return nil, fmt.Errorf("lingjing: task %d not found", taskID)
	}
	return task, nil
}

// GetClient 返回底层客户端（供 PollRunner 使用）。
func (s *LingjingGatewayService) GetClient() *LingjingClient {
	return s.client
}

// lingjingModelCatalogEntry 描述一个灵境模型在不同任务类型下的 apiId 路由。
// 一个 model_name 可能对应多个 apiId（视频/图片/参考生等）。
type lingjingModelCatalogEntry struct {
	// IsImageGen 表明该模型主用途是图片生成（非视频）。
	IsImageGen bool
	// TextToVideoAPIID 文生视频 apiId（空 = 不支持）
	TextToVideoAPIID string
	// ImageToVideoAPIID 图生视频 apiId
	ImageToVideoAPIID string
	// RefToVideoAPIID 参考生视频 apiId
	RefToVideoAPIID string
	// PictureAPIID 图片生成 apiId
	PictureAPIID string
	// ModelKey 提交时 params 字段名（"model" 或 "model_name"），不同产品约定不同
	ModelKey string
	// SupportedTestAPIID 账号测试使用的首选 apiId（通常优先 ttv，否则 picture）
	SupportedTestAPIID string
}

// lingjingModelCatalog 全部灵境官方模型的统一映射。
// 数据来源：docs.jdcloud.com/cn/lingjing 全部子页面（2026-06-03 抓取）。
// 与 migrations/151_seed_lingjing_models.sql 中的 model_id 保持严格一致。
var lingjingModelCatalog = map[string]lingjingModelCatalogEntry{
	// 可灵 ----------------------------------------------------------------
	"kling-v2-5-turbo": {TextToVideoAPIID: LingjingAPIIDKlingV25TurboTTV, ModelKey: "model_name", SupportedTestAPIID: LingjingAPIIDKlingV25TurboTTV},
	"kling-video-o1":   {TextToVideoAPIID: LingjingAPIIDKlingO1TTV, ImageToVideoAPIID: LingjingAPIIDKlingO1PTV, RefToVideoAPIID: LingjingAPIIDKlingO1RTV, ModelKey: "model_name", SupportedTestAPIID: LingjingAPIIDKlingO1TTV},
	"kling-v2-6":       {TextToVideoAPIID: LingjingAPIIDKlingV26TTV, ImageToVideoAPIID: LingjingAPIIDKlingV26PTV, ModelKey: "model_name", SupportedTestAPIID: LingjingAPIIDKlingV26TTV},
	"Kling-V3":         {TextToVideoAPIID: LingjingAPIIDKlingV3TTV, ImageToVideoAPIID: LingjingAPIIDKlingV3PTV, PictureAPIID: LingjingAPIIDKlingV3Pic, ModelKey: "model_name", SupportedTestAPIID: LingjingAPIIDKlingV3TTV},
	"kling-v2-1":       {ImageToVideoAPIID: LingjingAPIIDKlingV21PTV, PictureAPIID: LingjingAPIIDKlingV21Pic, ModelKey: "model_name", SupportedTestAPIID: LingjingAPIIDKlingV21PTV},
	"kling-v1-6":       {RefToVideoAPIID: LingjingAPIIDKlingV16RTV, ModelKey: "model_name", SupportedTestAPIID: LingjingAPIIDKlingV16RTV},
	"kling-v2":         {IsImageGen: true, PictureAPIID: LingjingAPIIDKlingV2Pic, ModelKey: "model_name", SupportedTestAPIID: LingjingAPIIDKlingV2Pic},

	// 海螺 ----------------------------------------------------------------
	"MiniMax-Hailuo-02":       {TextToVideoAPIID: LingjingAPIIDHailuo02TTV, ImageToVideoAPIID: LingjingAPIIDHailuo02PTV, ModelKey: "model", SupportedTestAPIID: LingjingAPIIDHailuo02TTV},
	"MiniMax-Hailuo-2.3":      {TextToVideoAPIID: LingjingAPIIDHailuo23TTV, ImageToVideoAPIID: LingjingAPIIDHailuo23PTV, ModelKey: "model", SupportedTestAPIID: LingjingAPIIDHailuo23TTV},
	"MiniMax-Hailuo-2.3-Fast": {ImageToVideoAPIID: LingjingAPIIDHailuo23FastPTV, ModelKey: "model", SupportedTestAPIID: LingjingAPIIDHailuo23FastPTV},
	"S2V-01":                  {RefToVideoAPIID: LingjingAPIIDHailuoS2V01, ModelKey: "model", SupportedTestAPIID: LingjingAPIIDHailuoS2V01},
	"image-01":                {IsImageGen: true, PictureAPIID: LingjingAPIIDHailuoImage01, ModelKey: "model", SupportedTestAPIID: LingjingAPIIDHailuoImage01},

	// Vidu ----------------------------------------------------------------
	"viduq1":     {TextToVideoAPIID: LingjingAPIIDViduQ1TTV, ImageToVideoAPIID: LingjingAPIIDViduQ1PTV, RefToVideoAPIID: LingjingAPIIDViduQ1RTV, PictureAPIID: LingjingAPIIDViduQ1Pic, ModelKey: "model", SupportedTestAPIID: LingjingAPIIDViduQ1TTV},
	"viduq2":     {TextToVideoAPIID: LingjingAPIIDViduQ2TTV, RefToVideoAPIID: LingjingAPIIDViduQ2RTV, PictureAPIID: LingjingAPIIDViduQ2Pic, ModelKey: "model", SupportedTestAPIID: LingjingAPIIDViduQ2TTV},
	"viduq3-pro": {TextToVideoAPIID: LingjingAPIIDViduQ3ProTTV, ImageToVideoAPIID: LingjingAPIIDViduQ3ProPTV, ModelKey: "model", SupportedTestAPIID: LingjingAPIIDViduQ3ProTTV},
	"vidu2.0":    {ImageToVideoAPIID: LingjingAPIIDVidu20PTV, RefToVideoAPIID: LingjingAPIIDVidu20RTV, ModelKey: "model", SupportedTestAPIID: LingjingAPIIDVidu20PTV},
	"viduq2-pro": {ImageToVideoAPIID: LingjingAPIIDViduQ2ProPTV, ModelKey: "model", SupportedTestAPIID: LingjingAPIIDViduQ2ProPTV},

	// 拍我 / Pixverse ------------------------------------------------------
	"v5":   {TextToVideoAPIID: LingjingAPIIDPaiwoV5TTV, ImageToVideoAPIID: LingjingAPIIDPaiwoV5PTV, RefToVideoAPIID: LingjingAPIIDPaiwoV5RTV, ModelKey: "model", SupportedTestAPIID: LingjingAPIIDPaiwoV5TTV},
	"v5.5": {TextToVideoAPIID: LingjingAPIIDPaiwoV55TTV, ImageToVideoAPIID: LingjingAPIIDPaiwoV55PTV, ModelKey: "model", SupportedTestAPIID: LingjingAPIIDPaiwoV55TTV},
	"v6":   {TextToVideoAPIID: LingjingAPIIDPixverseV6TTV, ImageToVideoAPIID: LingjingAPIIDPixverseV6PTV, ModelKey: "model", SupportedTestAPIID: LingjingAPIIDPixverseV6TTV},

	// 豆包 ----------------------------------------------------------------
	"doubao-seedream-4-0-250828": {IsImageGen: true, PictureAPIID: LingjingAPIIDSeedream40, ModelKey: "model", SupportedTestAPIID: LingjingAPIIDSeedream40},
	"doubao-seedream-4-5-251128": {IsImageGen: true, PictureAPIID: LingjingAPIIDSeedream45, ModelKey: "model", SupportedTestAPIID: LingjingAPIIDSeedream45},
	"Doubao-Seedream-5.0-lite":   {IsImageGen: true, PictureAPIID: LingjingAPIIDSeedream5Lite, ModelKey: "model", SupportedTestAPIID: LingjingAPIIDSeedream5Lite},
	"Doubao-Seedance-1.5-pro":    {TextToVideoAPIID: LingjingAPIIDTextToVideo, ImageToVideoAPIID: LingjingAPIIDImageToVideo, ModelKey: "model_name", SupportedTestAPIID: LingjingAPIIDTextToVideo},

	// Happy Horse ---------------------------------------------------------
	"HappyHorse-1.0": {TextToVideoAPIID: LingjingAPIIDHappyHorse10TTV, ImageToVideoAPIID: LingjingAPIIDHappyHorse10PTV, RefToVideoAPIID: LingjingAPIIDHappyHorse10RTV, ModelKey: "model_name", SupportedTestAPIID: LingjingAPIIDHappyHorse10TTV},

	// 数字人 --------------------------------------------------------------
	// OmniHuman 1.5 是 3 步流程（detection → query → submitTask 同 apiId=703），
	// 简化测试时只占位 apiId，不构造标准 prompt 提交。
	"OmniHuman1.5": {RefToVideoAPIID: LingjingAPIIDOmniHuman, ModelKey: "model", SupportedTestAPIID: LingjingAPIIDOmniHuman},

	// 遗留 ----------------------------------------------------------------
	"cinema-generate-2.0": {RefToVideoAPIID: LingjingAPIIDCinemaGenerate20, ModelKey: "model_name", SupportedTestAPIID: LingjingAPIIDCinemaGenerate20},

	// 注：用户配置中常见的别名（kling-v3 / kling-O1 / kling-2.6 / seedream4.0 等）
	// 通过 lingjingModelAliasToCanonical → lookupLingjingModel 解析到 canonical 条目，
	// 这里不重复登记，避免双源维护漂移。
}

// lingjingModelAliasToCanonical 把 JoySpace 内部表 / 用户配置中常见的别名指回
// docs 官方 model_name。买价表和路由表分别由 canonical 名维护，避免重复定义。
var lingjingModelAliasToCanonical = map[string]string{
	"kling-v3":                "Kling-V3",
	"kling-O1":                "kling-video-o1",
	"kling-2.6":               "kling-v2-6",
	"seedream4.0":             "doubao-seedream-4-0-250828",
	"seedream4.5":             "doubao-seedream-4-5-251128",
	"doubao-seedance-1-5-pro": "Doubao-Seedance-1.5-pro",
}

// lookupLingjingModel 解析模型名到 catalog 条目。
// 顺序：精确匹配 → 别名表 → 大小写不敏感模糊匹配。
// 返回的第一个值是"用于提交时的 canonical model_name"，应填进 params.model[_name] 字段。
func lookupLingjingModel(model string) (string, *lingjingModelCatalogEntry) {
	if e, ok := lingjingModelCatalog[model]; ok {
		return model, &e
	}
	if canonical, ok := lingjingModelAliasToCanonical[model]; ok {
		if e, ok := lingjingModelCatalog[canonical]; ok {
			return canonical, &e
		}
	}
	lower := strings.ToLower(model)
	for k, v := range lingjingModelCatalog {
		if strings.ToLower(k) == lower {
			entry := v
			return k, &entry
		}
	}
	for alias, canonical := range lingjingModelAliasToCanonical {
		if strings.ToLower(alias) == lower {
			if e, ok := lingjingModelCatalog[canonical]; ok {
				return canonical, &e
			}
		}
	}
	return "", nil
}

// lingjingModelToAPIID 根据模型名称返回 apiId（保留旧 API，主要给 ForwardSeedreamImage 用）。
// 仅对生图模型有效；视频路由请走 lingjingVideoAPIInfo 或 catalog。
func lingjingModelToAPIID(model string) string {
	if _, entry := lookupLingjingModel(model); entry != nil && entry.PictureAPIID != "" {
		return entry.PictureAPIID
	}
	// 兜底：未识别模型默认 Seedream 4.5（与原行为一致，避免旧请求失败）
	return LingjingAPIIDSeedream45
}

// lingjingVideoAPIInfo 返回视频任务的 apiId 和模型名称。
// 保留原签名，仅服务于 Seedance 文生/图生视频路径（plugin/gateway_service.go 调用）。
func lingjingVideoAPIInfo(taskType string) (apiID, modelName string) {
	modelName = "Doubao-Seedance-1.5-pro"
	if taskType == LingjingTaskTypeImage2Video {
		return LingjingAPIIDImageToVideo, modelName
	}
	return LingjingAPIIDTextToVideo, modelName
}

// isLingjingVideoModel 判断给定模型是否为灵境视频生成模型。
// 视频模型的 prompt 对内容质量敏感（"hi" 之类的无效 prompt 会被京东云排队挂起 5min+ 不返回失败），
// 账号测试时需要强制使用合理的中文 prompt 而不是透传前端默认值。
func isLingjingVideoModel(model string) bool {
	_, entry := lookupLingjingModel(model)
	if entry == nil {
		// 未识别模型保守按视频处理：宁可误用默认 prompt 也比让 "hi" 卡住 5min 强
		return true
	}
	return !entry.IsImageGen
}

// buildLingjingTestSubmitRequest 根据模型名称返回账号测试所需的 apiId 和请求参数。
// 参数模板按官方文档 curl 示例的最小必填集合构造，确保连通性测试不被参数校验拒绝。
// 未识别模型回退到 Seedream 生图模板（旧行为）。
func buildLingjingTestSubmitRequest(model, prompt string) (apiID string, params map[string]any) {
	canonical, entry := lookupLingjingModel(model)
	if entry == nil {
		// 未识别：仍按生图兜底（保留原行为，避免破坏管理员手工配置的新模型）
		return lingjingModelToAPIID(model), map[string]any{
			"prompt":  prompt,
			"model":   model,
			"size":    "1024x1024",
			"taskNum": 1,
		}
	}
	modelKey := entry.ModelKey
	if modelKey == "" {
		modelKey = "model_name"
	}

	switch canonical {
	// === 可灵 文生视频 ===
	case "kling-v2-5-turbo":
		return entry.SupportedTestAPIID, map[string]any{
			"prompt": prompt, modelKey: canonical,
			"duration": "5", "mode": "pro", "aspect_ratio": "16:9",
		}
	case "kling-video-o1":
		return entry.SupportedTestAPIID, map[string]any{
			"prompt": prompt, modelKey: canonical,
			"duration": "5", "mode": "pro", "aspect_ratio": "16:9",
		}
	case "kling-v2-6":
		return entry.SupportedTestAPIID, map[string]any{
			"prompt": prompt, modelKey: canonical,
			"duration": "5", "mode": "pro", "aspect_ratio": "16:9",
		}
	case "Kling-V3":
		return entry.SupportedTestAPIID, map[string]any{
			"prompt": prompt, modelKey: canonical,
			"duration": "5", "mode": "pro", "aspect_ratio": "16:9",
			"shot_type": "intelligence",
		}
	case "kling-v1-6":
		// V1.6 仅参考生视频，不带 image_references 上游会拒。账号测试不适用，但仍尝试。
		return entry.SupportedTestAPIID, map[string]any{
			"prompt": prompt, modelKey: canonical,
			"duration": "5", "mode": "pro", "aspect_ratio": "16:9",
			"image_references": []map[string]any{},
		}

	// === 海螺 ===
	case "MiniMax-Hailuo-02", "MiniMax-Hailuo-2.3":
		return entry.SupportedTestAPIID, map[string]any{
			"prompt": prompt, modelKey: canonical,
			"duration": 6, "resolution": "768P",
		}

	// === Vidu 文生视频 ===
	case "viduq1":
		return entry.SupportedTestAPIID, map[string]any{
			"prompt": prompt, modelKey: canonical,
			"duration": 5, "resolution": "1080p", "aspect_ratio": "16:9",
		}
	case "viduq2":
		return entry.SupportedTestAPIID, map[string]any{
			"prompt": prompt, modelKey: canonical,
			"duration": 5, "resolution": "720p", "aspect_ratio": "16:9",
		}
	case "viduq3-pro":
		return entry.SupportedTestAPIID, map[string]any{
			"prompt": prompt, modelKey: canonical,
			"duration": 5, "resolution": "720p", "aspect_ratio": "16:9",
		}

	// === Paiwo / Pixverse 文生视频 ===
	case "v5", "v5.5":
		return entry.SupportedTestAPIID, map[string]any{
			"prompt": prompt, modelKey: canonical,
			"duration": 5, "quality": "720p", "aspect_ratio": "16:9",
		}
	case "v6":
		return entry.SupportedTestAPIID, map[string]any{
			"prompt": prompt, modelKey: canonical,
			"duration": 5, "quality": "720p", "aspect_ratio": "16:9",
		}

	// === 豆包 Seedance 文生视频 ===
	case "Doubao-Seedance-1.5-pro":
		return entry.SupportedTestAPIID, map[string]any{
			"prompt": prompt, modelKey: canonical,
			"duration": "5", "mode": "720p", "aspect_ratio": "16:9",
			"generate_audio": false,
		}

	// === Happy Horse ===
	case "HappyHorse-1.0":
		return entry.SupportedTestAPIID, map[string]any{
			"prompt": prompt, modelKey: canonical,
			"duration": "5", "resolution": "720P", "aspect_ratio": "16:9",
			"watermark": false,
		}

	// === 遗留：cinema-generate-2.0 参考生视频 ===
	case "cinema-generate-2.0":
		return entry.SupportedTestAPIID, map[string]any{
			"multi_model_url": []map[string]any{},
			"prompt":          prompt,
			modelKey:          canonical,
			"duration":        "5",
			"mode":            "720p",
			"aspect_ratio":    "16:9",
			"generate_audio":  false,
		}

	// === 图片生成类 ===
	case "doubao-seedream-4-0-250828", "doubao-seedream-4-5-251128", "Doubao-Seedream-5.0-lite":
		return entry.SupportedTestAPIID, map[string]any{
			"prompt": prompt, modelKey: canonical,
			"size": "2048x2048", "taskNum": 1,
		}
	case "kling-v2-1":
		// V2.1 优先文生视频测试（apiId 550 需 image），改走图片生成 553 更稳
		if entry.PictureAPIID != "" {
			return entry.PictureAPIID, map[string]any{
				"prompt": prompt, modelKey: canonical,
				"taskNum": 1, "aspect_ratio": "16:9",
			}
		}
	case "kling-v2":
		return entry.SupportedTestAPIID, map[string]any{
			"prompt": prompt, modelKey: canonical,
			"taskNum": 1, "aspect_ratio": "16:9",
		}
	case "image-01":
		return entry.SupportedTestAPIID, map[string]any{
			"prompt": prompt, modelKey: canonical,
			"aspect_ratio":      "16:9",
			"subject_reference": []string{},
		}
	}

	// 兜底：构造一个最小生图请求，避免空响应
	return entry.SupportedTestAPIID, map[string]any{
		"prompt": prompt, modelKey: canonical,
		"size": "1024x1024", "taskNum": 1,
	}
}
