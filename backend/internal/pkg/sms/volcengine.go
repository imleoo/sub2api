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
	"sort"
	"strings"
	"time"
)

const (
	volcengineEndpoint = "https://sms.volcengineapi.com"
	volcengineService  = "sms"
	volcengineRegion   = "cn-north-1"
	volcengineAction   = "SendSms"
	volcengineVersion  = "2020-01-01"
)

// VolcengineConfig 火山引擎 SMS 配置
type VolcengineConfig struct {
	AccessKeyID     string
	AccessKeySecret string
	AccountID       string
	Sign            string
	TemplateID      string
}

// VolcengineClient 火山引擎 SMS HTTP 客户端（HMAC-SHA256 签名，兼容 AWS V4）
type VolcengineClient struct {
	cfg        VolcengineConfig
	httpClient *http.Client
}

// NewVolcengineClient 创建火山引擎 SMS 客户端
func NewVolcengineClient(cfg VolcengineConfig) *VolcengineClient {
	return &VolcengineClient{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendCode 发送短信验证码
// phone 格式：+8613812345678
func (c *VolcengineClient) SendCode(ctx context.Context, phone, code string) error {
	if c.cfg.AccessKeyID == "" || c.cfg.AccessKeySecret == "" {
		return fmt.Errorf("volcengine SMS not configured")
	}

	payload := map[string]interface{}{
		"SmsAccountId": c.cfg.AccountID,
		"Sign":         c.cfg.Sign,
		"TemplateId":   c.cfg.TemplateID,
		"TemplateParam": map[string]string{
			"code": code,
		},
		"PhoneNumbers": []string{phone},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal sms payload: %w", err)
	}

	now := time.Now().UTC()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, volcengineEndpoint+"/", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Date", now.Format("20060102T150405Z"))
	req.Header.Set("Host", "sms.volcengineapi.com")

	q := req.URL.Query()
	q.Set("Action", volcengineAction)
	q.Set("Version", volcengineVersion)
	req.URL.RawQuery = q.Encode()

	if err := c.signRequest(req, body, now); err != nil {
		return fmt.Errorf("sign request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send sms request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("volcengine SMS API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ResponseMetadata struct {
			Error *struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error"`
		} `json:"ResponseMetadata"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("parse sms response: %w", err)
	}
	if result.ResponseMetadata.Error != nil {
		return fmt.Errorf("volcengine SMS error %s: %s",
			result.ResponseMetadata.Error.Code,
			result.ResponseMetadata.Error.Message)
	}

	return nil
}

// signRequest 使用 HMAC-SHA256 对请求签名（兼容火山引擎 OpenAPI V4 签名）
func (c *VolcengineClient) signRequest(req *http.Request, body []byte, now time.Time) error {
	dateStr := now.Format("20060102")
	dateTimeStr := now.Format("20060102T150405Z")

	// 1. 规范化 headers
	canonicalHeaders := fmt.Sprintf("content-type:%s\nhost:%s\nx-date:%s\n",
		req.Header.Get("Content-Type"),
		req.Header.Get("Host"),
		dateTimeStr,
	)
	signedHeaders := "content-type;host;x-date"

	// 2. 规范化 query string
	params := make(map[string]string)
	for k, v := range req.URL.Query() {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}
	canonicalQueryString := strings.Join(parts, "&")

	// 3. 计算 body hash
	bodyHash := sha256Hex(body)

	// 4. 规范化请求
	canonicalRequest := strings.Join([]string{
		req.Method,
		"/",
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		bodyHash,
	}, "\n")

	// 5. 待签字符串
	credentialScope := strings.Join([]string{dateStr, volcengineRegion, volcengineService, "request"}, "/")
	stringToSign := strings.Join([]string{
		"HMAC-SHA256",
		dateTimeStr,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	// 6. 派生签名密钥
	signingKey := hmacSHA256(
		hmacSHA256(
			hmacSHA256(
				hmacSHA256([]byte(""+c.cfg.AccessKeySecret), []byte(dateStr)),
				[]byte(volcengineRegion),
			),
			[]byte(volcengineService),
		),
		[]byte("request"),
	)

	// 7. 计算签名
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	// 8. Authorization header
	req.Header.Set("Authorization", fmt.Sprintf(
		"HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		c.cfg.AccessKeyID, credentialScope, signedHeaders, signature,
	))

	return nil
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}
