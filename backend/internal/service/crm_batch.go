package service

import (
	"errors"
	"fmt"
	"net/http"

	khttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/bishop9910/hm-lesson/backend/internal/biz"
)

type batchDeleteRequest struct {
	IDs   []string `json:"ids"`
	Force bool     `json:"force"`
}

// BatchDeleteStudents 批量删除学员（owner）。force=true 时连同课时流水一起删。
func (s *Server) BatchDeleteStudents(ctx khttp.Context) error {
	actor, err := s.requireStaff(ctx, true)
	if err != nil {
		return err
	}
	var req batchDeleteRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求格式错误")
	}
	if req.Force {
		req.Force = true
	}
	results, err := s.crm.DeleteStudents(ctx, actor, req.IDs, req.Force)
	if err != nil {
		return mapCRMBizError(ctx, err)
	}
	deleted, blocked := 0, 0
	for _, item := range results {
		if item.OK {
			deleted++
		} else {
			blocked++
		}
	}
	s.ops.Record(ctx, actor, "student.batch_delete", "student", "", fmt.Sprintf(
		`{"requested":%d,"deleted":%d,"blocked":%d,"force":%t}`,
		len(req.IDs), deleted, blocked, req.Force,
	))
	return ctx.JSON(http.StatusOK, map[string]any{
		"items":   results,
		"deleted": deleted,
		"blocked": blocked,
	})
}

// BatchDeleteGuardians 批量删除家长账号（owner）。有在读学员的那条会失败并带原因。
func (s *Server) BatchDeleteGuardians(ctx khttp.Context) error {
	actor, err := s.requireStaff(ctx, true)
	if err != nil {
		return err
	}
	var req batchDeleteRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求格式错误")
	}
	results, err := s.crm.DeleteGuardians(ctx, actor, req.IDs)
	if err != nil {
		if errors.Is(err, biz.ErrInvalidBatch) {
			return fail(ctx, http.StatusBadRequest, "VALIDATION_FAILED", "ids 不能为空，且单次最多 50 个")
		}
		return mapCRMBizError(ctx, err)
	}
	deleted, blocked := 0, 0
	for _, item := range results {
		if item.OK {
			deleted++
		} else {
			blocked++
		}
	}
	s.ops.Record(ctx, actor, "guardian.batch_delete", "guardian", "", fmt.Sprintf(
		`{"requested":%d,"deleted":%d,"blocked":%d}`,
		len(req.IDs), deleted, blocked,
	))
	return ctx.JSON(http.StatusOK, map[string]any{
		"items":   results,
		"deleted": deleted,
		"blocked": blocked,
	})
}
