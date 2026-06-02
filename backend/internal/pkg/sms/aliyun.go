package sms

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	aliyunEndpoint = "https://dysmsapi.aliyuncs.com"
	aliyunVersion  = "2017-05-25"
	aliyunAction   = "SendSms"
)

// AliyunConfig 阿里云 SMS 配置
type AliyunConfig struct {
	AccessKeyID     string
	AccessKeySecret string
	Sign            string
	TemplateCode    string
}

// AliyunClient 阿里云 SMS HTTP 客户端（ACS3-HMAC-SHA256 签名）
type AliyunClient struct {
	cfg        AliyunConfig
	httpClient *http.Client
}

// NewAliyunClient 创建阿里云 SMS 客户端
func NewAliyunClient(cfg AliyunConfig) *AliyunClient {
	return &AliyunClient{
		cfg: cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// SendCode 发送短信验证码
// phone 格式：+8613812345678 → 自动去掉 + 前缀，阿里云国内号码不需要 +86
func (c *AliyunClient) SendCode(ctx context.Context, phone, code string) error {
	if c.cfg.AccessKeyID == "" || c.cfg.AccessKeySecret == "" {
		return fmt.Errorf("aliyun SMS not configured")
	}

	// 阿里云国内短信号码格式：去掉 +86 前缀
	phoneLocal := strings.TrimPrefix(phone, "+86")

	templateParam, _ := json.Marshal(map[string]string{"code": code})
	body := url.Values{
		"PhoneNumbers":  {phoneLocal},
		"SignName":      {c.cfg.Sign},
		"TemplateCode":  {c.cfg.TemplateCode},
		"TemplateParam": {string(templateParam)},
	}.Encode()
	bodyBytes := []byte(body)

	now := time.Now().UTC()
	dateStr := now.Format("20060102T150405Z")
	nonce := randomNonce()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, aliyunEndpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Host", "dysmsapi.aliyuncs.com")
	req.Header.Set("x-acs-action", aliyunAction)
	req.Header.Set("x-acs-version", aliyunVersion)
	req.Header.Set("x-acs-date", dateStr)
	req.Header.Set("x-acs-signature-nonce", nonce)

	bodyHash := sha256Hex(bodyBytes)
	req.Header.Set("x-acs-content-sha256", bodyHash)

	authorization := c.buildAuthorization(req, bodyHash)
	req.Header.Set("Authorization", authorization)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send aliyun sms request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("aliyun SMS API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Code      string `json:"Code"`
		Message   string `json:"Message"`
		RequestID string `json:"RequestId"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("parse aliyun sms response: %w", err)
	}
	if result.Code != "OK" {
		return fmt.Errorf("aliyun SMS error %s: %s", result.Code, result.Message)
	}
	return nil
}

// buildAuthorization 构建阿里云 ACS3-HMAC-SHA256 签名头
func (c *AliyunClient) buildAuthorization(req *http.Request, bodyHash string) string {
	// 收集参与签名的 headers（小写，按字母排序）
	signedHeaderNames := []string{
		"content-type",
		"host",
		"x-acs-action",
		"x-acs-content-sha256",
		"x-acs-date",
		"x-acs-signature-nonce",
		"x-acs-version",
	}

	var canonicalHeaderLines strings.Builder
	for _, name := range signedHeaderNames {
		canonicalHeaderLines.WriteString(name)
		canonicalHeaderLines.WriteByte(':')
		canonicalHeaderLines.WriteString(req.Header.Get(name))
		canonicalHeaderLines.WriteByte('\n')
	}
	signedHeadersStr := strings.Join(signedHeaderNames, ";")

	canonicalRequest := strings.Join([]string{
		"POST",
		"/",
		"",
		canonicalHeaderLines.String(),
		signedHeadersStr,
		bodyHash,
	}, "\n")

	hashedCanonical := sha256Hex([]byte(canonicalRequest))
	stringToSign := "ACS3-HMAC-SHA256\n" + hashedCanonical

	sig := hex.EncodeToString(acsHmacSHA256([]byte(c.cfg.AccessKeySecret), []byte(stringToSign)))

	return fmt.Sprintf(
		"ACS3-HMAC-SHA256 Credential=%s,SignedHeaders=%s,Signature=%s",
		c.cfg.AccessKeyID, signedHeadersStr, sig,
	)
}

func acsHmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func randomNonce() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 32)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}
