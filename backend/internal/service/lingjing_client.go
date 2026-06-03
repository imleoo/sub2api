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

	// apiId 常量。命名约定：LingjingAPIID<产品><版本><任务类型>
	// 任务类型缩写：TTV=文生视频, PTV=图生视频, RTV=参考生视频, Pic=图片生成
	//
	// 豆包 (Doubao) ---------------------------------------------------------
	LingjingAPIIDSeedream40      = "700" // doubao-seedream-4-0-250828 (图片)
	LingjingAPIIDSeedream45      = "701" // doubao-seedream-4-5-251128 (图片)
	LingjingAPIIDSeedream5Lite   = "707" // Doubao-Seedream-5.0-lite (图片)
	LingjingAPIIDTextToVideo     = "750" // Doubao-Seedance-1.5-pro 文生视频
	LingjingAPIIDImageToVideo    = "751" // Doubao-Seedance-1.5-pro 图生视频
	LingjingAPIIDCinemaGenerate20 = "754" // cinema-generate-2.0 (Seedance 2.0 参考生视频)

	// 可灵 (Kling) ----------------------------------------------------------
	LingjingAPIIDKlingV21PTV     = "550" // kling-v2-1 图生视频
	LingjingAPIIDKlingV25TurboTTV = "551" // kling-v2-5-turbo 文生视频
	LingjingAPIIDKlingV16RTV     = "552" // kling-v1-6 参考生视频
	LingjingAPIIDKlingV21Pic     = "553" // kling-v2-1 图片生成
	LingjingAPIIDKlingV2Pic      = "554" // kling-v2 图片生成
	LingjingAPIIDKlingO1TTV      = "560" // kling-video-o1 文生视频
	LingjingAPIIDKlingO1PTV      = "561" // kling-video-o1 图生视频
	LingjingAPIIDKlingO1RTV      = "562" // kling-video-o1 参考生视频
	LingjingAPIIDKlingV26TTV     = "563" // kling-v2-6 文生视频
	LingjingAPIIDKlingV26PTV     = "564" // kling-v2-6 图生视频
	LingjingAPIIDKlingV3TTV      = "565" // Kling-V3 文生视频
	LingjingAPIIDKlingV3PTV      = "566" // Kling-V3 图生视频
	LingjingAPIIDKlingV3Pic      = "567" // Kling-V3 图片生成

	// 海螺 (MiniMax Hailuo) --------------------------------------------------
	LingjingAPIIDHailuoImage01    = "456" // image-01 图片生成
	LingjingAPIIDHailuo02PTV      = "457" // MiniMax-Hailuo-02 图生视频
	LingjingAPIIDHailuo02TTV      = "458" // MiniMax-Hailuo-02 文生视频
	LingjingAPIIDHailuoS2V01      = "459" // S2V-01 主体参考生视频
	LingjingAPIIDHailuo23TTV      = "460" // MiniMax-Hailuo-2.3 文生视频
	LingjingAPIIDHailuo23PTV      = "461" // MiniMax-Hailuo-2.3 图生视频
	LingjingAPIIDHailuo23FastPTV  = "462" // MiniMax-Hailuo-2.3-Fast 图生视频

	// Vidu -----------------------------------------------------------------
	LingjingAPIIDViduQ1PTV       = "0"   // viduq1 图生视频
	LingjingAPIIDVidu20PTV       = "2"   // vidu2.0 图生视频
	LingjingAPIIDViduQ1RTV       = "4"   // viduq1 参考生视频
	LingjingAPIIDVidu20RTV       = "5"   // vidu2.0 参考生视频
	LingjingAPIIDViduQ1TTV       = "7"   // viduq1 文生视频
	LingjingAPIIDViduQ1Pic       = "16"  // viduq1 图片生成
	LingjingAPIIDViduQ2ProPTV    = "17"  // viduq2-pro 图生视频
	LingjingAPIIDViduQ2ProPTVAlt = "19"  // viduq2-pro 图生视频（单图 API 专版）
	LingjingAPIIDViduQ2TTV       = "20"  // viduq2 文生视频
	LingjingAPIIDViduQ2RTV       = "21"  // viduq2 参考生视频
	LingjingAPIIDViduQ2Pic       = "22"  // viduq2 图片生成
	LingjingAPIIDViduQ3ProTTV    = "23"  // viduq3-pro 文生视频
	LingjingAPIIDViduQ3ProPTV    = "24"  // viduq3-pro 图生视频

	// 拍我 / Pixverse -------------------------------------------------------
	LingjingAPIIDPaiwoV5TTV      = "400" // v5 文生视频
	LingjingAPIIDPaiwoV55TTV     = "401" // v5.5 文生视频
	LingjingAPIIDPaiwoV55PTV     = "402" // v5.5 图生视频
	LingjingAPIIDPaiwoV5PTV      = "501" // v5 图生视频
	LingjingAPIIDPaiwoV5PTVAlt   = "502" // v5 图生视频（API 专版）
	LingjingAPIIDPaiwoV5RTV      = "503" // v5 参考生视频
	LingjingAPIIDPixverseV6PTV   = "504" // v6 图生视频
	LingjingAPIIDPixverseV6TTV   = "505" // v6 文生视频

	// Happy Horse ----------------------------------------------------------
	LingjingAPIIDHappyHorse10TTV = "200202" // HappyHorse-1.0 文生视频
	LingjingAPIIDHappyHorse10PTV = "200203" // HappyHorse-1.0 图生视频
	LingjingAPIIDHappyHorse10RTV = "200204" // HappyHorse-1.0 参考生视频

	// 数字人 (Digital Human) -------------------------------------------------
	LingjingAPIIDOmniHuman = "703" // OmniHuman1.5 数字人（识别 + 视频生成共用）

	// taskStatus 来自京东云文档
	lingjingTaskStatusJDSuccess = 4
	lingjingTaskStatusJDFailed  = 2
)

// LingjingSubmitRequest 提交任务请求体。
type LingjingSubmitRequest struct {
	APIID  string         `json:"apiId"`
	Params map[string]any `json:"params"`
}

// LingjingSubmitError 提交任务响应中的顶层错误对象。
type LingjingSubmitError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// LingjingSubmitResponse 提交任务响应。
// 实际格式：顶层 error 为对象或 null；实际任务字段在 result.result 双层嵌套下。
type LingjingSubmitResponse struct {
	RequestID string               `json:"requestId"`
	Error     *LingjingSubmitError `json:"error"`
	Result    struct {
		Result struct {
			AppID          string `json:"appId"`
			GenTaskID      string `json:"genTaskId"`
			Success        bool   `json:"success"`
			Error          string `json:"error"`
			ErrorParamName string `json:"errorParamName"`
		} `json:"result"`
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
	if result.Error != nil {
		return nil, fmt.Errorf("lingjing: submit task error: %s", result.Error.Message)
	}
	if !result.Result.Result.Success {
		return nil, fmt.Errorf("lingjing: submit task failed: %s (param: %s)", result.Result.Result.Error, result.Result.Result.ErrorParamName)
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
