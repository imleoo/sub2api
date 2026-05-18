package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

const (
	lingjingBaseURL      = "https://model.jdcloud.com"
	lingjingSubmitPath   = "/joycreator/openApi/submitTask"
	lingjingQueryPath    = "/joycreator/openApi/queryTasKResult" // 保留官方 typo
	lingjingHTTPTimeout  = 30 * time.Second

	// apiId 常量
	LingjingAPIIDSeedream40   = "700" // doubao-seedream-4-0-250828
	LingjingAPIIDSeedream45   = "701" // doubao-seedream-4-5-251128
	LingjingAPIIDSeedream5Lite = "707" // Doubao-Seedream-5.0-lite
	LingjingAPIIDTextToVideo  = "750" // Doubao-Seedance-1.5-pro 文生视频
	LingjingAPIIDImageToVideo = "751" // Doubao-Seedance-1.5-pro 图生视频

	// taskStatus 来自京东云文档
	lingjingTaskStatusJDSuccess = 4
	lingjingTaskStatusJDFailed  = 2
)

// LingjingSubmitRequest 提交任务请求体。
type LingjingSubmitRequest struct {
	APIID  string         `json:"apiId"`
	Params map[string]any `json:"params"`
}

// LingjingSubmitResponse 提交任务响应。
type LingjingSubmitResponse struct {
	RequestID string `json:"requestId"`
	Error     string `json:"error,omitempty"`
	Result    struct {
		GenTaskID      string `json:"genTaskId"`
		Success        bool   `json:"success"`
		Error          string `json:"error,omitempty"`
		ErrorParamName string `json:"errorParamName,omitempty"`
	} `json:"result"`
}

// LingjingQueryRequest 查询任务结果请求体。
type LingjingQueryRequest struct {
	GenTaskID string `json:"genTaskId"`
}

// LingjingSubTaskResult 子任务结果。
type LingjingSubTaskResult struct {
	SubStatus   int    `json:"subStatus"`
	URL         string `json:"url"`          // 无水印地址
	WatermarkURL string `json:"watermarkUrl"` // 有水印地址
	ErrorReason string `json:"errorReason,omitempty"`
}

// LingjingQueryResponse 查询任务结果响应。
type LingjingQueryResponse struct {
	RequestID string `json:"requestId"`
	Error     string `json:"error,omitempty"`
	Result    struct {
		Result struct {
			ID          int64                   `json:"id"`
			TaskStatus  int                     `json:"taskStatus"`
			TaskResults []LingjingSubTaskResult `json:"taskResults,omitempty"`
			ResultNum   int                     `json:"resultNum"`
			TargetNum   int                     `json:"targetNum"`
			Error       string                  `json:"error,omitempty"`
			Success     bool                    `json:"success"`
		} `json:"result"`
	} `json:"result"`
}

// IsSuccess 任务是否整体成功。
func (r *LingjingQueryResponse) IsSuccess() bool {
	return r.Result.Result.TaskStatus == lingjingTaskStatusJDSuccess
}

// IsFailed 任务是否整体失败。
func (r *LingjingQueryResponse) IsFailed() bool {
	return r.Result.Result.TaskStatus == lingjingTaskStatusJDFailed
}

// SuccessURLs 返回所有成功子任务的无水印 URL。
func (r *LingjingQueryResponse) SuccessURLs() []string {
	var urls []string
	for _, t := range r.Result.Result.TaskResults {
		if t.SubStatus == lingjingTaskStatusJDSuccess && t.URL != "" {
			urls = append(urls, t.URL)
		}
	}
	return urls
}

// LingjingClient 京东云灵境 HTTP 客户端。
type LingjingClient struct {
	httpClient *http.Client
}

// NewLingjingClient 创建新的 LingjingClient。
func NewLingjingClient() *LingjingClient {
	return &LingjingClient{
		httpClient: &http.Client{
			Timeout: lingjingHTTPTimeout,
		},
	}
}

// SubmitTask 提交生成任务，返回 genTaskId。
func (c *LingjingClient) SubmitTask(ctx context.Context, apiKey string, req LingjingSubmitRequest) (*LingjingSubmitResponse, error) {
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("lingjing: marshal submit request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, lingjingBaseURL+lingjingSubmitPath, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("lingjing: build submit request: %w", err)
	}
	c.injectHeaders(httpReq, apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("lingjing: submit task: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("lingjing: read submit response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lingjing: submit task HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	var result LingjingSubmitResponse
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, fmt.Errorf("lingjing: decode submit response: %w", err)
	}
	if result.Error != "" {
		return nil, fmt.Errorf("lingjing: submit task error: %s", result.Error)
	}
	if !result.Result.Success {
		return nil, fmt.Errorf("lingjing: submit task failed: %s (param: %s)", result.Result.Error, result.Result.ErrorParamName)
	}
	return &result, nil
}

// QueryTaskResult 查询任务状态和结果。
func (c *LingjingClient) QueryTaskResult(ctx context.Context, apiKey string, genTaskID string) (*LingjingQueryResponse, error) {
	reqBody := LingjingQueryRequest{GenTaskID: genTaskID}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("lingjing: marshal query request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, lingjingBaseURL+lingjingQueryPath, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("lingjing: build query request: %w", err)
	}
	c.injectHeaders(httpReq, apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("lingjing: query task result: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("lingjing: read query response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lingjing: query task HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	var result LingjingQueryResponse
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, fmt.Errorf("lingjing: decode query response: %w", err)
	}
	if result.Error != "" {
		return nil, fmt.Errorf("lingjing: query task error: %s", result.Error)
	}
	return &result, nil
}

func (c *LingjingClient) injectHeaders(req *http.Request, apiKey string) {
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-jdcloud-request-id", uuid.New().String())
}

// PollUntilDone 同步轮询直到任务完成或超时。
// interval：轮询间隔；maxWait：最大等待时间。
func (c *LingjingClient) PollUntilDone(ctx context.Context, apiKey, genTaskID string, interval, maxWait time.Duration) (*LingjingQueryResponse, error) {
	deadline := time.Now().Add(maxWait)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case t := <-ticker.C:
			if t.After(deadline) {
				return nil, fmt.Errorf("lingjing: poll timeout after %v for task %s", maxWait, genTaskID)
			}
			result, err := c.QueryTaskResult(ctx, apiKey, genTaskID)
			if err != nil {
				return nil, err
			}
			if result.IsSuccess() || result.IsFailed() {
				return result, nil
			}
			// 任务仍在处理中，继续等待
		}
	}
}
