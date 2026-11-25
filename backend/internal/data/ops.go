package data

import (
	"context"
	"database/sql"
	"time"

	"github.com/bishop9910/hm-lesson/backend/internal/biz"
	"gorm.io/gorm"
)

// OpsRepo 提供内部系统账号的运维查询能力。
type OpsRepo struct {
	db *gorm.DB
}

// NewOpsRepo 构造运维仓储。
func NewOpsRepo(db *gorm.DB) *OpsRepo {
	return &OpsRepo{db: db}
}

// auditRow 是 audit_logs 的查询行。
type auditRow struct {
	ID            string         `gorm:"column:id"`
	ActorType     string         `gorm:"column:actor_type"`
	ActorID       string         `gorm:"column:actor_id"`
	ActorName     sql.NullString `gorm:"column:actor_name"`
	Action        string         `gorm:"column:action"`
	TargetType    string         `gorm:"column:target_type"`
	TargetID      string         `gorm:"column:target_id"`
	BeforeSummary sql.NullString `gorm:"column:before_summary"`
	AfterSummary  sql.NullString `gorm:"column:after_summary"`
	CreatedAt     time.Time      `gorm:"column:created_at"`
}

// ListAudit 返回最近的审计记录。
func (r *OpsRepo) ListAudit(ctx context.Context, action string, limit int) ([]biz.AuditEntry, error) {
	sqlText := `SELECT a.id, a.actor_type, a.actor_id, s.name AS actor_name, a.action,
		a.target_type, a.target_id, a.before_summary, a.after_summary, a.created_at
		FROM audit_logs a
		LEFT JOIN staff s ON s.id = a.actor_id AND a.actor_type = 'staff'`
	args := []any{}
	if action != "" {
		sqlText += " WHERE a.action = ?"
		args = append(args, action)
	}
	sqlText += " ORDER BY a.created_at DESC LIMIT ?"
	args = append(args, limit)

	var rows []auditRow
	if err := r.db.WithContext(ctx).Raw(sqlText, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]biz.AuditEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, biz.AuditEntry{
			ID:            row.ID,
			ActorType:     row.ActorType,
			ActorID:       row.ActorID,
			ActorName:     row.ActorName.String,
			Action:        row.Action,
			TargetType:    row.TargetType,
			TargetID:      row.TargetID,
			BeforeSummary: row.BeforeSummary.String,
			AfterSummary:  row.AfterSummary.String,
			CreatedAt:     row.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	return out, nil
}

// staffPermRow 是权限视图的查询行。
type staffPermRow struct {
	ID     string `gorm:"column:id"`
	Phone  string `gorm:"column:phone"`
	Name   string `gorm:"column:name"`
	Role   string `gorm:"column:role"`
	Status string `gorm:"column:status"`
	Hidden bool   `gorm:"column:hidden"`
}

// ListStaffPermissions 返回全部成员（含内部系统账号）的角色与状态。
func (r *OpsRepo) ListStaffPermissions(ctx context.Context) ([]biz.StaffPermission, error) {
	var rows []staffPermRow
	err := r.db.WithContext(ctx).Raw(
		`SELECT id, phone, name, role, status, hidden FROM staff
		 WHERE deleted_at IS NULL
		 ORDER BY hidden ASC, FIELD(role, 'owner', 'assistant'), created_at ASC`,
	).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]biz.StaffPermission, 0, len(rows))
	for _, row := range rows {
		out = append(out, biz.StaffPermission{
			ID:     row.ID,
			Phone:  row.Phone,
			Name:   row.Name,
			Role:   row.Role,
			Status: row.Status,
			Hidden: row.Hidden,
		})
	}
	return out, nil
}

// RecordAudit 写入一条操作记录（before/after 摘要为 JSON 列）。
func (r *OpsRepo) RecordAudit(ctx context.Context, entry biz.AuditEntry) error {
	before := nullSummary(entry.BeforeSummary)
	after := nullSummary(entry.AfterSummary)
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO audit_logs (id, actor_type, actor_id, action, target_type, target_id,
			before_summary, after_summary, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW(3))`,
		entry.ID, entry.ActorType, entry.ActorID, entry.Action, entry.TargetType, entry.TargetID,
		before, after,
	).Error
}

// nullSummary 把空摘要转成 NULL，非空则按 JSON 字符串写入。
func nullSummary(raw string) any {
	if raw == "" {
		return nil
	}
	return raw
}

// QueryReadOnly 执行只读语句并返回列名与行数据。
func (r *OpsRepo) QueryReadOnly(
	ctx context.Context, statement string, maxRows int,
) ([]string, [][]string, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	rows, err := r.db.WithContext(ctx).Raw(statement).Rows()
	if err != nil {
		return nil, nil, false, err
	}
	defer func() {
		_ = rows.Close()
	}()

	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, false, err
	}
	result := make([][]string, 0, maxRows)
	truncated := false
	for rows.Next() {
		if len(result) >= maxRows {
			truncated = true
			break
		}
		values := make([]sql.RawBytes, len(columns))
		targets := make([]any, len(columns))
		for i := range values {
			targets[i] = &values[i]
		}
		if err := rows.Scan(targets...); err != nil {
			return nil, nil, false, err
		}
		line := make([]string, len(columns))
		for i, value := range values {
			if value == nil {
				line[i] = ""
				continue
			}
			line[i] = string(value)
		}
		result = append(result, line)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, false, err
	}
	return columns, result, truncated, nil
}
