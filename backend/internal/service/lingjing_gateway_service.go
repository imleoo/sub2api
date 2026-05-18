package service

import (
	"context"
	"fmt"
	"time"
)

const (
	lingjingImagePollInterval = 2 * time.Second
	lingjingImagePollMaxWait  = 60 * time.Second
)

// LingjingVideoRequest 视频生成任务请求参数。
type LingjingVideoRequest struct {
	Prompt        string   `json:"prompt"`
	TaskType      string   `json:"task_type"`       // "text2video" / "image2video"
	Duration      string   `json:"duration"`        // "5" / "10" / "12"
	Mode          string   `json:"mode"`            // "480p" / "720p" / "1080p"
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

	queryResp, err := s.client.PollUntilDone(ctx, apiKey, submitResp.Result.GenTaskID, lingjingImagePollInterval, lingjingImagePollMaxWait)
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
		GenTaskID:   submitResp.Result.GenTaskID,
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

// lingjingModelToAPIID 根据模型名称返回 apiId。
func lingjingModelToAPIID(model string) string {
	switch model {
	case "doubao-seedream-4-0-250828":
		return LingjingAPIIDSeedream40
	case "Doubao-Seedream-5.0-lite":
		return LingjingAPIIDSeedream5Lite
	default:
		// 默认 Seedream 4.5
		return LingjingAPIIDSeedream45
	}
}

// lingjingVideoAPIInfo 返回视频任务的 apiId 和模型名称。
func lingjingVideoAPIInfo(taskType string) (apiID, modelName string) {
	modelName = "Doubao-Seedance-1.5-pro"
	if taskType == LingjingTaskTypeImage2Video {
		return LingjingAPIIDImageToVideo, modelName
	}
	return LingjingAPIIDTextToVideo, modelName
}
