// Package wx 封装微信服务端接口（订阅消息）。
//
// 只依赖标准库：一期不用 Redis，access_token 缓存在进程内（Q24，单实例）。
// 所有错误信息都不包含 app secret 与 access_token，避免日志泄露。
package wx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// DefaultBaseURL 是微信开放接口的默认地址；单测通过 NewClientWithBaseURL 注入 httptest 地址。
const DefaultBaseURL = "https://api.weixin.qq.com"

// tokenRefreshMargin 提前刷新 access_token，避免边界过期。
const tokenRefreshMargin = 5 * time.Minute

// APIError 是微信接口返回的业务错误（HTTP 200 + errcode）。
type APIError struct {
	Op   string
	Code int
	Msg  string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("微信接口 %s 失败: errcode=%d errmsg=%s", e.Op, e.Code, e.Msg)
}

// IsPermanent 判断该错误重试也不会成功（openid/模板/page/参数问题或用户已无订阅额度）。
// 43101 表示用户拒绝接收或一次性订阅额度已用完。
func IsPermanent(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.Code {
	case 40003, 40037, 41030, 43101, 47003:
		return true
	default:
		return false
	}
}

// redactedError 保留底层错误类型（便于 errors.As 区分网络错误与 API 错误），
// 但对外只暴露脱敏后的文本，避免 secret/access_token 进入日志。
type redactedError struct {
	msg   string
	cause error
}

func (e *redactedError) Error() string { return e.msg }

func (e *redactedError) Unwrap() error { return e.cause }

// redact 把敏感串（app secret、access_token）从错误文本里抹掉。
//
// net/http 的网络错误会把完整请求 URL 拼进 err.Error()，
// 而微信接口的 secret 与 access_token 只能放在 query 里，所以必须在返回前替换。
func redact(text string, secrets ...string) string {
	for _, s := range secrets {
		if s == "" {
			continue
		}
		text = strings.ReplaceAll(text, s, "***")
	}
	return text
}

// Client 是微信服务端接口客户端。
type Client struct {
	appID      string
	appSecret  string
	baseURL    string
	httpClient *http.Client
	now        func() time.Time

	mu          sync.Mutex
	token       string
	tokenExpiry time.Time
}

// NewClient 使用默认地址与 10s 超时创建客户端；appID/appSecret 为空时发送会直接返回错误。
func NewClient(appID, appSecret string) *Client {
	return NewClientWithBaseURL(appID, appSecret, DefaultBaseURL, &http.Client{Timeout: 10 * time.Second})
}

// NewClientWithBaseURL 供单测注入 httptest 地址；httpClient 为 nil 时使用 10s 超时。
func NewClientWithBaseURL(appID, appSecret, baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{
		appID:      appID,
		appSecret:  appSecret,
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
		now:        time.Now,
	}
}

// AccessToken 返回缓存的 access_token，必要时刷新。
// 注意：不要在日志里打印返回值。
func (c *Client) AccessToken(ctx context.Context) (string, error) {
	if c.appID == "" || c.appSecret == "" {
		return "", errors.New("微信 AppID/AppSecret 未配置")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && c.now().Before(c.tokenExpiry) {
		return c.token, nil
	}
	token, ttl, err := c.fetchToken(ctx)
	if err != nil {
		return "", err
	}
	c.token = token
	c.tokenExpiry = c.now().Add(ttl - tokenRefreshMargin)
	return token, nil
}

// invalidate 在 token 失效时清空缓存，供调用方重试。
func (c *Client) invalidate() {
	c.mu.Lock()
	c.token = ""
	c.tokenExpiry = time.Time{}
	c.mu.Unlock()
}

func (c *Client) fetchToken(ctx context.Context) (string, time.Duration, error) {
	q := url.Values{}
	q.Set("grant_type", "client_credential")
	q.Set("appid", c.appID)
	q.Set("secret", c.appSecret)
	endpoint := c.baseURL + "/cgi-bin/token?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", 0, fmt.Errorf("构造 access_token 请求: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// 网络类错误会把完整 URL（含 secret）带进文本，必须脱敏后再返回，否则会写进日志。
		return "", 0, &redactedError{msg: "请求 access_token: " + redact(err.Error(), c.appSecret), cause: err}
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return "", 0, fmt.Errorf("读取 access_token 响应: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("access_token 接口 HTTP %d", resp.StatusCode)
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", 0, fmt.Errorf("解析 access_token 响应: %w", err)
	}
	if out.ErrCode != 0 || out.AccessToken == "" {
		return "", 0, &APIError{Op: "access_token", Code: out.ErrCode, Msg: out.ErrMsg}
	}
	ttl := time.Duration(out.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = 2 * time.Hour
	}
	return out.AccessToken, ttl, nil
}
