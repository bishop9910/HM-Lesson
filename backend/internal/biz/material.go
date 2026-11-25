package biz

import (
	"context"
	"errors"
	"strings"
	"time"
)

// 资料状态机（docs/backend-requirements.md §3.6）。
const (
	MaterialDraft     = "draft"
	MaterialPublished = "published"
	MaterialArchived  = "archived"
)

// 分组可见矩阵的分组键，与 students.group_code 保持一致。
const (
	GroupClassActive = "class_active"
	GroupOneOnOne    = "one_on_one"
	GroupIntent      = "intent"
	GroupVisitor     = "visitor"
)

// MaterialGroups 是矩阵支持的四个分组，顺序用于初始化与展示。
var MaterialGroups = []string{GroupClassActive, GroupOneOnOne, GroupIntent, GroupVisitor}

// 分类 code 的取值（与迁移里的 CHECK 一致）。
const (
	CategoryClassHandout = "class_handout"
	CategoryOneOnOne     = "one_on_one"
	CategoryExam         = "exam"
	CategoryTrial        = "trial"
	CategoryEnrollment   = "enrollment"
	CategoryRecording    = "recording"
)

var (
	// ErrMaterialNotFound 资料不存在。
	ErrMaterialNotFound = errors.New("资料不存在")
	// ErrMaterialCategoryNotFound 分类不存在。
	ErrMaterialCategoryNotFound = errors.New("资料分类不存在")
	// ErrInvalidMaterial 参数不合法。
	ErrInvalidMaterial = errors.New("资料参数不合法")
	// ErrMaterialNoFiles 发布前没有附件。
	ErrMaterialNoFiles = errors.New("发布前至少添加 1 个附件")
	// ErrMaterialState 状态不允许该操作。
	ErrMaterialState = errors.New("资料当前状态不允许该操作")
)

// DefaultCategories 是蓝本表 7 的六个分类与默认可见矩阵。
// 迁移不写业务数据，改为启动时幂等补齐（与 EnsureSystem 同一思路）。
func DefaultCategories() []MaterialCategory {
	return []MaterialCategory{
		{
			Code: CategoryClassHandout, Name: "班课正式讲义", Sort: 10,
			Visibility: map[string]bool{GroupClassActive: true, GroupOneOnOne: false, GroupIntent: false, GroupVisitor: false},
		},
		{
			Code: CategoryOneOnOne, Name: "一对一定制资料", Sort: 20,
			Visibility: map[string]bool{GroupClassActive: false, GroupOneOnOne: true, GroupIntent: false, GroupVisitor: false},
		},
		{
			Code: CategoryExam, Name: "月考真题与解析", Sort: 30,
			Visibility: map[string]bool{GroupClassActive: true, GroupOneOnOne: true, GroupIntent: false, GroupVisitor: false},
		},
		{
			Code: CategoryTrial, Name: "试听讲义与体验任务", Sort: 40,
			Visibility: map[string]bool{GroupClassActive: true, GroupOneOnOne: true, GroupIntent: true, GroupVisitor: true},
		},
		{
			Code: CategoryEnrollment, Name: "招生介绍与课表", Sort: 50,
			Visibility: map[string]bool{GroupClassActive: true, GroupOneOnOne: true, GroupIntent: true, GroupVisitor: true},
		},
		{
			Code: CategoryRecording, Name: "课堂实录与录播", Sort: 60,
			Visibility: map[string]bool{GroupClassActive: false, GroupOneOnOne: false, GroupIntent: false, GroupVisitor: false},
		},
	}
}

// MaterialFile 是资料里的一个附件；file_id 是 wx.cloud.uploadFile 返回的云存储 fileID。
type MaterialFile struct {
	Name   string `json:"name"`
	FileID string `json:"file_id"`
	Size   int64  `json:"size,omitempty"`
}

// MaterialCategory 资料分类 + 分组可见矩阵。
type MaterialCategory struct {
	ID         string          `json:"id"`
	Code       string          `json:"code"`
	Name       string          `json:"name"`
	Sort       int             `json:"sort"`
	Visibility map[string]bool `json:"visibility"`
	Materials  int             `json:"materials"`
	Published  int             `json:"published"`
}

// VisibleGroups 返回矩阵里为 true 的分组，便于前端显示。
func (c *MaterialCategory) VisibleGroups() []string {
	groups := make([]string, 0, len(MaterialGroups))
	for _, g := range MaterialGroups {
		if c.Visibility[g] {
			groups = append(groups, g)
		}
	}
	return groups
}

// Material 一份资料。
type Material struct {
	ID           string
	CategoryID   string
	CategoryName string
	CategoryCode string
	Title        string
	Description  string
	Grade        string
	Status       string
	Files        []MaterialFile
	PublishedAt  *time.Time
	CreatedBy    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	// VisibleGroups 只在家长端查询时填充：哪些分组可见（用于提醒老师这份资料谁会看到）。
	VisibleGroups []string
}

// MaterialDownloadLog 下载记录（老师端排查用）。
type MaterialDownloadLog struct {
	ID           string
	MaterialID   string
	MaterialName string
	FileID       string
	GuardianID   string
	GuardianName string
	StudentID    string
	StudentName  string
	IP           string
	CreatedAt    time.Time
}

// MaterialFilter 老师端列表筛选。
type MaterialFilter struct {
	CategoryID string
	Status     string
	Query      string
}

// MaterialRepo 资料库的数据访问。
type MaterialRepo interface {
	// EnsureCategories 幂等补齐默认分类与矩阵，返回新建数量。
	EnsureCategories(ctx context.Context, defaults []MaterialCategory) (int, error)

	ListCategories(ctx context.Context) ([]MaterialCategory, error)
	CreateCategory(ctx context.Context, id, code, name string, sort int) (*MaterialCategory, error)
	UpdateCategory(ctx context.Context, id, name string, sort int) (*MaterialCategory, error)
	SetCategoryVisibility(ctx context.Context, categoryID string, visibility map[string]bool) (*MaterialCategory, error)

	ListMaterials(ctx context.Context, filter MaterialFilter) ([]Material, error)
	GetMaterial(ctx context.Context, id string) (*Material, error)
	CreateMaterial(ctx context.Context, material *Material) (*Material, error)
	UpdateMaterial(ctx context.Context, material *Material) (*Material, error)
	SetMaterialStatus(ctx context.Context, id, status string) (*Material, error)
	DeleteMaterial(ctx context.Context, id string) error

	// PortalMaterials 只返回 published 且该家长某个在读学员的分组可见的资料。
	PortalMaterials(ctx context.Context, guardianID string) ([]Material, error)
	// PortalMaterial 校验可见性；不可见时返回 ErrMaterialNotFound（不泄露存在性）。
	PortalMaterial(ctx context.Context, guardianID, materialID string) (*Material, error)
	RecordDownload(ctx context.Context, log *MaterialDownloadLog) error
	ListDownloadLogs(ctx context.Context, materialID string, limit int) ([]MaterialDownloadLog, error)
}

// MaterialUsecase 资料库用例。
type MaterialUsecase struct {
	repo MaterialRepo
}

// NewMaterialUsecase 构造。
func NewMaterialUsecase(repo MaterialRepo) *MaterialUsecase {
	return &MaterialUsecase{repo: repo}
}

// EnsureDefaults 启动时补齐默认分类（无 repo 时跳过）。
func (uc *MaterialUsecase) EnsureDefaults(ctx context.Context) (int, error) {
	if uc == nil || uc.repo == nil {
		return 0, nil
	}
	return uc.repo.EnsureCategories(ctx, DefaultCategories())
}

// Categories 分类列表。
func (uc *MaterialUsecase) Categories(ctx context.Context) ([]MaterialCategory, error) {
	return uc.repo.ListCategories(ctx)
}

// CreateCategory 新建分类（owner 维护）。
func (uc *MaterialUsecase) CreateCategory(ctx context.Context, actor *Staff, code, name string, sort int) (*MaterialCategory, error) {
	if actor == nil || actor.Role != RoleOwner {
		return nil, ErrForbidden
	}
	code = strings.TrimSpace(code)
	name = strings.TrimSpace(name)
	if code == "" || name == "" || len([]rune(name)) > 64 {
		return nil, ErrInvalidMaterial
	}
	switch code {
	case CategoryClassHandout, CategoryOneOnOne, CategoryExam, CategoryTrial, CategoryEnrollment, CategoryRecording:
	default:
		return nil, ErrInvalidMaterial
	}
	return uc.repo.CreateCategory(ctx, NewID(), code, name, sort)
}

// UpdateCategory 改分类名/排序。
func (uc *MaterialUsecase) UpdateCategory(ctx context.Context, actor *Staff, id, name string, sort int) (*MaterialCategory, error) {
	if actor == nil || actor.Role != RoleOwner {
		return nil, ErrForbidden
	}
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 64 {
		return nil, ErrInvalidMaterial
	}
	return uc.repo.UpdateCategory(ctx, id, name, sort)
}

// SetCategoryVisibility 更新分组可见矩阵，缺省分组按 false 补齐。
func (uc *MaterialUsecase) SetCategoryVisibility(ctx context.Context, actor *Staff, categoryID string, visibility map[string]bool) (*MaterialCategory, error) {
	if actor == nil || actor.Role != RoleOwner {
		return nil, ErrForbidden
	}
	normalized := make(map[string]bool, len(MaterialGroups))
	for _, g := range MaterialGroups {
		normalized[g] = visibility[g]
	}
	for g := range visibility {
		if _, ok := normalized[g]; !ok {
			return nil, ErrInvalidMaterial
		}
	}
	return uc.repo.SetCategoryVisibility(ctx, categoryID, normalized)
}

// Materials 老师端列表（含草稿与归档）。
func (uc *MaterialUsecase) Materials(ctx context.Context, filter MaterialFilter) ([]Material, error) {
	switch filter.Status {
	case "", MaterialDraft, MaterialPublished, MaterialArchived:
	default:
		return nil, ErrInvalidMaterial
	}
	return uc.repo.ListMaterials(ctx, filter)
}

// Material 单份资料。
func (uc *MaterialUsecase) Material(ctx context.Context, id string) (*Material, error) {
	if strings.TrimSpace(id) == "" {
		return nil, ErrInvalidMaterial
	}
	return uc.repo.GetMaterial(ctx, id)
}

// MaterialInput 新建/修改资料的入参。
type MaterialInput struct {
	CategoryID  string
	Title       string
	Description string
	Grade       string
	Files       []MaterialFile
}

func (in *MaterialInput) normalize() error {
	in.CategoryID = strings.TrimSpace(in.CategoryID)
	in.Title = strings.TrimSpace(in.Title)
	in.Description = strings.TrimSpace(in.Description)
	in.Grade = strings.TrimSpace(in.Grade)
	if in.CategoryID == "" {
		return ErrInvalidMaterial
	}
	if in.Title == "" || len([]rune(in.Title)) > 200 {
		return ErrInvalidMaterial
	}
	if len([]rune(in.Description)) > 1000 {
		return ErrInvalidMaterial
	}
	switch in.Grade {
	case "", "g1", "g2", "g3", "other":
	default:
		return ErrInvalidMaterial
	}
	if len(in.Files) > 9 {
		return ErrInvalidMaterial
	}
	for i := range in.Files {
		in.Files[i].Name = strings.TrimSpace(in.Files[i].Name)
		in.Files[i].FileID = strings.TrimSpace(in.Files[i].FileID)
		if in.Files[i].FileID == "" {
			return ErrInvalidMaterial
		}
		if len([]rune(in.Files[i].Name)) > 128 {
			in.Files[i].Name = string([]rune(in.Files[i].Name)[:128])
		}
	}
	return nil
}

// CreateMaterial 新建资料（默认草稿）。
func (uc *MaterialUsecase) CreateMaterial(ctx context.Context, actor *Staff, in MaterialInput) (*Material, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	material := &Material{
		ID:          NewID(),
		CategoryID:  in.CategoryID,
		Title:       in.Title,
		Description: in.Description,
		Grade:       in.Grade,
		Status:      MaterialDraft,
		Files:       in.Files,
		CreatedBy:   actor.ID,
	}
	return uc.repo.CreateMaterial(ctx, material)
}

// UpdateMaterial 改资料内容；已发布的资料也可以改，改完仍是 published。
func (uc *MaterialUsecase) UpdateMaterial(ctx context.Context, actor *Staff, id string, in MaterialInput) (*Material, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	current, err := uc.repo.GetMaterial(ctx, id)
	if err != nil {
		return nil, err
	}
	current.CategoryID = in.CategoryID
	current.Title = in.Title
	current.Description = in.Description
	current.Grade = in.Grade
	current.Files = in.Files
	return uc.repo.UpdateMaterial(ctx, current)
}

// Publish 发布：draft/archived → published，记录发布时间。
func (uc *MaterialUsecase) Publish(ctx context.Context, actor *Staff, id string) (*Material, error) {
	// 没有附件的资料发出去，家长端只能看到标题却下载不到东西，直接拒绝。
	current, err := uc.repo.GetMaterial(ctx, id)
	if err != nil {
		return nil, err
	}
	if len(current.Files) == 0 {
		return nil, ErrMaterialNoFiles
	}
	return uc.repo.SetMaterialStatus(ctx, id, MaterialPublished)
}

// Archive 归档：只能归档已发布的资料。
func (uc *MaterialUsecase) Archive(ctx context.Context, actor *Staff, id string) (*Material, error) {
	current, err := uc.repo.GetMaterial(ctx, id)
	if err != nil {
		return nil, err
	}
	if current.Status != MaterialPublished {
		return nil, ErrMaterialState
	}
	return uc.repo.SetMaterialStatus(ctx, id, MaterialArchived)
}

// DeleteMaterial 物理删除（下载日志一并删）。
func (uc *MaterialUsecase) DeleteMaterial(ctx context.Context, actor *Staff, id string) error {
	if actor == nil || actor.Role != RoleOwner {
		return ErrForbidden
	}
	return uc.repo.DeleteMaterial(ctx, id)
}

// PortalMaterials 家长端可见资料。
func (uc *MaterialUsecase) PortalMaterials(ctx context.Context, guardianID string) ([]Material, error) {
	return uc.repo.PortalMaterials(ctx, guardianID)
}

// PortalDownload 家长下载：先校验可见性，再记下载日志，最后返回文件列表。
// 返回的 file_id 供 wx.cloud.downloadFile 使用（云托管不签发临时 URL 也能下载）。
func (uc *MaterialUsecase) PortalDownload(ctx context.Context, guardianID, materialID, ip string) (*Material, error) {
	material, err := uc.repo.PortalMaterial(ctx, guardianID, materialID)
	if err != nil {
		return nil, err
	}
	for _, f := range material.Files {
		if err := uc.repo.RecordDownload(ctx, &MaterialDownloadLog{
			ID:         NewID(),
			MaterialID: material.ID,
			FileID:     f.FileID,
			GuardianID: guardianID,
			IP:         ip,
		}); err != nil {
			return nil, err
		}
	}
	return material, nil
}

// DownloadLogs 老师端下载记录。
func (uc *MaterialUsecase) DownloadLogs(ctx context.Context, materialID string, limit int) ([]MaterialDownloadLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	return uc.repo.ListDownloadLogs(ctx, materialID, limit)
}
