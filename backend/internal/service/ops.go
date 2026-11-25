package service

import (
	"errors"
	"net/http"

	khttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/bishop9910/hm-lesson/backend/internal/biz"
)

// requireSystem 要求调用者是内部系统账号（staff.hidden）。
func (s *Server) requireSystem(ctx khttp.Context) (*biz.Staff, error) {
	actor, err := s.requireStaff(ctx, true)
	if err != nil {
		return nil, err
	}
	if err := biz.RequireSystem(actor); err != nil {
		return nil, fail(ctx, http.StatusForbidden, "SYSTEM_ONLY", "该功能仅内部系统账号可用")
	}
	return actor, nil
}

// auditEntryDTO 是操作记录的一行。
type auditEntryDTO struct {
	ID            string `json:"id"`
	ActorType     string `json:"actor_type"`
	ActorID       string `json:"actor_id"`
	ActorName     string `json:"actor_name"`
	Action        string `json:"action"`
	TargetType    string `json:"target_type"`
	TargetID      string `json:"target_id"`
	BeforeSummary string `json:"before_summary,omitempty"`
	AfterSummary  string `json:"after_summary,omitempty"`
	CreatedAt     string `json:"created_at"`
}

// SystemAudit 返回最近的操作记录（仅系统账号）。
func (s *Server) SystemAudit(ctx khttp.Context) error {
	if s.ops == nil {
		return fail(ctx, http.StatusServiceUnavailable, "OPS_DISABLED", "运维接口未启用")
	}
	if _, err := s.requireSystem(ctx); err != nil {
		return err
	}
	action := ctx.Query().Get("action")
	limit := 50
	if raw := ctx.Query().Get("limit"); raw != "" {
		if parsed, convErr := parsePositiveInt(raw); convErr == nil {
			limit = parsed
		}
	}
	items, err := s.ops.Audit(ctx, action, limit)
	if err != nil {
		return fail(ctx, http.StatusInternalServerError, "INTERNAL", "读取操作记录失败")
	}
	out := make([]auditEntryDTO, 0, len(items))
	for _, item := range items {
		out = append(out, auditEntryDTO{
			ID:            item.ID,
			ActorType:     item.ActorType,
			ActorID:       item.ActorID,
			ActorName:     item.ActorName,
			Action:        item.Action,
			TargetType:    item.TargetType,
			TargetID:      item.TargetID,
			BeforeSummary: item.BeforeSummary,
			AfterSummary:  item.AfterSummary,
			CreatedAt:     item.CreatedAt,
		})
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": out})
}

// systemPermissionDTO 是权限视图的一行。
type systemPermissionDTO struct {
	ID        string   `json:"id"`
	Phone     string   `json:"phone"`
	Name      string   `json:"name"`
	Role      string   `json:"role"`
	Status    string   `json:"status"`
	Hidden    bool     `json:"hidden"`
	CanManage bool     `json:"can_manage"`
	CanReview bool     `json:"can_review"`
	CanExport bool     `json:"can_export"`
	Caveats   []string `json:"caveats"`
}

// SystemPermissions 返回账号与权限矩阵（仅系统账号）。
func (s *Server) SystemPermissions(ctx khttp.Context) error {
	if s.ops == nil {
		return fail(ctx, http.StatusServiceUnavailable, "OPS_DISABLED", "运维接口未启用")
	}
	if _, err := s.requireSystem(ctx); err != nil {
		return err
	}
	items, err := s.ops.Permissions(ctx)
	if err != nil {
		return fail(ctx, http.StatusInternalServerError, "INTERNAL", "读取权限视图失败")
	}
	out := make([]systemPermissionDTO, 0, len(items))
	for _, item := range items {
		out = append(out, systemPermissionDTO{
			ID:        item.ID,
			Phone:     item.Phone,
			Name:      item.Name,
			Role:      item.Role,
			Status:    item.Status,
			Hidden:    item.Hidden,
			CanManage: item.CanManage,
			CanReview: item.CanReview,
			CanExport: item.CanExport,
			Caveats:   item.Caveats,
		})
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": out})
}

// systemQueryRequest 是只读控制台的请求体。
type systemQueryRequest struct {
	SQL string `json:"sql"`
}

// SystemQuery 执行只读 SQL（仅系统账号，最多 200 行、8 秒超时）。
func (s *Server) SystemQuery(ctx khttp.Context) error {
	if s.ops == nil {
		return fail(ctx, http.StatusServiceUnavailable, "OPS_DISABLED", "运维接口未启用")
	}
	actor, err := s.requireSystem(ctx)
	if err != nil {
		return err
	}
	var req systemQueryRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求格式错误")
	}
	result, err := s.ops.Query(ctx, req.SQL)
	if err != nil {
		switch {
		case errors.Is(err, biz.ErrInvalidQuery):
			return fail(ctx, http.StatusBadRequest, "INVALID_QUERY", "只允许执行只读查询（SELECT / SHOW / DESCRIBE / EXPLAIN），且单条语句")
		case errors.Is(err, biz.ErrQueryTooLong):
			return fail(ctx, http.StatusBadRequest, "INVALID_QUERY", "语句过长，请精简到 4000 字符以内")
		default:
			// 把数据库报错原样带回，方便在控制台里调试语法与列名。
			return fail(ctx, http.StatusBadRequest, "QUERY_FAILED", err.Error())
		}
	}
	rows := result.Rows
	if rows == nil {
		rows = [][]string{}
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"actor":     actor.Phone,
		"columns":   result.Columns,
		"rows":      rows,
		"truncated": result.Truncated,
		"max_rows":  biz.ConsoleMaxRows,
	})
}

// parsePositiveInt 解析大于 0 的十进制整数，失败返回错误。
func parsePositiveInt(raw string) (int, error) {
	value := 0
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return 0, errors.New("不是正整数")
		}
		value = value*10 + int(ch-'0')
		if value > 1_000_000 {
			return 0, errors.New("超出范围")
		}
	}
	if value <= 0 {
		return 0, errors.New("不是正整数")
	}
	return value, nil
}
