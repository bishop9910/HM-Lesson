package service

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	khttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/bishop9910/hm-lesson/backend/internal/biz"
)

type importPreviewRequest struct {
	ContentBase64 string `json:"content_base64"`
	Mode          string `json:"mode"`
}

// ImportTemplate 返回学员导入模板（base64 xlsx）。
func (s *Server) ImportTemplate(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, true); err != nil {
		return err
	}
	if s.importUC == nil {
		return ctx.JSON(http.StatusServiceUnavailable, map[string]string{"reason": "DB_UNAVAILABLE", "message": "数据库未配置"})
	}
	data, err := s.importUC.Template()
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, map[string]string{"reason": "TEMPLATE_FAILED", "message": "生成模板失败"})
	}
	return ctx.JSON(http.StatusOK, map[string]string{
		"filename":       "students-template.xlsx",
		"content_base64": base64.StdEncoding.EncodeToString(data),
	})
}

// PreviewStudentsImport 解析 Excel 并保存预览批次。
func (s *Server) PreviewStudentsImport(ctx khttp.Context) error {
	actor, err := s.requireStaff(ctx, true)
	if err != nil {
		return err
	}
	var req importPreviewRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(http.StatusBadRequest, map[string]string{"reason": "BAD_REQUEST", "message": "请求体格式错误"})
	}
	raw := strings.TrimSpace(req.ContentBase64)
	if raw == "" {
		return ctx.JSON(http.StatusBadRequest, map[string]string{"reason": "EMPTY_FILE", "message": "请选择 Excel 文件"})
	}
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, map[string]string{"reason": "BAD_FILE", "message": "文件内容不是有效的 base64"})
	}
	preview, err := s.importUC.Preview(ctx, data, req.Mode, actor.ID)
	if err != nil {
		return mapImportError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, preview)
}

// CommitStudentsImport 提交预览批次。
func (s *Server) CommitStudentsImport(ctx khttp.Context) error {
	actor, err := s.requireStaff(ctx, true)
	if err != nil {
		return err
	}
	batch, err := s.importUC.Commit(ctx, ctx.Vars().Get("id"), actor.ID)
	if err != nil {
		return mapImportError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"batch": batch})
}

// GetImportBatch 查询批次。
func (s *Server) GetImportBatch(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, true); err != nil {
		return err
	}
	batch, err := s.importUC.Get(ctx, ctx.Vars().Get("id"))
	if err != nil {
		return mapImportError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"batch": batch})
}

// RollbackImportBatch 回滚批次新建的学员。
func (s *Server) RollbackImportBatch(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, true); err != nil {
		return err
	}
	batch, err := s.importUC.Rollback(ctx, ctx.Vars().Get("id"))
	if err != nil {
		return mapImportError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"batch": batch})
}

func mapImportError(ctx khttp.Context, err error) error {
	switch {
	case errors.Is(err, biz.ErrImportNotFound):
		return ctx.JSON(http.StatusNotFound, map[string]string{"reason": "IMPORT_NOT_FOUND", "message": "导入批次不存在"})
	case errors.Is(err, biz.ErrImportHasErrors):
		return ctx.JSON(http.StatusConflict, map[string]string{"reason": "IMPORT_HAS_ERRORS", "message": "存在校验错误，请先修正 Excel"})
	case errors.Is(err, biz.ErrImportState):
		return ctx.JSON(http.StatusConflict, map[string]string{"reason": "IMPORT_STATE", "message": "当前批次状态不允许该操作"})
	case errors.Is(err, biz.ErrImportReferenced):
		return ctx.JSON(http.StatusConflict, map[string]string{"reason": "IMPORT_REFERENCED", "message": "批次内学员已有跟进或业务数据，不能回滚"})
	case errors.Is(err, biz.ErrImportInvalid):
		return ctx.JSON(http.StatusBadRequest, map[string]string{"reason": "IMPORT_INVALID", "message": "导入参数无效"})
	default:
		return ctx.JSON(http.StatusBadRequest, map[string]string{"reason": "IMPORT_FAILED", "message": err.Error()})
	}
}
