package data

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/bishop9910/hm-lesson/backend/internal/biz"
)

type staffRow struct {
	ID           string    `gorm:"column:id"`
	Name         string    `gorm:"column:name"`
	Phone        string    `gorm:"column:phone"`
	PasswordHash string    `gorm:"column:password_hash"`
	Role         string    `gorm:"column:role"`
	Status       string    `gorm:"column:status"`
	CreatedBy    *string   `gorm:"column:created_by"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

func (staffRow) TableName() string { return "staff" }

type StaffRepo struct {
	db *gorm.DB
}

func NewStaffRepo(db *gorm.DB) *StaffRepo { return &StaffRepo{db: db} }

func (r *StaffRepo) Count(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Raw("SELECT COUNT(*) FROM staff WHERE deleted_at IS NULL").Scan(&n).Error
	return n, err
}

func (r *StaffRepo) Create(ctx context.Context, s *biz.Staff) error {
	var createdBy any
	if s.CreatedBy != "" {
		createdBy = s.CreatedBy
	}
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO staff (id, name, phone, password_hash, role, status, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.Name, s.Phone, s.PasswordHash, s.Role, s.Status, createdBy,
	).Error
}

func (r *StaffRepo) Update(ctx context.Context, s *biz.Staff) error {
	return r.db.WithContext(ctx).Exec(
		`UPDATE staff SET name = ?, role = ?, status = ?, password_hash = ? WHERE id = ? AND deleted_at IS NULL`,
		s.Name, s.Role, s.Status, s.PasswordHash, s.ID,
	).Error
}

// UpdatePassword 自助改密只写这一列，避免与成员管理（Update）互相覆盖字段。
func (r *StaffRepo) UpdatePassword(ctx context.Context, staffID, passwordHash string) error {
	res := r.db.WithContext(ctx).Exec(
		"UPDATE staff SET password_hash = ?, updated_at = NOW(3) WHERE id = ? AND deleted_at IS NULL",
		passwordHash, staffID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return biz.ErrStaffNotFound
	}
	return nil
}

func (r *StaffRepo) ByPhone(ctx context.Context, phone string) (*biz.Staff, error) {
	var row staffRow
	err := r.db.WithContext(ctx).
		Where("phone = ? AND deleted_at IS NULL", phone).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, biz.ErrStaffNotFound
	}
	if err != nil {
		return nil, err
	}
	return toStaff(row), nil
}

func (r *StaffRepo) ByID(ctx context.Context, id string) (*biz.Staff, error) {
	var row staffRow
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, biz.ErrStaffNotFound
	}
	if err != nil {
		return nil, err
	}
	return toStaff(row), nil
}

func (r *StaffRepo) List(ctx context.Context) ([]*biz.Staff, error) {
	var rows []staffRow
	if err := r.db.WithContext(ctx).
		Where("deleted_at IS NULL").
		Order("created_at ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*biz.Staff, 0, len(rows))
	for _, row := range rows {
		out = append(out, toStaff(row))
	}
	return out, nil
}

func toStaff(row staffRow) *biz.Staff {
	createdBy := ""
	if row.CreatedBy != nil {
		createdBy = *row.CreatedBy
	}
	return &biz.Staff{
		ID:           row.ID,
		Name:         row.Name,
		Phone:        row.Phone,
		PasswordHash: row.PasswordHash,
		Role:         row.Role,
		Status:       row.Status,
		CreatedBy:    createdBy,
	}
}
