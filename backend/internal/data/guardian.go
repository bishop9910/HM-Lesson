package data

import (
	"context"
	"database/sql"
	"errors"

	"gorm.io/gorm"

	"github.com/bishop9910/hm-lesson/backend/internal/biz"
)

type guardianRepo struct {
	db *gorm.DB
}

func NewGuardianRepo(db *gorm.DB) biz.BindRepo {
	return &guardianRepo{db: db}
}

// RequestByPhone 返回已有家长状态；没有则创建 pending 记录。
// 真正的学员关联与审批在 M2 完成，这里只把申请落库，避免前端假提示。
func (r *guardianRepo) RequestByPhone(ctx context.Context, phone string) (string, error) {
	var foundID, foundStatus string
	err := r.db.WithContext(ctx).
		Raw(`SELECT id, status FROM guardians WHERE phone = ? ORDER BY FIELD(status,'active','pending','rejected','disabled'), created_at LIMIT 1`, phone).
		Row().
		Scan(&foundID, &foundStatus)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	if foundID != "" {
		return foundStatus, nil
	}
	err = r.db.WithContext(ctx).Exec(
		`INSERT INTO guardians (id, phone, status) VALUES (?, ?, 'pending')`,
		biz.NewID(), phone,
	).Error
	if err != nil {
		return "", err
	}
	return "pending", nil
}
