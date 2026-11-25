package wx

import (
	"context"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// closedListener 返回一个刚关闭的本地端口地址，用来制造真实的网络错误。
func closedAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听失败: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("关闭监听失败: %v", err)
	}
	return addr
}

// 线上事故：net/http 的网络错误会把完整 URL 拼进错误文本，secret 就跟着写进了云托管日志。
func TestAccessTokenErrorRedactsSecret(t *testing.T) {
	const secret = "s3cr3t-do-not-log"
	client := NewClientWithBaseURL("appid", secret, "http://"+closedAddr(t), &http.Client{Timeout: 2 * time.Second})

	_, err := client.AccessToken(context.Background())
	if err == nil {
		t.Fatal("期望网络错误，实际成功")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("错误信息泄露了 app secret: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "请求 access_token") {
		t.Fatalf("错误信息缺少上下文: %s", err.Error())
	}
}

func TestSendSubscribeErrorRedactsToken(t *testing.T) {
	const secret = "s3cr3t-do-not-log"
	const token = "tok-must-not-leak"

	client := NewClientWithBaseURL("appid", secret, "http://"+closedAddr(t), &http.Client{Timeout: 2 * time.Second})
	client.token = token
	client.tokenExpiry = time.Now().Add(time.Hour)

	err := client.SendSubscribe(context.Background(), SubscribeMessage{
		ToUser:     "openid-1",
		TemplateID: "tmpl-1",
		Data:       map[string]string{"thing1": "作业"},
	})
	if err == nil {
		t.Fatal("期望网络错误，实际成功")
	}
	if strings.Contains(err.Error(), token) || strings.Contains(err.Error(), secret) {
		t.Fatalf("错误信息泄露了 token/secret: %s", err.Error())
	}
}

func TestRedact(t *testing.T) {
	got := redact("url?secret=abc&access_token=xyz", "abc", "xyz")
	if strings.Contains(got, "abc") || strings.Contains(got, "xyz") {
		t.Fatalf("脱敏失败: %s", got)
	}
	if got != "url?secret=***&access_token=***" {
		t.Fatalf("脱敏结果 = %q", got)
	}
	if redact("plain", "", "") != "plain" {
		t.Fatal("空敏感串不应修改文本")
	}
}
