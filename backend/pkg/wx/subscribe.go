package wx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// SubscribeMessage 是一条一次性订阅消息。字段与微信 subscribeMessage.send 对齐。
type SubscribeMessage struct {
	ToUser           string
	TemplateID       string
	Page             string
	MiniprogramState string
	Lang             string
	// Data 的键是微信模板关键词（thing1/time2…），值直接取自业务内容。
	Data map[string]string
}

type subscribeValue struct {
	Value string `json:"value"`
}

// SendSubscribe 发送一条订阅消息；access_token 过期时自动刷新并重试一次。
// 返回值可能是 *APIError，调用方用 IsPermanent 判断是否值得重试。
func (c *Client) SendSubscribe(ctx context.Context, msg SubscribeMessage) error {
	if strings.TrimSpace(msg.ToUser) == "" {
		return errors.New("订阅消息缺少 openid")
	}
	if strings.TrimSpace(msg.TemplateID) == "" {
		return errors.New("订阅消息缺少 template_id")
	}
	body := map[string]any{
		"touser":      msg.ToUser,
		"template_id": msg.TemplateID,
		"lang":        defaultString(msg.Lang, "zh_CN"),
		"data":        toSubscribeData(msg.Data),
	}
	if msg.Page != "" {
		body["page"] = msg.Page
	}
	if msg.MiniprogramState != "" {
		body["miniprogram_state"] = msg.MiniprogramState
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("序列化订阅消息: %w", err)
	}

	err = c.sendOnce(ctx, payload)
	if isTokenExpired(err) {
		c.invalidate()
		err = c.sendOnce(ctx, payload)
	}
	return err
}

func (c *Client) sendOnce(ctx context.Context, payload []byte) error {
	token, err := c.AccessToken(ctx)
	if err != nil {
		return err
	}
	endpoint := c.baseURL + "/cgi-bin/message/subscribe/send?access_token=" + token
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("构造订阅消息请求: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("发送订阅消息: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return fmt.Errorf("读取订阅消息响应: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("订阅消息接口 HTTP %d", resp.StatusCode)
	}
	var out struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("解析订阅消息响应: %w", err)
	}
	if out.ErrCode != 0 {
		return &APIError{Op: "subscribe.send", Code: out.ErrCode, Msg: out.ErrMsg}
	}
	return nil
}

func toSubscribeData(in map[string]string) map[string]subscribeValue {
	out := make(map[string]subscribeValue, len(in))
	for k, v := range in {
		out[k] = subscribeValue{Value: v}
	}
	return out
}

func isTokenExpired(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	// 40001 invalid credential / 42001 access_token expired。
	return apiErr.Code == 40001 || apiErr.Code == 42001
}

func defaultString(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
