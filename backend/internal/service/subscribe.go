package service

import (
	"errors"
	"net/http"
	"strings"

	khttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/bishop9910/hm-lesson/backend/internal/biz"
)

// gatewayOpenID 只信任云托管网关注入的身份头。
// AGENTS §7：没有 X-WX-SOURCE 时不能把 X-WX-OPENID 当身份，避免客户端自报 openid 冒充别人。
func gatewayOpenID(ctx khttp.Context) string {
	if strings.TrimSpace(ctx.Header().Get("X-WX-SOURCE")) == "" {
		return ""
	}
	return strings.TrimSpace(ctx.Header().Get("X-WX-OPENID"))
}

// PortalSubscriptions 返回三个订阅模板的配置与剩余额度，家长端据此决定弹哪些模板。
func (s *Server) PortalSubscriptions(ctx khttp.Context) error {
	gid, err := s.requireGuardian(ctx)
	if err != nil {
		return err
	}
	if s.subs == nil {
		return fail(ctx, http.StatusServiceUnavailable, "SUBSCRIPTION_DISABLED", "订阅消息未启用")
	}
	views, err := s.subs.Status(ctx, gid)
	if err != nil {
		return mapSubscribeError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": views, "enabled": s.subs.Enabled()})
}

type grantSubscriptionsRequest struct {
	Grants []struct {
		TemplateKey string `json:"template_key"`
		State       string `json:"state"`
	} `json:"grants"`
}

// GrantSubscriptions 接收 wx.requestSubscribeMessage 的结果；accept 才增加额度。
func (s *Server) GrantSubscriptions(ctx khttp.Context) error {
	gid, err := s.requireGuardian(ctx)
	if err != nil {
		return err
	}
	if s.subs == nil {
		return fail(ctx, http.StatusServiceUnavailable, "SUBSCRIPTION_DISABLED", "订阅消息未启用")
	}
	var req grantSubscriptionsRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求格式错误")
	}
	if len(req.Grants) == 0 || len(req.Grants) > len(biz.TemplateKeys) {
		return fail(ctx, http.StatusBadRequest, "INVALID_SUBSCRIPTION_KEY", "订阅项不合法")
	}
	inputs := make([]biz.GrantInput, 0, len(req.Grants))
	for _, g := range req.Grants {
		inputs = append(inputs, biz.GrantInput{TemplateKey: strings.TrimSpace(g.TemplateKey), State: strings.TrimSpace(g.State)})
	}
	views, err := s.subs.Grant(ctx, gid, inputs)
	if err != nil {
		return mapSubscribeError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": views})
}

func mapSubscribeError(ctx khttp.Context, err error) error {
	switch {
	case errors.Is(err, biz.ErrInvalidTemplateKey):
		return fail(ctx, http.StatusBadRequest, "INVALID_SUBSCRIPTION_KEY", "订阅项不合法")
	case errors.Is(err, biz.ErrSubscriptionOff):
		return fail(ctx, http.StatusServiceUnavailable, "SUBSCRIPTION_DISABLED", "订阅消息未启用")
	default:
		return fail(ctx, http.StatusInternalServerError, "INTERNAL", "操作失败")
	}
}
