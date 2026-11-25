package main

import (
	"context"
	"log/slog"
	"net"
	"strings"
	"time"
)

// 出网体检用的两个域名：一个微信接口域名，一个对照域名。
// 对照域名能通、微信域名不通 ⇒ 平台把微信域名劫持到了内网网关（需要开通云调用/开放接口服务）；
// 两个都不通 ⇒ 容器没有公网出口。
const (
	egressWXHost      = "api.weixin.qq.com"
	egressControlHost = "www.baidu.com"
)

// probeEgress 启动时打印一次出网体检（只记日志，不阻塞启动）。
// 排查“订阅消息发不出去”时先看这几行，比反复猜平台开关快。
func probeEgress(logger *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	checkHost(ctx, logger, egressWXHost)
	checkHost(ctx, logger, egressControlHost)
}

func checkHost(ctx context.Context, logger *slog.Logger, host string) {
	addrs, err := net.DefaultResolver.LookupHost(ctx, host)
	if err != nil {
		logger.Warn("出网体检：DNS 解析失败", "host", host, "error", err.Error())
		return
	}
	logger.Info("出网体检：DNS 解析完成", "host", host, "addrs", strings.Join(addrs, ","), "link_local", anyLinkLocal(addrs))
	// 优先只连 IPv4：容器通常没有 IPv6 路由，逐条试 IPv6 只会刷出无意义的
	// "network is unreachable"，把真正的问题淹掉。
	targets := addrs
	if v4 := ipv4Only(addrs); len(v4) > 0 {
		targets = v4
	}
	for _, addr := range targets {
		dialer := net.Dialer{Timeout: 3 * time.Second}
		conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(addr, "443"))
		if err != nil {
			logger.Warn("出网体检：TCP 443 连接失败", "host", host, "addr", addr, "error", err.Error())
			continue
		}
		_ = conn.Close()
		logger.Info("出网体检：TCP 443 连接成功", "host", host, "addr", addr)
	}
}

// ipv4Only 过滤出 IPv4 地址；解析不到时返回空切片，调用方退回全部地址。
func ipv4Only(addrs []string) []string {
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		if ip := net.ParseIP(a); ip != nil && ip.To4() != nil {
			out = append(out, a)
		}
	}
	return out
}

// anyLinkLocal 判断解析结果里是否有 169.254.0.0/16 这类链路本地地址。
func anyLinkLocal(addrs []string) bool {
	for _, a := range addrs {
		ip := net.ParseIP(a)
		if ip == nil {
			continue
		}
		if ip4 := ip.To4(); ip4 != nil && ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
	}
	return false
}
