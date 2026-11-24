package service

import (
	"net/http"

	khttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/bishop9910/hm-lesson/backend/internal/biz"
)

type bindRequest struct {
	Phone string `json:"phone"`
}

// RequestBind 记录家长绑定申请；公共接口，不需要登录。
func (s *Server) RequestBind(ctx khttp.Context) error {
	if s.bind == nil {
		return fail(ctx, http.StatusServiceUnavailable, "BIND_DISABLED", "绑定服务未就绪")
	}
	var req bindRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求格式错误")
	}
	status, err := s.bind.Request(ctx, req.Phone)
	if err != nil {
		switch {
		case err == biz.ErrInvalidPhone:
			return fail(ctx, http.StatusBadRequest, "INVALID_PHONE", "请输入 11 位手机号")
		case err == biz.ErrBindDisabled:
			return fail(ctx, http.StatusServiceUnavailable, "BIND_DISABLED", "绑定服务未就绪")
		default:
			return fail(ctx, http.StatusInternalServerError, "INTERNAL", "提交失败")
		}
	}
	return ctx.JSON(http.StatusOK, map[string]any{"status": status})
}
