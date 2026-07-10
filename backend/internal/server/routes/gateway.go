package routes

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// RegisterGatewayRoutes 注册 API 网关路由（Claude/OpenAI/Gemini 兼容）
func RegisterGatewayRoutes(
	r *gin.Engine,
	h *handler.Handlers,
	apiKeyAuth middleware.APIKeyAuthMiddleware,
	apiKeyService *service.APIKeyService,
	subscriptionService *service.SubscriptionService,
	opsService *service.OpsService,
	settingService *service.SettingService,
	cfg *config.Config,
) {
	bodyLimit := middleware.RequestBodyLimit(cfg.Gateway.MaxBodySize)
	clientRequestID := middleware.ClientRequestID()
	opsErrorLogger := handler.OpsErrorLoggerMiddleware(opsService)
	endpointNorm := handler.InboundEndpointMiddleware()

	// 未分组 Key 拦截中间件（按协议格式区分错误响应）
	requireGroupAnthropic := middleware.RequireGroupAssignment(settingService, middleware.AnthropicErrorWriter)
	requireGroupGoogle := middleware.RequireGroupAssignment(settingService, middleware.GoogleErrorWriter)

	// API网关（Claude API兼容）
	imagesHandler := func(c *gin.Context) {
		platform := getGroupPlatform(c)
		// fork 12：lingjing 平台的 Seedream 同步生图复用本路由（协议感知，非 OpenAI 入站也放行）。
		if isOpenAIInbound(c) || platform == service.PlatformLingjing {
			h.OpenAIGateway.Images(c)
			return
		}
		if platform == service.PlatformGrok {
			h.OpenAIGateway.GrokImages(c)
			return
		}
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"type":    "not_found_error",
				"message": "Images API is not supported for this platform",
			},
		})
	}
	videoGenerationHandler := func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformGrok {
			h.OpenAIGateway.GrokVideoGeneration(c)
			return
		}
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"type":    "not_found_error",
				"message": "Videos API is not supported for this platform",
			},
		})
	}
	videoStatusHandler := func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformGrok {
			h.OpenAIGateway.GrokVideoStatus(c)
			return
		}
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"type":    "not_found_error",
				"message": "Videos API is not supported for this platform",
			},
		})
	}
	gateway := r.Group("/v1")
	gateway.Use(bodyLimit)
	gateway.Use(clientRequestID)
	gateway.Use(opsErrorLogger)
	gateway.Use(endpointNorm)
	gateway.Use(gin.HandlerFunc(apiKeyAuth))
	gateway.Use(requireGroupAnthropic)
	{
		// /v1/messages: route by inbound protocol (P2-3 双写期：proto 优先，platform 兜底)
		gateway.POST("/messages", func(c *gin.Context) {
			if isOpenAIInbound(c) {
				h.OpenAIGateway.Messages(c)
				return
			}
			h.Gateway.Messages(c)
		})
		// /v1/messages/count_tokens:
		//   - anthropic 入站 → 直接转发给上游 /v1/messages/count_tokens
		//   - openai 平台分组 → 走 Anthropic-compat 桥（上游官方 /v1/responses/input_tokens）
		//   - 其它 openai 协议入站（grok / generic 等）→ 保持 404（上游无对应端点）
		gateway.POST("/messages/count_tokens", func(c *gin.Context) {
			if getGroupPlatform(c) == service.PlatformOpenAI {
				h.OpenAIGateway.CountTokens(c)
				return
			}
			if isOpenAIInbound(c) {
				service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
				c.JSON(http.StatusNotFound, gin.H{
					"type": "error",
					"error": gin.H{
						"type":    "not_found_error",
						"message": "Token counting is not supported for this platform",
					},
				})
				return
			}
			h.Gateway.CountTokens(c)
		})
		// fork：Codex manifest 分支（伪装 Codex 客户端的 /models 变体）随 codex 逆向链移除，
		// 统一返回标准模型列表（功能 35）。
		gateway.GET("/models", h.Gateway.Models)
		gateway.GET("/usage", h.Gateway.Usage)
		// OpenAI Responses API: route by inbound protocol (P2-3)
		gateway.POST("/responses", func(c *gin.Context) {
			if isOpenAIInbound(c) {
				h.OpenAIGateway.Responses(c)
				return
			}
			h.Gateway.Responses(c)
		})
		gateway.POST("/responses/*subpath", func(c *gin.Context) {
			if isOpenAIInbound(c) {
				h.OpenAIGateway.Responses(c)
				return
			}
			h.Gateway.Responses(c)
		})
		gateway.GET("/responses", h.OpenAIGateway.ResponsesWebSocket)
		// OpenAI Chat Completions API: route by inbound protocol (P2-3)
		gateway.POST("/chat/completions", func(c *gin.Context) {
			if isOpenAIInbound(c) {
				h.OpenAIGateway.ChatCompletions(c)
				return
			}
			h.Gateway.ChatCompletions(c)
		})
		gateway.POST("/embeddings", func(c *gin.Context) {
			if getGroupPlatform(c) != service.PlatformOpenAI {
				service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
				c.JSON(http.StatusNotFound, gin.H{
					"error": gin.H{
						"type":    "not_found_error",
						"message": "Embeddings API is not supported for this platform",
					},
				})
				return
			}
			h.OpenAIGateway.Embeddings(c)
		})
		gateway.POST("/images/generations", imagesHandler)
		gateway.POST("/images/edits", imagesHandler)
		gateway.POST("/images/batches", h.BatchImage.Submit)
		gateway.GET("/images/batches", h.BatchImage.List)
		gateway.GET("/images/batches/models", h.BatchImage.Models)
		gateway.GET("/images/batches/:id", h.BatchImage.Get)
		gateway.GET("/images/batches/:id/items", h.BatchImage.Items)
		gateway.GET("/images/batches/:id/items/:custom_id/content", h.BatchImage.ItemContent)
		gateway.GET("/images/batches/:id/download", h.BatchImage.Download)
		gateway.POST("/images/batches/:id/cancel", h.BatchImage.Cancel)
		gateway.DELETE("/images/batches/:id", h.BatchImage.DeleteRecord)
		gateway.DELETE("/images/batches/:id/outputs", h.BatchImage.DeleteOutputs)
		gateway.POST("/videos/generations", videoGenerationHandler)
		gateway.GET("/videos/:request_id", videoStatusHandler)
	}

	// Gemini 原生 API 兼容层（Gemini SDK/CLI 直连）
	gemini := r.Group("/v1beta")
	gemini.Use(bodyLimit)
	gemini.Use(clientRequestID)
	gemini.Use(opsErrorLogger)
	gemini.Use(endpointNorm)
	gemini.Use(middleware.APIKeyAuthWithSubscriptionGoogle(apiKeyService, subscriptionService, cfg))
	gemini.Use(requireGroupGoogle)
	{
		gemini.GET("/models", h.Gateway.GeminiV1BetaListModels)
		gemini.GET("/models/:model", h.Gateway.GeminiV1BetaGetModel)
		// Gin treats ":" as a param marker, but Gemini uses "{model}:{action}" in the same segment.
		gemini.POST("/models/*modelAction", h.Gateway.GeminiV1BetaModels)
	}

	// OpenAI Responses API（不带v1前缀的别名）— P2-3 协议感知分流
	responsesHandler := func(c *gin.Context) {
		if isOpenAIInbound(c) {
			h.OpenAIGateway.Responses(c)
			return
		}
		h.Gateway.Responses(c)
	}
	r.POST("/responses", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, responsesHandler)
	r.POST("/responses/*subpath", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, responsesHandler)
	r.GET("/responses", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, h.OpenAIGateway.ResponsesWebSocket)
	codexDirect := r.Group("/backend-api/codex")
	codexDirect.Use(bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic)
	{
		codexDirect.POST("/responses", responsesHandler)
		codexDirect.POST("/responses/*subpath", responsesHandler)
		codexDirect.GET("/responses", h.OpenAIGateway.ResponsesWebSocket)
	}
	// OpenAI Chat Completions API（不带v1前缀的别名）— P2-3 协议感知分流
	r.POST("/chat/completions", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, func(c *gin.Context) {
		if isOpenAIInbound(c) {
			h.OpenAIGateway.ChatCompletions(c)
			return
		}
		h.Gateway.ChatCompletions(c)
	})
	r.POST("/embeddings", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, func(c *gin.Context) {
		if getGroupPlatform(c) != service.PlatformOpenAI {
			service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"type":    "not_found_error",
					"message": "Embeddings API is not supported for this platform",
				},
			})
			return
		}
		h.OpenAIGateway.Embeddings(c)
	})
	r.POST("/images/generations", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, func(c *gin.Context) {
		// fork 12 lingjing 警戒：lingjing 平台的 Seedream 同步生图复用此路由
		// 协议感知判断：OpenAI 入站走 OpenAI handler；非 OpenAI 但 platform=lingjing 也允许
		platform := getGroupPlatform(c)
		if !isOpenAIInbound(c) && platform != service.PlatformLingjing {
			service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"type":    "not_found_error",
					"message": "Images API is not supported for this platform",
				},
			})
			return
		}
		h.OpenAIGateway.Images(c)
	})
	r.POST("/images/edits", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, gin.HandlerFunc(apiKeyAuth), requireGroupAnthropic, func(c *gin.Context) {
		if !isOpenAIInbound(c) {
			service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"type":    "not_found_error",
					"message": "Images API is not supported for this platform",
				},
			})
			return
		}
		h.OpenAIGateway.Images(c)
	})

	// 灵境（京东云）视频异步任务路由
	lingjingV1 := r.Group("/lingjing/v1")
	lingjingV1.Use(bodyLimit)
	lingjingV1.Use(clientRequestID)
	lingjingV1.Use(opsErrorLogger)
	lingjingV1.Use(endpointNorm)
	lingjingV1.Use(middleware.ForcePlatform(service.PlatformLingjing))
	lingjingV1.Use(gin.HandlerFunc(apiKeyAuth))
	lingjingV1.Use(requireGroupAnthropic)
	{
		lingjingV1.POST("/video/submit", h.Lingjing.SubmitVideoTask)
		lingjingV1.GET("/video/:taskId", h.Lingjing.GetVideoTaskStatus)
	}

}

// getGroupPlatform extracts the group platform from the API Key stored in context.
func getGroupPlatform(c *gin.Context) string {
	apiKey, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || apiKey.Group == nil {
		return ""
	}
	return apiKey.Group.Platform
}

// getGroupInboundProtocol returns the inbound protocol of the requesting group (Phase 2 P2-3).
//
// Resolution order (docs/relay-architecture-design.md §3.2):
//  1. Group.InboundProtocol if present and valid (per docs/glossary.md §1.1)
//  2. fall back to platform-derived default (compatibility for legacy groups)
//  3. empty string when no API key / no group context — caller should use legacy platform branch
func getGroupInboundProtocol(c *gin.Context) string {
	apiKey, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || apiKey.Group == nil {
		return ""
	}
	return domain.ResolveInboundProtocol(apiKey.Group.InboundProtocol, apiKey.Group.Platform)
}

// isOpenAIInbound returns true when the requesting group's inbound protocol is OpenAI-shaped.
//
// Replaces all `getGroupPlatform(c) == service.PlatformOpenAI` branches with a protocol-aware
// check that still falls back to the legacy platform field. Both openai_chat and openai_responses
// inbound shapes route to the OpenAI handler family.
func isOpenAIInbound(c *gin.Context) bool {
	proto := getGroupInboundProtocol(c)
	if proto == domain.ProtocolOpenAIChat || proto == domain.ProtocolOpenAIResponses {
		return true
	}
	// 兜底（双写期）：proto 为空时按 platform 判断，保持旧行为
	if proto == "" {
		return getGroupPlatform(c) == service.PlatformOpenAI
	}
	return false
}
