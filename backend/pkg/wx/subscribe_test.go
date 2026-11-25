package wx

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestSendSubscribeSuccess(t *testing.T) {
	var tokenCalls, sendCalls int32
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/token":
			atomic.AddInt32(&tokenCalls, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok-1", "expires_in": 7200})
		case "/cgi-bin/message/subscribe/send":
			atomic.AddInt32(&sendCalls, 1)
			if r.URL.Query().Get("access_token") != "tok-1" {
				t.Errorf("access_token = %q", r.URL.Query().Get("access_token"))
			}
			raw, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(raw, &gotBody); err != nil {
				t.Errorf("请求体解析失败: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 0, "errmsg": "ok"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClientWithBaseURL("appid", "secret", srv.URL, srv.Client())
	msg := SubscribeMessage{
		ToUser:           "openid-1",
		TemplateID:       "tmpl-1",
		Page:             "pages/portal/tasks/detail?id=a1",
		MiniprogramState: "formal",
		Data:             map[string]string{"thing1": "物理作业"},
	}
	if err := client.SendSubscribe(context.Background(), msg); err != nil {
		t.Fatalf("SendSubscribe: %v", err)
	}
	if tokenCalls != 1 || sendCalls != 1 {
		t.Fatalf("tokenCalls=%d sendCalls=%d", tokenCalls, sendCalls)
	}
	if gotBody["touser"] != "openid-1" || gotBody["template_id"] != "tmpl-1" || gotBody["miniprogram_state"] != "formal" {
		t.Fatalf("请求体错误: %+v", gotBody)
	}
	data, ok := gotBody["data"].(map[string]any)
	if !ok {
		t.Fatalf("data 缺失: %+v", gotBody)
	}
	thing1, _ := data["thing1"].(map[string]any)
	if thing1["value"] != "物理作业" {
		t.Fatalf("thing1 = %+v", data["thing1"])
	}

	// 第二次发送应复用缓存的 token。
	if err := client.SendSubscribe(context.Background(), msg); err != nil {
		t.Fatalf("第二次 SendSubscribe: %v", err)
	}
	if tokenCalls != 1 {
		t.Fatalf("token 未缓存，tokenCalls=%d", tokenCalls)
	}
}

func TestSendSubscribeRefreshesExpiredToken(t *testing.T) {
	var tokenCalls int32
	var sendCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/token":
			n := atomic.AddInt32(&tokenCalls, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok-" + string(rune('0'+n)), "expires_in": 7200})
		case "/cgi-bin/message/subscribe/send":
			n := atomic.AddInt32(&sendCalls, 1)
			if n == 1 {
				_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 40001, "errmsg": "invalid credential"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 0, "errmsg": "ok"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewClientWithBaseURL("appid", "secret", srv.URL, srv.Client())
	err := client.SendSubscribe(context.Background(), SubscribeMessage{
		ToUser: "openid-1", TemplateID: "tmpl-1", Data: map[string]string{},
	})
	if err != nil {
		t.Fatalf("应重试后成功: %v", err)
	}
	if tokenCalls != 2 || sendCalls != 2 {
		t.Fatalf("tokenCalls=%d sendCalls=%d", tokenCalls, sendCalls)
	}
}

func TestAPIErrorClassify(t *testing.T) {
	cases := []struct {
		code      int
		permanent bool
	}{
		{40003, true},
		{40037, true},
		{41030, true},
		{43101, true},
		{47003, true},
		{45009, false},
		{0, false},
	}
	for _, c := range cases {
		err := error(&APIError{Op: "subscribe.send", Code: c.code, Msg: "x"})
		if got := IsPermanent(err); got != c.permanent {
			t.Fatalf("IsPermanent(%d) = %v, want %v", c.code, got, c.permanent)
		}
	}
	if IsPermanent(context.Canceled) {
		t.Fatal("非 APIError 不应判为永久失败")
	}
}

func TestSendSubscribeRequiresOpenID(t *testing.T) {
	client := NewClientWithBaseURL("appid", "secret", "http://127.0.0.1:1", nil)
	if err := client.SendSubscribe(context.Background(), SubscribeMessage{TemplateID: "t"}); err == nil {
		t.Fatal("缺少 openid 应报错")
	}
}

// 云调用模式：不取 access_token，请求也不带 access_token 参数（平台网关鉴权）。
func TestSendSubscribeCloudModeSkipsToken(t *testing.T) {
	var tokenCalls, sendCalls int32
	var sendQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/token":
			atomic.AddInt32(&tokenCalls, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "should-not-be-used", "expires_in": 7200})
		case "/cgi-bin/message/subscribe/send":
			atomic.AddInt32(&sendCalls, 1)
			sendQuery = r.URL.RawQuery
			_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 0, "errmsg": "ok"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := NewCloudClientWithBaseURL("appid", srv.URL, srv.Client())
	if !client.InCloudMode() {
		t.Fatal("InCloudMode 应为 true")
	}
	// 云调用模式不要求 AppSecret。
	err := client.SendSubscribe(context.Background(), SubscribeMessage{
		ToUser:     "openid-1",
		TemplateID: "tmpl-1",
		Data:       map[string]string{"thing1": "作业"},
	})
	if err != nil {
		t.Fatalf("发送失败: %v", err)
	}
	if tokenCalls != 0 {
		t.Fatalf("云调用模式不应请求 access_token，实际 %d 次", tokenCalls)
	}
	if sendCalls != 1 {
		t.Fatalf("send 调用次数 = %d", sendCalls)
	}
	if sendQuery != "" {
		t.Fatalf("云调用请求不应带 query，实际 %q", sendQuery)
	}
}

func TestAccessTokenRejectsCloudMode(t *testing.T) {
	client := NewCloudClientWithBaseURL("appid", "http://127.0.0.1:1", nil)
	if _, err := client.AccessToken(context.Background()); err == nil {
		t.Fatal("云调用模式调用 AccessToken 应报错")
	}
}
