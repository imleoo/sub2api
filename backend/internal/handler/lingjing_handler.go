package handler

import (
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/service"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"

	"github.com/gin-gonic/gin"
)

// LingjingHandler 处理灵境视频异步任务提交和查询。
type LingjingHandler struct {
	lingjingSvc         *service.LingjingGatewayService
	billingCacheService *service.BillingCacheService
	apiKeyService       *service.APIKeyService
}

// NewLingjingHandler 构造 LingjingHandler。
func NewLingjingHandler(
	lingjingSvc *service.LingjingGatewayService,
	billingCacheService *service.BillingCacheService,
	apiKeyService *service.APIKeyService,
) *LingjingHandler {
	return &LingjingHandler{
		lingjingSvc:         lingjingSvc,
		billingCacheService: billingCacheService,
		apiKeyService:       apiKeyService,
	}
}

func (h *LingjingHandler) errorResponse(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
}

// SubmitVideoTask POST /lingjing/v1/video/submit
func (h *LingjingHandler) SubmitVideoTask(c *gin.Context) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "invalid api key")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "user context not found")
		return
	}

	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription); err != nil {
		h.errorResponse(c, http.StatusPaymentRequired, err.Error())
		return
	}

	var req service.LingjingVideoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Prompt == "" {
		h.errorResponse(c, http.StatusBadRequest, "prompt is required")
		return
	}
	if req.TaskType != service.LingjingTaskTypeText2Video && req.TaskType != service.LingjingTaskTypeImage2Video {
		h.errorResponse(c, http.StatusBadRequest, "task_type must be text2video or image2video")
		return
	}

	account, err := h.lingjingSvc.SelectAccount(c.Request.Context(), apiKey.GroupID, nil)
	if err != nil {
		h.errorResponse(c, http.StatusServiceUnavailable, "no available lingjing account")
		return
	}

	task, err := h.lingjingSvc.SubmitVideoTask(
		c.Request.Context(),
		account,
		subject.UserID,
		apiKey.ID,
		apiKey.GroupID,
		&req,
	)
	if err != nil {
		h.errorResponse(c, http.StatusInternalServerError, "submit failed: "+err.Error())
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"task_id":     task.ID,
		"gen_task_id": task.GenTaskID,
		"status":      task.Status,
	})
}

// GetVideoTaskStatus GET /lingjing/v1/video/:taskId
func (h *LingjingHandler) GetVideoTaskStatus(c *gin.Context) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "invalid api key")
		return
	}

	taskIDStr := c.Param("taskId")
	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid task id")
		return
	}

	task, err := h.lingjingSvc.GetTaskStatus(c.Request.Context(), taskID, apiKey.ID)
	if err != nil {
		h.errorResponse(c, http.StatusNotFound, "task not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"task_id":     task.ID,
		"gen_task_id": task.GenTaskID,
		"status":      task.Status,
		"result_url":  task.ResultURL,
		"error":       task.ErrorMessage,
		"model":       task.Model,
		"duration":    task.Duration,
		"mode":        task.Mode,
		"created_at":  task.CreatedAt,
	})
}
