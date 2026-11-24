// Package service 提供 HTTP 适配层；不写业务规则。
package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	khttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/bishop9910/hm-lesson/backend/internal/biz"
	"github.com/bishop9910/hm-lesson/backend/pkg/token"
)

type Server struct {
	uc        *biz.AuthUsecase
	dashboard *biz.DashboardUsecase
	bind      *biz.BindUsecase
	crm       *biz.CRMUsecase
	importUC  *biz.ImportUsecase
	tokens    *token.Issuer
}

func New(uc *biz.AuthUsecase, dashboard *biz.DashboardUsecase, bind *biz.BindUsecase, crm *biz.CRMUsecase, importUC *biz.ImportUsecase, tokens *token.Issuer) *Server {
	return &Server{uc: uc, dashboard: dashboard, bind: bind, crm: crm, importUC: importUC, tokens: tokens}
}

type loginRequest struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

type staffDTO struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Phone  string `json:"phone"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

func toDTO(s *biz.Staff) staffDTO {
	return staffDTO{ID: s.ID, Name: s.Name, Phone: s.Phone, Role: s.Role, Status: s.Status}
}

func (s *Server) Login(ctx khttp.Context) error {
	if s.uc == nil {
		return fail(ctx, http.StatusServiceUnavailable, "AUTH_DISABLED", "数据库未配置")
	}
	var req loginRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求格式错误")
	}
	staff, tok, exp, err := s.uc.Login(ctx, req.Phone, req.Password)
	if err != nil {
		if errors.Is(err, biz.ErrInvalidCredentials) {
			return fail(ctx, http.StatusUnauthorized, "INVALID_CREDENTIALS", "手机号或密码错误")
		}
		return fail(ctx, http.StatusInternalServerError, "INTERNAL", "登录失败")
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"token":      tok,
		"expires_at": exp.UTC().Format("2006-01-02T15:04:05Z"),
		"staff":      toDTO(staff),
	})
}

func (s *Server) Me(ctx khttp.Context) error {
	staff, err := s.requireStaff(ctx, false)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, map[string]any{"staff": toDTO(staff)})
}

func (s *Server) ListStaff(ctx khttp.Context) error {
	staff, err := s.requireStaff(ctx, true)
	if err != nil {
		return err
	}
	list, err := s.uc.ListStaff(ctx, staff)
	if err != nil {
		return mapBizError(ctx, err)
	}
	out := make([]staffDTO, 0, len(list))
	for _, item := range list {
		out = append(out, toDTO(item))
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": out})
}

type createStaffRequest struct {
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func (s *Server) CreateStaff(ctx khttp.Context) error {
	actor, err := s.requireStaff(ctx, true)
	if err != nil {
		return err
	}
	var req createStaffRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求格式错误")
	}
	created, err := s.uc.CreateStaff(ctx, actor, biz.CreateStaffInput{
		Name: req.Name, Phone: req.Phone, Password: req.Password, Role: req.Role,
	})
	if err != nil {
		return mapBizError(ctx, err)
	}
	return ctx.JSON(http.StatusCreated, map[string]any{"staff": toDTO(created)})
}

type updateStaffRequest struct {
	Name     *string `json:"name"`
	Role     *string `json:"role"`
	Status   *string `json:"status"`
	Password *string `json:"password"`
}

func (s *Server) UpdateStaff(ctx khttp.Context) error {
	actor, err := s.requireStaff(ctx, true)
	if err != nil {
		return err
	}
	id := ctx.Vars().Get("id")
	if id == "" {
		return fail(ctx, http.StatusBadRequest, "BAD_REQUEST", "缺少 id")
	}
	var req updateStaffRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求格式错误")
	}
	updated, err := s.uc.UpdateStaff(ctx, actor, id, biz.UpdateStaffInput{
		Name: req.Name, Role: req.Role, Status: req.Status, Password: req.Password,
	})
	if err != nil {
		return mapBizError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"staff": toDTO(updated)})
}

// requireStaff 解析 Bearer token 并回查账号状态；ownerOnly 用于成员管理。
func (s *Server) requireStaff(ctx khttp.Context, ownerOnly bool) (*biz.Staff, error) {
	if s.uc == nil || s.tokens == nil {
		return nil, fail(ctx, http.StatusServiceUnavailable, "AUTH_DISABLED", "数据库未配置")
	}
	raw := strings.TrimSpace(ctx.Header().Get("Authorization"))
	if raw == "" {
		// callContainer 场景由小程序用 X-HM-Token 传同一个 JWT，便于与网关注入头区分。
		if v := strings.TrimSpace(ctx.Header().Get("X-HM-Token")); v != "" {
			raw = "Bearer " + v
		}
	}
	if !strings.HasPrefix(raw, "Bearer ") {
		return nil, fail(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "未登录")
	}
	claims, err := s.tokens.Parse(strings.TrimSpace(strings.TrimPrefix(raw, "Bearer ")))
	if err != nil {
		return nil, fail(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "登录已过期")
	}
	staff, err := s.uc.Me(ctx, claims.Subject)
	if err != nil {
		return nil, fail(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "账号不可用")
	}
	if ownerOnly && staff.Role != biz.RoleOwner {
		return nil, fail(ctx, http.StatusForbidden, "FORBIDDEN", "仅负责人可操作")
	}
	return staff, nil
}

func mapBizError(ctx khttp.Context, err error) error {
	switch {
	case errors.Is(err, biz.ErrForbidden):
		return fail(ctx, http.StatusForbidden, "FORBIDDEN", "没有权限")
	case errors.Is(err, biz.ErrPhoneTaken):
		return fail(ctx, http.StatusConflict, "PHONE_TAKEN", "手机号已被使用")
	case errors.Is(err, biz.ErrStaffNotFound):
		return fail(ctx, http.StatusNotFound, "NOT_FOUND", "老师不存在")
	case errors.Is(err, biz.ErrStaffDisabled):
		return fail(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "账号已停用")
	default:
		return fail(ctx, http.StatusInternalServerError, "INTERNAL", "操作失败")
	}
}

func fail(ctx khttp.Context, code int, reason, message string) error {
	return ctx.JSON(code, map[string]any{
		"reason":  reason,
		"message": message,
	})
}

// WriteJSON 仅用于测试/工具，保持与路由返回一致。
func WriteJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
