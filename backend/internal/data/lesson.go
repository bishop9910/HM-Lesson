package data

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/bishop9910/hm-lesson/backend/internal/biz"
)

// lessonRepo 实现课时账户与只追加流水（M4 课时台账，ADR-012）。
type lessonRepo struct {
	db *gorm.DB
}

func NewLessonRepo(db *gorm.DB) biz.LessonRepo {
	return &lessonRepo{db: db}
}

const lessonAccountSelect = `
SELECT s.id, s.name, COALESCE(s.grade, ''),
       COALESCE(a.purchased_hours, 0), COALESCE(a.used_hours, 0), COALESCE(a.balance_hours, 0),
       COALESCE(a.status, 'active'), a.valid_until, COALESCE(a.updated_at, s.created_at)
FROM students s
LEFT JOIN lesson_accounts a ON a.student_id = s.id
`

func (r *lessonRepo) ListAccounts(ctx context.Context) ([]biz.LessonAccount, error) {
	query := lessonAccountSelect + `
WHERE s.archived_at IS NULL
ORDER BY COALESCE(a.balance_hours, 0) ASC, s.name
LIMIT 500`
	rows, err := r.db.WithContext(ctx).Raw(query).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	accounts := make([]biz.LessonAccount, 0, 32)
	for rows.Next() {
		account, err := scanLessonAccount(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, *account)
	}
	return accounts, rows.Err()
}

func (r *lessonRepo) GetAccount(ctx context.Context, studentID string) (*biz.LessonAccount, error) {
	var (
		id, name, grade, status  string
		purchased, used, balance float64
		validUntil               sql.NullTime
		updatedAt                time.Time
	)
	query := lessonAccountSelect + ` WHERE s.id = ?`
	err := r.db.WithContext(ctx).Raw(query, studentID).Row().
		Scan(&id, &name, &grade, &purchased, &used, &balance, &status, &validUntil, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, biz.ErrLessonAccountNotFound
		}
		return nil, err
	}
	account := &biz.LessonAccount{
		StudentID:      id,
		StudentName:    name,
		Grade:          grade,
		PurchasedHours: purchased,
		UsedHours:      used,
		BalanceHours:   balance,
		Status:         status,
		UpdatedAt:      updatedAt,
	}
	if validUntil.Valid {
		t := validUntil.Time
		account.ValidUntil = &t
	}
	return account, nil
}

func (r *lessonRepo) ListLedger(ctx context.Context, studentID string, from, to *time.Time) ([]biz.LedgerEntry, error) {
	query := `SELECT l.id, l.student_id, l.type, l.delta_hours, l.balance_after,
       COALESCE(l.ref_type, ''), COALESCE(l.ref_id, ''), l.reason, l.operator_id,
       COALESCE(st.name, ''), l.occurred_at
FROM lesson_ledger l
LEFT JOIN staff st ON st.id = l.operator_id
WHERE 1 = 1`
	args := []any{}
	if studentID != "" {
		query += ` AND l.student_id = ?`
		args = append(args, studentID)
	}
	if from != nil {
		query += ` AND l.occurred_at >= ?`
		args = append(args, from.UTC())
	}
	if to != nil {
		query += ` AND l.occurred_at < ?`
		args = append(args, to.UTC())
	}
	query += ` ORDER BY l.occurred_at DESC LIMIT 300`

	rows, err := r.db.WithContext(ctx).Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]biz.LedgerEntry, 0, 32)
	for rows.Next() {
		var e biz.LedgerEntry
		if err := rows.Scan(&e.ID, &e.StudentID, &e.Type, &e.DeltaHours, &e.BalanceAfter,
			&e.RefType, &e.RefID, &e.Reason, &e.OperatorID, &e.OperatorName, &e.OccurredAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ApplyLedger 锁定账户行 → 追加流水 → 更新缓存余额（同一事务）。
func (r *lessonRepo) ApplyLedger(ctx context.Context, entry *biz.LedgerEntry) (*biz.LessonAccount, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 账户不存在时先补一条 0 余额记录（充值/调整允许首个流水创建账户）。
		if err := tx.Exec(`INSERT IGNORE INTO lesson_accounts (student_id, purchased_hours, used_hours, balance_hours, status)
			VALUES (?, 0, 0, 0, 'active')`, entry.StudentID).Error; err != nil {
			return mapFKError(err)
		}
		var purchased, used, balance float64
		var status string
		var validUntil sql.NullTime
		row := tx.Raw(`SELECT purchased_hours, used_hours, balance_hours, status, valid_until
			FROM lesson_accounts WHERE student_id = ? FOR UPDATE`, entry.StudentID).Row()
		if err := row.Scan(&purchased, &used, &balance, &status, &validUntil); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return biz.ErrLessonAccountNotFound
			}
			return err
		}
		delta := roundHours(entry.DeltaHours)
		balanceAfter := roundHours(balance + delta)
		if balanceAfter < 0 {
			return biz.ErrInsufficientHours
		}
		// 不变量：balance = purchased - used（purchase 加购买；consume 加消耗；adjust 按符号归入一侧）。
		switch entry.Type {
		case biz.LedgerPurchase:
			purchased = roundHours(purchased + delta)
		case biz.LedgerConsume:
			used = roundHours(used - delta)
		case biz.LedgerAdjust:
			if delta > 0 {
				purchased = roundHours(purchased + delta)
			} else {
				used = roundHours(used - delta)
			}
		}
		newStatus := status
		if balanceAfter <= 0 {
			newStatus = biz.LessonExhausted
		} else if status != biz.LessonExpired {
			newStatus = biz.LessonActive
		}
		entry.ID = biz.NewID()
		entry.BalanceAfter = balanceAfter
		entry.OccurredAt = time.Now().UTC()
		if err := tx.Exec(`INSERT INTO lesson_ledger (id, student_id, type, delta_hours, balance_after, ref_type, ref_id, reason, operator_id, occurred_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			entry.ID, entry.StudentID, entry.Type, delta, balanceAfter, entry.RefType,
			nullString(entry.RefID), entry.Reason, entry.OperatorID, entry.OccurredAt).Error; err != nil {
			return err
		}
		return tx.Exec(`UPDATE lesson_accounts SET purchased_hours = ?, used_hours = ?, balance_hours = ?,
			status = ?, updated_at = NOW(3) WHERE student_id = ?`,
			purchased, used, balanceAfter, newStatus, entry.StudentID).Error
	})
	if err != nil {
		return nil, err
	}
	return r.GetAccount(ctx, entry.StudentID)
}

func scanLessonAccount(rows interface{ Scan(...any) error }) (*biz.LessonAccount, error) {
	var (
		id, name, grade, status  string
		purchased, used, balance float64
		validUntil               sql.NullTime
		updatedAt                time.Time
	)
	if err := rows.Scan(&id, &name, &grade, &purchased, &used, &balance, &status, &validUntil, &updatedAt); err != nil {
		return nil, err
	}
	account := &biz.LessonAccount{
		StudentID:      id,
		StudentName:    name,
		Grade:          grade,
		PurchasedHours: purchased,
		UsedHours:      used,
		BalanceHours:   balance,
		Status:         status,
		UpdatedAt:      updatedAt,
	}
	if validUntil.Valid {
		t := validUntil.Time
		account.ValidUntil = &t
	}
	return account, nil
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// mapFKError 把外键失败（学员不存在）转成领域错误。
func mapFKError(err error) error {
	if err == nil {
		return nil
	}
	if isFKViolation(err) {
		return biz.ErrLessonAccountNotFound
	}
	return err
}
