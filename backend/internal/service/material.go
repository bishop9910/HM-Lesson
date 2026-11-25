package service

import (
	"errors"
	"net/http"
	"strings"
	"time"

	khttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/bishop9910/hm-lesson/backend/internal/biz"
)

// ---- DTO ----

type materialFileDTO struct {
	Name   string `json:"name"`
	FileID string `json:"file_id"`
	Size   int64  `json:"size,omitempty"`
}

type materialDTO struct {
	ID            string            `json:"id"`
	CategoryID    string            `json:"category_id"`
	CategoryName  string            `json:"category_name"`
	CategoryCode  string            `json:"category_code"`
	Title         string            `json:"title"`
	Description   string            `json:"description"`
	Grade         string            `json:"grade"`
	Status        string            `json:"status"`
	Files         []materialFileDTO `json:"files"`
	PublishedAt   string            `json:"published_at"`
	VisibleGroups []string          `json:"visible_groups"`
	CreatedAt     string            `json:"created_at"`
	UpdatedAt     string            `json:"updated_at"`
}

type materialCategoryDTO struct {
	ID            string          `json:"id"`
	Code          string          `json:"code"`
	Name          string          `json:"name"`
	Sort          int             `json:"sort"`
	Materials     int             `json:"materials"`
	Published     int             `json:"published"`
	VisibleGroups []string        `json:"visible_groups"`
	Visibility    map[string]bool `json:"visibility"`
}

func materialTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

func toMaterialDTO(material *biz.Material) materialDTO {
	files := make([]materialFileDTO, 0, len(material.Files))
	for _, f := range material.Files {
		files = append(files, materialFileDTO{Name: f.Name, FileID: f.FileID, Size: f.Size})
	}
	published := ""
	if material.PublishedAt != nil {
		published = materialTime(*material.PublishedAt)
	}
	return materialDTO{
		ID:            material.ID,
		CategoryID:    material.CategoryID,
		CategoryName:  material.CategoryName,
		CategoryCode:  material.CategoryCode,
		Title:         material.Title,
		Description:   material.Description,
		Grade:         material.Grade,
		Status:        material.Status,
		Files:         files,
		PublishedAt:   published,
		VisibleGroups: material.VisibleGroups,
		CreatedAt:     materialTime(material.CreatedAt),
		UpdatedAt:     materialTime(material.UpdatedAt),
	}
}

func toMaterialCategoryDTO(category *biz.MaterialCategory) materialCategoryDTO {
	return materialCategoryDTO{
		ID:            category.ID,
		Code:          category.Code,
		Name:          category.Name,
		Sort:          category.Sort,
		Materials:     category.Materials,
		Published:     category.Published,
		VisibleGroups: category.VisibleGroups(),
		Visibility:    category.Visibility,
	}
}

func (s *Server) requireMaterials(ctx khttp.Context) error {
	if s.materials == nil {
		return fail(ctx, http.StatusServiceUnavailable, "MATERIAL_DISABLED", "资料库未启用")
	}
	return nil
}

// ---- 老师端 ----

// ListMaterials 列出资料（可按分类 / 状态 / 关键词筛选）。
func (s *Server) ListMaterials(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	if err := s.requireMaterials(ctx); err != nil {
		return err
	}
	query := ctx.Query()
	items, err := s.materials.Materials(ctx, biz.MaterialFilter{
		CategoryID: strings.TrimSpace(query.Get("category_id")),
		Status:     strings.TrimSpace(query.Get("status")),
		Query:      query.Get("q"),
	})
	if err != nil {
		return s.mapMaterialError(ctx, err)
	}
	out := make([]materialDTO, 0, len(items))
	for i := range items {
		out = append(out, toMaterialDTO(&items[i]))
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": out})
}

type materialRequest struct {
	CategoryID  string            `json:"category_id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Grade       string            `json:"grade"`
	Files       []materialFileDTO `json:"files"`
}

func (b materialRequest) input() biz.MaterialInput {
	files := make([]biz.MaterialFile, 0, len(b.Files))
	for _, f := range b.Files {
		files = append(files, biz.MaterialFile{Name: f.Name, FileID: f.FileID, Size: f.Size})
	}
	return biz.MaterialInput{
		CategoryID:  b.CategoryID,
		Title:       b.Title,
		Description: b.Description,
		Grade:       b.Grade,
		Files:       files,
	}
}

// CreateMaterial 新建资料（默认草稿）。
func (s *Server) CreateMaterial(ctx khttp.Context) error {
	actor, err := s.requireStaff(ctx, false)
	if err != nil {
		return err
	}
	if err := s.requireMaterials(ctx); err != nil {
		return err
	}
	var body materialRequest
	if err := ctx.Bind(&body); err != nil {
		return fail(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求格式错误")
	}
	material, err := s.materials.CreateMaterial(ctx, actor, body.input())
	if err != nil {
		return s.mapMaterialError(ctx, err)
	}
	return ctx.JSON(http.StatusCreated, map[string]any{"material": toMaterialDTO(material)})
}

// GetMaterial 单份资料详情。
func (s *Server) GetMaterial(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	if err := s.requireMaterials(ctx); err != nil {
		return err
	}
	material, err := s.materials.Material(ctx, ctx.Vars().Get("id"))
	if err != nil {
		return s.mapMaterialError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"material": toMaterialDTO(material)})
}

// UpdateMaterial 修改资料内容。
func (s *Server) UpdateMaterial(ctx khttp.Context) error {
	actor, err := s.requireStaff(ctx, false)
	if err != nil {
		return err
	}
	if err := s.requireMaterials(ctx); err != nil {
		return err
	}
	var body materialRequest
	if err := ctx.Bind(&body); err != nil {
		return fail(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求格式错误")
	}
	material, err := s.materials.UpdateMaterial(ctx, actor, ctx.Vars().Get("id"), body.input())
	if err != nil {
		return s.mapMaterialError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"material": toMaterialDTO(material)})
}

// PublishMaterial 发布资料。
func (s *Server) PublishMaterial(ctx khttp.Context) error {
	actor, err := s.requireStaff(ctx, false)
	if err != nil {
		return err
	}
	if err := s.requireMaterials(ctx); err != nil {
		return err
	}
	material, err := s.materials.Publish(ctx, actor, ctx.Vars().Get("id"))
	if err != nil {
		return s.mapMaterialError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"material": toMaterialDTO(material)})
}

// ArchiveMaterial 归档资料。
func (s *Server) ArchiveMaterial(ctx khttp.Context) error {
	actor, err := s.requireStaff(ctx, false)
	if err != nil {
		return err
	}
	if err := s.requireMaterials(ctx); err != nil {
		return err
	}
	material, err := s.materials.Archive(ctx, actor, ctx.Vars().Get("id"))
	if err != nil {
		return s.mapMaterialError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"material": toMaterialDTO(material)})
}

// DeleteMaterial 删除资料（仅 owner）。
func (s *Server) DeleteMaterial(ctx khttp.Context) error {
	actor, err := s.requireStaff(ctx, true)
	if err != nil {
		return err
	}
	if err := s.requireMaterials(ctx); err != nil {
		return err
	}
	if err := s.materials.DeleteMaterial(ctx, actor, ctx.Vars().Get("id")); err != nil {
		return s.mapMaterialError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"ok": true})
}

// ListMaterialCategories 分类 + 分组可见矩阵。
func (s *Server) ListMaterialCategories(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	if err := s.requireMaterials(ctx); err != nil {
		return err
	}
	categories, err := s.materials.Categories(ctx)
	if err != nil {
		return s.mapMaterialError(ctx, err)
	}
	out := make([]materialCategoryDTO, 0, len(categories))
	for i := range categories {
		out = append(out, toMaterialCategoryDTO(&categories[i]))
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": out, "groups": biz.MaterialGroups})
}

type materialCategoryRequest struct {
	Code       string          `json:"code"`
	Name       string          `json:"name"`
	Sort       int             `json:"sort"`
	Visibility map[string]bool `json:"visibility"`
}

// CreateMaterialCategory 新建分类（owner）。
func (s *Server) CreateMaterialCategory(ctx khttp.Context) error {
	actor, err := s.requireStaff(ctx, true)
	if err != nil {
		return err
	}
	if err := s.requireMaterials(ctx); err != nil {
		return err
	}
	var body materialCategoryRequest
	if err := ctx.Bind(&body); err != nil {
		return fail(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求格式错误")
	}
	category, err := s.materials.CreateCategory(ctx, actor, body.Code, body.Name, body.Sort)
	if err != nil {
		return s.mapMaterialError(ctx, err)
	}
	return ctx.JSON(http.StatusCreated, map[string]any{"category": toMaterialCategoryDTO(category)})
}

// UpdateMaterialCategory 改分类名 / 排序（owner）。
func (s *Server) UpdateMaterialCategory(ctx khttp.Context) error {
	actor, err := s.requireStaff(ctx, true)
	if err != nil {
		return err
	}
	if err := s.requireMaterials(ctx); err != nil {
		return err
	}
	var body materialCategoryRequest
	if err := ctx.Bind(&body); err != nil {
		return fail(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求格式错误")
	}
	category, err := s.materials.UpdateCategory(ctx, actor, ctx.Vars().Get("id"), body.Name, body.Sort)
	if err != nil {
		return s.mapMaterialError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"category": toMaterialCategoryDTO(category)})
}

// SetMaterialCategoryVisibility 更新分组可见矩阵（owner）。
func (s *Server) SetMaterialCategoryVisibility(ctx khttp.Context) error {
	actor, err := s.requireStaff(ctx, true)
	if err != nil {
		return err
	}
	if err := s.requireMaterials(ctx); err != nil {
		return err
	}
	var body materialCategoryRequest
	if err := ctx.Bind(&body); err != nil {
		return fail(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求格式错误")
	}
	category, err := s.materials.SetCategoryVisibility(ctx, actor, ctx.Vars().Get("id"), body.Visibility)
	if err != nil {
		return s.mapMaterialError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"category": toMaterialCategoryDTO(category)})
}

type materialDownloadDTO struct {
	ID           string `json:"id"`
	MaterialID   string `json:"material_id"`
	MaterialName string `json:"material_name"`
	GuardianName string `json:"guardian_name"`
	StudentName  string `json:"student_name"`
	CreatedAt    string `json:"created_at"`
}

// ListMaterialDownloads 下载记录（owner 排查用）。
func (s *Server) ListMaterialDownloads(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, true); err != nil {
		return err
	}
	if err := s.requireMaterials(ctx); err != nil {
		return err
	}
	logs, err := s.materials.DownloadLogs(ctx, ctx.Query().Get("material_id"), 100)
	if err != nil {
		return s.mapMaterialError(ctx, err)
	}
	out := make([]materialDownloadDTO, 0, len(logs))
	for _, item := range logs {
		out = append(out, materialDownloadDTO{
			ID:           item.ID,
			MaterialID:   item.MaterialID,
			MaterialName: item.MaterialName,
			GuardianName: item.GuardianName,
			StudentName:  item.StudentName,
			CreatedAt:    materialTime(item.CreatedAt),
		})
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": out})
}

// ---- 家长端 ----

// PortalMaterials 家长可见资料（服务端按分组矩阵过滤）。
func (s *Server) PortalMaterials(ctx khttp.Context) error {
	guardianID, err := s.requireGuardian(ctx)
	if err != nil {
		return err
	}
	if err := s.requireMaterials(ctx); err != nil {
		return err
	}
	items, err := s.materials.PortalMaterials(ctx, guardianID)
	if err != nil {
		return s.mapMaterialError(ctx, err)
	}
	out := make([]materialDTO, 0, len(items))
	for i := range items {
		out = append(out, toMaterialDTO(&items[i]))
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": out})
}

// PortalDownloadMaterial 家长下载：校验可见性 → 记下载日志 → 返回 fileID 列表。
func (s *Server) PortalDownloadMaterial(ctx khttp.Context) error {
	guardianID, err := s.requireGuardian(ctx)
	if err != nil {
		return err
	}
	if err := s.requireMaterials(ctx); err != nil {
		return err
	}
	material, err := s.materials.PortalDownload(ctx, guardianID, ctx.Vars().Get("id"), ctx.Request().RemoteAddr)
	if err != nil {
		return s.mapMaterialError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"material": toMaterialDTO(material)})
}

func (s *Server) mapMaterialError(ctx khttp.Context, err error) error {
	switch {
	case errors.Is(err, biz.ErrMaterialNotFound):
		return fail(ctx, http.StatusNotFound, "MATERIAL_NOT_FOUND", "资料不存在")
	case errors.Is(err, biz.ErrMaterialCategoryNotFound):
		return fail(ctx, http.StatusNotFound, "MATERIAL_CATEGORY_NOT_FOUND", "资料分类不存在")
	case errors.Is(err, biz.ErrMaterialState):
		return fail(ctx, http.StatusConflict, "MATERIAL_STATE", "资料当前状态不允许该操作")
	case errors.Is(err, biz.ErrInvalidMaterial):
		return fail(ctx, http.StatusBadRequest, "VALIDATION_FAILED", "资料参数不合法")
	case errors.Is(err, biz.ErrForbidden):
		return fail(ctx, http.StatusForbidden, "FORBIDDEN", "没有权限执行该操作")
	default:
		return fail(ctx, http.StatusInternalServerError, "INTERNAL", "操作失败")
	}
}
