package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/bishop9910/hm-lesson/backend/internal/biz"
)

// materialRepo 资料库仓储。
type materialRepo struct {
	db *gorm.DB
}

// NewMaterialRepo 构造资料库仓储。
func NewMaterialRepo(db *gorm.DB) biz.MaterialRepo {
	return &materialRepo{db: db}
}

const materialSelect = `
SELECT m.id, m.category_id, m.title, m.description, COALESCE(m.grade,'') AS grade,
       m.status, m.file_ids, m.published_at, m.created_by, m.created_at, m.updated_at,
       c.name AS category_name, c.code AS category_code
FROM materials m
JOIN material_categories c ON c.id = m.category_id`

type materialRow struct {
	ID           string
	CategoryID   string
	Title        string
	Description  string
	Grade        string
	Status       string
	FileIDs      []byte
	PublishedAt  sql.NullTime
	CreatedBy    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	CategoryName string
	CategoryCode string
}

func (r materialRow) toBiz() biz.Material {
	material := biz.Material{
		ID:           r.ID,
		CategoryID:   r.CategoryID,
		Title:        r.Title,
		Description:  r.Description,
		Grade:        r.Grade,
		Status:       r.Status,
		CreatedBy:    r.CreatedBy,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
		CategoryName: r.CategoryName,
		CategoryCode: r.CategoryCode,
	}
	if r.PublishedAt.Valid {
		t := r.PublishedAt.Time
		material.PublishedAt = &t
	}
	material.Files = decodeMaterialFiles(r.FileIDs)
	return material
}

func decodeMaterialFiles(raw []byte) []biz.MaterialFile {
	if len(raw) == 0 {
		return nil
	}
	var files []biz.MaterialFile
	if err := json.Unmarshal(raw, &files); err != nil {
		return nil
	}
	return files
}

func encodeMaterialFiles(files []biz.MaterialFile) any {
	if len(files) == 0 {
		return nil
	}
	payload, err := json.Marshal(files)
	if err != nil {
		return nil
	}
	// MySQL 5.7 的 JSON 列不接受 binary 字符集，必须传字符串。
	return string(payload)
}

func (r *materialRepo) EnsureCategories(ctx context.Context, defaults []biz.MaterialCategory) (int, error) {
	created := 0
	for _, item := range defaults {
		result := r.db.WithContext(ctx).Exec(
			`INSERT IGNORE INTO material_categories (id, code, name, sort, created_at, updated_at)
			 VALUES (?, ?, ?, ?, NOW(3), NOW(3))`,
			biz.NewID(), item.Code, item.Name, item.Sort,
		)
		if result.Error != nil {
			return created, result.Error
		}
		if result.RowsAffected > 0 {
			created++
		}
		var categoryID string
		if err := r.db.WithContext(ctx).Raw(
			`SELECT id FROM material_categories WHERE code = ?`, item.Code,
		).Scan(&categoryID).Error; err != nil {
			return created, err
		}
		if categoryID == "" {
			continue
		}
		for _, group := range biz.MaterialGroups {
			visible := 0
			if item.Visibility[group] {
				visible = 1
			}
			if err := r.db.WithContext(ctx).Exec(
				`INSERT IGNORE INTO material_visibility (category_id, group_code, visible) VALUES (?, ?, ?)`,
				categoryID, group, visible,
			).Error; err != nil {
				return created, err
			}
		}
	}
	return created, nil
}

type categoryRow struct {
	ID         string
	Code       string
	Name       string
	Sort       int
	Materials  int
	Published  int
	Visibility sql.NullString
}

func (r *materialRepo) ListCategories(ctx context.Context) ([]biz.MaterialCategory, error) {
	var rows []categoryRow
	err := r.db.WithContext(ctx).Raw(`
SELECT c.id, c.code, c.name, c.sort,
       (SELECT COUNT(*) FROM materials m WHERE m.category_id = c.id) AS materials,
       (SELECT COUNT(*) FROM materials m WHERE m.category_id = c.id AND m.status = 'published') AS published,
       (SELECT GROUP_CONCAT(CONCAT(v.group_code, ':', v.visible)) FROM material_visibility v WHERE v.category_id = c.id) AS visibility
FROM material_categories c
ORDER BY c.sort, c.code`).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	categories := make([]biz.MaterialCategory, 0, len(rows))
	for _, row := range rows {
		categories = append(categories, biz.MaterialCategory{
			ID:         row.ID,
			Code:       row.Code,
			Name:       row.Name,
			Sort:       row.Sort,
			Materials:  row.Materials,
			Published:  row.Published,
			Visibility: parseVisibility(row.Visibility.String),
		})
	}
	return categories, nil
}

func parseVisibility(raw string) map[string]bool {
	visibility := make(map[string]bool, len(biz.MaterialGroups))
	for _, group := range biz.MaterialGroups {
		visibility[group] = false
	}
	for _, pair := range strings.Split(raw, ",") {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			continue
		}
		visibility[parts[0]] = parts[1] == "1"
	}
	return visibility
}

func (r *materialRepo) categoryByID(ctx context.Context, id string) (*biz.MaterialCategory, error) {
	var rows []categoryRow
	err := r.db.WithContext(ctx).Raw(`
SELECT c.id, c.code, c.name, c.sort, 0 AS materials, 0 AS published,
       (SELECT GROUP_CONCAT(CONCAT(v.group_code, ':', v.visible)) FROM material_visibility v WHERE v.category_id = c.id) AS visibility
FROM material_categories c WHERE c.id = ?`, id).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, biz.ErrMaterialCategoryNotFound
	}
	row := rows[0]
	return &biz.MaterialCategory{
		ID:         row.ID,
		Code:       row.Code,
		Name:       row.Name,
		Sort:       row.Sort,
		Visibility: parseVisibility(row.Visibility.String),
	}, nil
}

func (r *materialRepo) CreateCategory(ctx context.Context, id, code, name string, sort int) (*biz.MaterialCategory, error) {
	err := r.db.WithContext(ctx).Exec(
		`INSERT INTO material_categories (id, code, name, sort, created_at, updated_at)
		 VALUES (?, ?, ?, ?, NOW(3), NOW(3))`,
		id, code, name, sort,
	).Error
	if err != nil {
		if isDuplicate(err) {
			return nil, biz.ErrInvalidMaterial
		}
		return nil, err
	}
	for _, group := range biz.MaterialGroups {
		visible := 0
		if code == biz.CategoryTrial || code == biz.CategoryEnrollment {
			visible = 1
		}
		if err := r.db.WithContext(ctx).Exec(
			`INSERT IGNORE INTO material_visibility (category_id, group_code, visible) VALUES (?, ?, ?)`,
			id, group, visible,
		).Error; err != nil {
			return nil, err
		}
	}
	return r.categoryByID(ctx, id)
}

func (r *materialRepo) UpdateCategory(ctx context.Context, id, name string, sort int) (*biz.MaterialCategory, error) {
	result := r.db.WithContext(ctx).Exec(
		`UPDATE material_categories SET name = ?, sort = ?, updated_at = NOW(3) WHERE id = ?`,
		name, sort, id,
	)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		if _, err := r.categoryByID(ctx, id); err != nil {
			return nil, err
		}
	}
	return r.categoryByID(ctx, id)
}

func (r *materialRepo) SetCategoryVisibility(ctx context.Context, categoryID string, visibility map[string]bool) (*biz.MaterialCategory, error) {
	if _, err := r.categoryByID(ctx, categoryID); err != nil {
		return nil, err
	}
	for _, group := range biz.MaterialGroups {
		visible := 0
		if visibility[group] {
			visible = 1
		}
		if err := r.db.WithContext(ctx).Exec(
			`INSERT INTO material_visibility (category_id, group_code, visible) VALUES (?, ?, ?)
			 ON DUPLICATE KEY UPDATE visible = VALUES(visible)`,
			categoryID, group, visible,
		).Error; err != nil {
			return nil, err
		}
	}
	return r.categoryByID(ctx, categoryID)
}

func (r *materialRepo) ListMaterials(ctx context.Context, filter biz.MaterialFilter) ([]biz.Material, error) {
	query := materialSelect + ` WHERE 1 = 1`
	args := make([]any, 0, 3)
	if filter.CategoryID != "" {
		query += ` AND m.category_id = ?`
		args = append(args, filter.CategoryID)
	}
	if filter.Status != "" {
		query += ` AND m.status = ?`
		args = append(args, filter.Status)
	}
	if q := strings.TrimSpace(filter.Query); q != "" {
		query += ` AND (m.title LIKE ? OR m.description LIKE ?)`
		like := "%" + q + "%"
		args = append(args, like, like)
	}
	query += ` ORDER BY c.sort, m.published_at DESC, m.created_at DESC LIMIT 300`
	var rows []materialRow
	if err := r.db.WithContext(ctx).Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	materials := make([]biz.Material, 0, len(rows))
	for _, row := range rows {
		materials = append(materials, row.toBiz())
	}
	return materials, nil
}

func (r *materialRepo) GetMaterial(ctx context.Context, id string) (*biz.Material, error) {
	var rows []materialRow
	if err := r.db.WithContext(ctx).Raw(materialSelect+` WHERE m.id = ?`, id).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, biz.ErrMaterialNotFound
	}
	material := rows[0].toBiz()
	return &material, nil
}

func (r *materialRepo) CreateMaterial(ctx context.Context, material *biz.Material) (*biz.Material, error) {
	err := r.db.WithContext(ctx).Exec(
		`INSERT INTO materials (id, category_id, title, description, grade, status, file_ids, created_by, created_at, updated_at)
		 VALUES (?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, NOW(3), NOW(3))`,
		material.ID, material.CategoryID, material.Title, material.Description, material.Grade,
		material.Status, encodeMaterialFiles(material.Files), material.CreatedBy,
	).Error
	if err != nil {
		if isFKViolation(err) {
			return nil, biz.ErrMaterialCategoryNotFound
		}
		return nil, err
	}
	return r.GetMaterial(ctx, material.ID)
}

func (r *materialRepo) UpdateMaterial(ctx context.Context, material *biz.Material) (*biz.Material, error) {
	result := r.db.WithContext(ctx).Exec(
		`UPDATE materials SET category_id = ?, title = ?, description = ?, grade = NULLIF(?, ''),
		        file_ids = ?, updated_at = NOW(3)
		 WHERE id = ?`,
		material.CategoryID, material.Title, material.Description, material.Grade,
		encodeMaterialFiles(material.Files), material.ID,
	)
	if result.Error != nil {
		if isFKViolation(result.Error) {
			return nil, biz.ErrMaterialCategoryNotFound
		}
		return nil, result.Error
	}
	return r.GetMaterial(ctx, material.ID)
}

func (r *materialRepo) SetMaterialStatus(ctx context.Context, id, status string) (*biz.Material, error) {
	var result *gorm.DB
	if status == biz.MaterialPublished {
		result = r.db.WithContext(ctx).Exec(
			`UPDATE materials SET status = ?, published_at = NOW(3), updated_at = NOW(3) WHERE id = ?`,
			status, id,
		)
	} else {
		result = r.db.WithContext(ctx).Exec(
			`UPDATE materials SET status = ?, updated_at = NOW(3) WHERE id = ?`,
			status, id,
		)
	}
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		if _, err := r.GetMaterial(ctx, id); err != nil {
			return nil, err
		}
	}
	return r.GetMaterial(ctx, id)
}

func (r *materialRepo) DeleteMaterial(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int
		if err := tx.Raw(`SELECT COUNT(*) FROM materials WHERE id = ?`, id).Scan(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return biz.ErrMaterialNotFound
		}
		if err := tx.Exec(`DELETE FROM material_download_logs WHERE material_id = ?`, id).Error; err != nil {
			return err
		}
		return tx.Exec(`DELETE FROM materials WHERE id = ?`, id).Error
	})
}

// portalMaterialSelect 只保留「已发布 + 分类矩阵允许 + 该家长有在读学员属于该分组」的资料。
const portalMaterialSelect = `
SELECT m.id, m.category_id, m.title, m.description, COALESCE(m.grade,'') AS grade,
       m.status, m.file_ids, m.published_at, m.created_by, m.created_at, m.updated_at,
       c.name AS category_name, c.code AS category_code,
       GROUP_CONCAT(DISTINCT v.group_code) AS visible_groups
FROM materials m
JOIN material_categories c ON c.id = m.category_id
JOIN material_visibility v ON v.category_id = m.category_id AND v.visible = 1
JOIN students s ON s.group_code = v.group_code AND s.status = 'active' AND s.archived_at IS NULL
JOIN student_guardians sg ON sg.student_id = s.id
JOIN guardians g ON g.id = sg.guardian_id AND g.status = 'active'
WHERE m.status = 'published' AND sg.guardian_id = ?`

type portalMaterialRow struct {
	ID            string
	CategoryID    string
	Title         string
	Description   string
	Grade         string
	Status        string
	FileIDs       []byte
	PublishedAt   sql.NullTime
	CreatedBy     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	CategoryName  string
	CategoryCode  string
	VisibleGroups sql.NullString
}

func (r portalMaterialRow) toBiz() biz.Material {
	material := biz.Material{
		ID:            r.ID,
		CategoryID:    r.CategoryID,
		Title:         r.Title,
		Description:   r.Description,
		Grade:         r.Grade,
		Status:        r.Status,
		CreatedBy:     r.CreatedBy,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
		CategoryName:  r.CategoryName,
		CategoryCode:  r.CategoryCode,
		VisibleGroups: splitGroups(r.VisibleGroups.String),
	}
	if r.PublishedAt.Valid {
		t := r.PublishedAt.Time
		material.PublishedAt = &t
	}
	material.Files = decodeMaterialFiles(r.FileIDs)
	return material
}

func (r *materialRepo) PortalMaterials(ctx context.Context, guardianID string) ([]biz.Material, error) {
	var rows []portalMaterialRow
	err := r.db.WithContext(ctx).Raw(portalMaterialSelect+`
 GROUP BY m.id, c.sort, c.name, c.code
 ORDER BY c.sort, m.published_at DESC`, guardianID).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	materials := make([]biz.Material, 0, len(rows))
	for _, row := range rows {
		materials = append(materials, row.toBiz())
	}
	return materials, nil
}

func splitGroups(raw string) []string {
	if raw == "" {
		return nil
	}
	return strings.Split(raw, ",")
}

func (r *materialRepo) PortalMaterial(ctx context.Context, guardianID, materialID string) (*biz.Material, error) {
	var rows []portalMaterialRow
	err := r.db.WithContext(ctx).Raw(portalMaterialSelect+` AND m.id = ?
 GROUP BY m.id, c.sort, c.name, c.code`, guardianID, materialID).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		// 不可见与不存在返回同一个错误，避免家长枚举资料。
		return nil, biz.ErrMaterialNotFound
	}
	material := rows[0].toBiz()
	return &material, nil
}

func (r *materialRepo) RecordDownload(ctx context.Context, log *biz.MaterialDownloadLog) error {
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO material_download_logs (id, material_id, file_id, guardian_id, student_id, ip, created_at)
		 VALUES (?, ?, ?, ?, NULLIF(?, ''), ?, NOW(3))`,
		log.ID, log.MaterialID, log.FileID, log.GuardianID, log.StudentID, log.IP,
	).Error
}

func (r *materialRepo) ListDownloadLogs(ctx context.Context, materialID string, limit int) ([]biz.MaterialDownloadLog, error) {
	query := `
SELECT d.id, d.material_id, m.title AS material_name, d.file_id, d.guardian_id,
       COALESCE(g.name,'') AS guardian_name, COALESCE(g.phone,'') AS guardian_phone,
       COALESCE(s.name,'') AS student_name, d.ip, d.created_at
FROM material_download_logs d
JOIN materials m ON m.id = d.material_id
LEFT JOIN guardians g ON g.id = d.guardian_id
LEFT JOIN students s ON s.id = d.student_id
WHERE 1 = 1`
	args := make([]any, 0, 2)
	if materialID != "" {
		query += ` AND d.material_id = ?`
		args = append(args, materialID)
	}
	query += ` ORDER BY d.created_at DESC LIMIT ?`
	args = append(args, limit)

	var rows []struct {
		ID            string
		MaterialID    string
		MaterialName  string
		FileID        string
		GuardianID    string
		GuardianName  string
		GuardianPhone string
		StudentName   string
		IP            string
		CreatedAt     time.Time
	}
	if err := r.db.WithContext(ctx).Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	logs := make([]biz.MaterialDownloadLog, 0, len(rows))
	for _, row := range rows {
		name := row.GuardianName
		if name == "" {
			name = row.GuardianPhone
		}
		logs = append(logs, biz.MaterialDownloadLog{
			ID:           row.ID,
			MaterialID:   row.MaterialID,
			MaterialName: row.MaterialName,
			FileID:       row.FileID,
			GuardianID:   row.GuardianID,
			GuardianName: name,
			StudentName:  row.StudentName,
			IP:           row.IP,
			CreatedAt:    row.CreatedAt,
		})
	}
	return logs, nil
}
