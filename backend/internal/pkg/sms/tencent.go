package sms

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	tencentEndpoint = "https://sms.tencentcloudapi.com"
	tencentService  = "sms"
	tencentVersion  = "2021-01-11"
	tencentAction   = "SendSms"
	tencentRegion   = "ap-guangzhou"
)

// TencentConfig 腾讯云 SMS 配置
type TencentConfig struct {
	SecretID    string
	SecretKey   string
	SmsSdkAppID string
	Sign        string
	TemplateID  string
}

// TencentClient 腾讯云 SMS HTTP 客户端（TC3-HMAC-SHA256 签名）
type TencentClient struct {
	cfg        TencentConfig
	httpClient *http.Client
}

// NewTencentClient 创建腾讯云 SMS 客户端
func NewTencentClient(cfg TencentConfig) *TencentClient {
	return &TencentClient{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// SendCode 发送短信验证码
// phone 格式：+8613812345678
func (c *TencentClient) SendCode(ctx context.Context, phone, code string) error {
	if c.cfg.SecretID == "" || c.cfg.SecretKey == "" {
		return fmt.Errorf("tencent SMS not configured")
	}

	payload := map[string]any{
		"PhoneNumberSet":   []string{phone},
		"SmsSdkAppId":      c.cfg.SmsSdkAppID,
		"SignName":         c.cfg.Sign,
		"TemplateId":       c.cfg.TemplateID,
		"TemplateParamSet": []string{code},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal tencent sms payload: %w", err)
	}

	now := time.Now().UTC()
	timestamp := strconv.FormatInt(now.Unix(), 10)
	dateStr := now.Format("2006-01-02")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tencentEndpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Host", "sms.tencentcloudapi.com")
	req.Header.Set("X-TC-Timestamp", timestamp)
	req.Header.Set("X-TC-Action", tencentAction)
	req.Header.Set("X-TC-Version", tencentVersion)
	req.Header.Set("X-TC-Region", tencentRegion)

	authorization := c.buildAuthorization(body, dateStr, timestamp)
	req.Header.Set("Authorization", authorization)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send tencent sms request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tencent SMS API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Response struct {
			Error *struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error"`
			SendStatusSet []struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"SendStatusSet"`
		} `json:"Response"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("parse tencent sms response: %w", err)
	}
	if result.Response.Error != nil {
		return fmt.Errorf("tencent SMS error %s: %s",
			result.Response.Error.Code, result.Response.Error.Message)
	}
	if len(result.Response.SendStatusSet) > 0 && result.Response.SendStatusSet[0].Code != "Ok" {
		return fmt.Errorf("tencent SMS send failed: %s", result.Response.SendStatusSet[0].Message)
	}
	return nil
}

// buildAuthorization 构建腾讯云 TC3-HMAC-SHA256 签名头
func (c *TencentClient) buildAuthorization(body []byte, dateStr, timestamp string) string {
	contentType := "application/json"
	host := "sms.tencentcloudapi.com"
	signedHeaders := "content-type;host"

	hashedPayload := sha256Hex(body)
	canonicalRequest := strings.Join([]string{
		"POST",
		"/",
		"",
		"content-type:" + contentType + "\n" + "host:" + host + "\n",
		signedHeaders,
		hashedPayload,
	}, "\n")

	credentialScope := dateStr + "/" + tencentService + "/tc3_request"
	stringToSign := strings.Join([]string{
		"TC3-HMAC-SHA256",
		timestamp,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	secretDate := tc3HmacSHA256([]byte("TC3"+c.cfg.SecretKey), []byte(dateStr))
	secretService := tc3HmacSHA256(secretDate, []byte(tencentService))
	secretSigning := tc3HmacSHA256(secretService, []byte("tc3_request"))
	signature := hex.EncodeToString(tc3HmacSHA256(secretSigning, []byte(stringToSign)))

	return fmt.Sprintf(
		"TC3-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		c.cfg.SecretID, credentialScope, signedHeaders, signature,
	)
}

func tc3HmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}
