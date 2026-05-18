package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

// lingjingSizeToTier 将灵境生图尺寸映射到计费档位。
func lingjingSizeToTier(size string) string {
	if size == "3024x1296" {
		return "3K"
	}
	return "2K"
}

// lingjingValidateSize 校验并规范化尺寸参数；无效值回退到默认方形尺寸。
func lingjingValidateSize(size string) string {
	switch size {
	case "2048x2048", "2304x1728", "1728x2304",
		"2560x1440", "1440x2560", "2496x1664",
		"1664x2496", "3024x1296":
		return size
	default:
		return "2048x2048"
	}
}

type lingjingImagesResponseItem struct {
	URL string `json:"url"`
}

type lingjingImagesResponse struct {
	Created int64                        `json:"created"`
	Data    []lingjingImagesResponseItem `json:"data"`
}

// forwardLingjingImages 通过灵境平台执行同步生图，并以 OpenAI 兼容格式写回响应。
func (s *OpenAIGatewayService) forwardLingjingImages(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	parsed *OpenAIImagesRequest,
) (*OpenAIForwardResult, error) {
	if s.lingjingSvc == nil {
		return nil, fmt.Errorf("lingjing service not configured")
	}

	n := parsed.N
	if n <= 0 {
		n = 1
	}
	size := lingjingValidateSize(parsed.Size)

	req := &LingjingImageRequest{
		Prompt:  parsed.Prompt,
		Model:   parsed.Model,
		Size:    size,
		TaskNum: n,
	}
	if len(parsed.InputImageURLs) > 0 {
		req.Image = parsed.InputImageURLs
	}

	result, err := s.lingjingSvc.ForwardSeedreamImage(ctx, account, req)
	if err != nil {
		return nil, fmt.Errorf("lingjing images: %w", err)
	}

	items := make([]lingjingImagesResponseItem, len(result.URLs))
	for i, u := range result.URLs {
		items[i] = lingjingImagesResponseItem{URL: u}
	}
	respBody, err := json.Marshal(lingjingImagesResponse{
		Created: time.Now().Unix(),
		Data:    items,
	})
	if err != nil {
		return nil, fmt.Errorf("lingjing images: marshal response: %w", err)
	}

	c.Data(200, "application/json", respBody)

	return &OpenAIForwardResult{
		Model:      parsed.Model,
		ImageCount: result.TaskCount,
		ImageSize:  lingjingSizeToTier(size),
	}, nil
}
