package biz

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/bishop9910/hm-lesson/backend/pkg/xlsximport"
)

var (
	ErrImportInvalid    = errors.New("导入参数无效")
	ErrImportNotFound   = errors.New("导入批次不存在")
	ErrImportHasErrors  = errors.New("存在校验错误，请先修正")
	ErrImportState      = errors.New("导入批次状态不允许该操作")
	ErrImportReferenced = errors.New("批次内学员已有跟进或业务数据，不能回滚")
)

// ImportRow 是导入预览保存的一行。
type ImportRow = xlsximport.Row

// ImportBatch 对应 import_batches。
type ImportBatch struct {
	ID           string             `json:"id"`
	Kind         string             `json:"kind"`
	Status       string             `json:"status"`
	Mode         string             `json:"mode"`
	TotalRows    int                `json:"total_rows"`
	CreatedRows  int                `json:"created_rows"`
	SkippedRows  int                `json:"skipped_rows"`
	ErrorCount   int                `json:"error_count"`
	Issues       []xlsximport.Issue `json:"issues"`
	CreatedBy    string             `json:"created_by"`
	CreatedAt    time.Time          `json:"created_at"`
	CommittedAt  *time.Time         `json:"committed_at,omitempty"`
	RolledBackAt *time.Time         `json:"rolled_back_at,omitempty"`
}

// ImportPreview 是预览接口的返回。
type ImportPreview struct {
	Batch    *ImportBatch `json:"batch"`
	ToCreate int          `json:"to_create"`
	ToSkip   int          `json:"to_skip"`
}

// ImportRepo 由 data 层实现。
type ImportRepo interface {
	CreateImportBatch(ctx context.Context, batch *ImportBatch, rows []ImportRow) error
	GetImportBatch(ctx context.Context, id string) (*ImportBatch, []ImportRow, error)
	CommitImportBatch(ctx context.Context, id, staffID string) (*ImportBatch, error)
	RollbackImportBatch(ctx context.Context, id string) (*ImportBatch, error)
	StudentExistsByNamePhone(ctx context.Context, name, phone string) (bool, error)
}

// ImportUsecase 处理学员 Excel 预览/提交/回滚。
type ImportUsecase struct {
	repo ImportRepo
}

func NewImportUsecase(repo ImportRepo) *ImportUsecase {
	return &ImportUsecase{repo: repo}
}

// Template 返回导入模板。
func (u *ImportUsecase) Template() ([]byte, error) {
	return xlsximport.Template()
}

// Preview 解析并保存预览批次。
func (u *ImportUsecase) Preview(ctx context.Context, data []byte, mode, staffID string) (*ImportPreview, error) {
	if len(data) == 0 {
		return nil, ErrImportInvalid
	}
	if len(data) > 5<<20 {
		return nil, errors.New("文件不能超过 5MB")
	}
	if mode == "" {
		mode = "create"
	}
	if mode != "create" && mode != "update" {
		return nil, ErrImportInvalid
	}
	parsed, err := xlsximport.Parse(data)
	if err != nil {
		return nil, err
	}
	batch := &ImportBatch{
		ID:         NewID(),
		Kind:       "students",
		Status:     "preview",
		Mode:       mode,
		TotalRows:  parsed.Total,
		ErrorCount: len(parsed.Issues),
		Issues:     parsed.Issues,
		CreatedBy:  staffID,
		CreatedAt:  time.Now().UTC(),
	}
	toCreate, toSkip := 0, 0
	for _, row := range parsed.Rows {
		exists, err := u.repo.StudentExistsByNamePhone(ctx, row.Name, row.Phone)
		if err != nil {
			return nil, err
		}
		if exists {
			toSkip++
		} else {
			toCreate++
		}
	}
	if err := u.repo.CreateImportBatch(ctx, batch, parsed.Rows); err != nil {
		return nil, err
	}
	return &ImportPreview{Batch: batch, ToCreate: toCreate, ToSkip: toSkip}, nil
}

// Commit 提交预览批次。
func (u *ImportUsecase) Commit(ctx context.Context, id, staffID string) (*ImportBatch, error) {
	if strings.TrimSpace(id) == "" {
		return nil, ErrImportNotFound
	}
	return u.repo.CommitImportBatch(ctx, id, staffID)
}

// Get 读取批次。
func (u *ImportUsecase) Get(ctx context.Context, id string) (*ImportBatch, error) {
	batch, _, err := u.repo.GetImportBatch(ctx, id)
	return batch, err
}

// Rollback 回滚批次新建的学员。
func (u *ImportUsecase) Rollback(ctx context.Context, id string) (*ImportBatch, error) {
	return u.repo.RollbackImportBatch(ctx, id)
}
