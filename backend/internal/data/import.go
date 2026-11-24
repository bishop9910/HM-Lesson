package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/bishop9910/hm-lesson/backend/internal/biz"
	"github.com/bishop9910/hm-lesson/backend/pkg/xlsximport"
)

type importRepo struct {
	db *gorm.DB
}

func NewImportRepo(db *gorm.DB) biz.ImportRepo { return &importRepo{db: db} }

type importBatchRow struct {
	ID           string
	Kind         string
	Status       string
	Mode         string
	TotalRows    int
	CreatedRows  int
	SkippedRows  int
	ErrorCount   int
	ErrorReport  []byte
	RowsJSON     sql.NullString
	CreatedBy    string
	CreatedAt    time.Time
	CommittedAt  sql.NullTime
	RolledBackAt sql.NullTime
}

func (r importBatchRow) toBiz() *biz.ImportBatch {
	b := &biz.ImportBatch{
		ID: r.ID, Kind: r.Kind, Status: r.Status, Mode: r.Mode,
		TotalRows: r.TotalRows, CreatedRows: r.CreatedRows, SkippedRows: r.SkippedRows,
		ErrorCount: r.ErrorCount, CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt,
		Issues: []xlsximport.Issue{},
	}
	if len(r.ErrorReport) > 0 {
		_ = json.Unmarshal(r.ErrorReport, &b.Issues)
	}
	if r.CommittedAt.Valid {
		t := r.CommittedAt.Time
		b.CommittedAt = &t
	}
	if r.RolledBackAt.Valid {
		t := r.RolledBackAt.Time
		b.RolledBackAt = &t
	}
	return b
}

// xlsxIssue 已由 pkg/xlsximport.Issue 取代。

const importBatchColumns = "id,kind,status,mode,total_rows,created_rows,skipped_rows,error_count,error_report,rows_json,created_by,created_at,committed_at,rolled_back_at"

func (r *importRepo) CreateImportBatch(ctx context.Context, batch *biz.ImportBatch, rows []biz.ImportRow) error {
	rowsJSON, err := json.Marshal(rows)
	if err != nil {
		return err
	}
	issuesJSON, _ := json.Marshal(batch.Issues)
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO import_batches (id,kind,status,mode,total_rows,created_rows,skipped_rows,error_count,error_report,rows_json,created_by)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		batch.ID, batch.Kind, batch.Status, batch.Mode, batch.TotalRows, 0, 0, batch.ErrorCount, string(issuesJSON), string(rowsJSON), batch.CreatedBy,
	).Error
}

func (r *importRepo) GetImportBatch(ctx context.Context, id string) (*biz.ImportBatch, []biz.ImportRow, error) {
	var row importBatchRow
	err := r.db.WithContext(ctx).Raw("SELECT "+importBatchColumns+" FROM import_batches WHERE id = ?", id).Scan(&row).Error
	if err != nil {
		return nil, nil, err
	}
	if row.ID == "" {
		return nil, nil, biz.ErrImportNotFound
	}
	var rows []biz.ImportRow
	if row.RowsJSON.Valid && row.RowsJSON.String != "" {
		_ = json.Unmarshal([]byte(row.RowsJSON.String), &rows)
	}
	return row.toBiz(), rows, nil
}

func (r *importRepo) StudentExistsByNamePhone(ctx context.Context, name, phone string) (bool, error) {
	var count int64
	var err error
	if phone == "" {
		err = r.db.WithContext(ctx).Raw(
			"SELECT COUNT(*) FROM students WHERE name = ? AND archived_at IS NULL", name,
		).Scan(&count).Error
	} else {
		err = r.db.WithContext(ctx).Raw(
			`SELECT COUNT(DISTINCT s.id) FROM students s
			 LEFT JOIN student_guardians sg ON sg.student_id = s.id
			 LEFT JOIN guardians g ON g.id = sg.guardian_id
			 WHERE s.name = ? AND s.archived_at IS NULL AND g.phone = ?`,
			name, phone,
		).Scan(&count).Error
	}
	return count > 0, err
}

func (r *importRepo) CommitImportBatch(ctx context.Context, id, staffID string) (*biz.ImportBatch, error) {
	batch, rows, err := r.GetImportBatch(ctx, id)
	if err != nil {
		return nil, err
	}
	if batch.Status != "preview" {
		return nil, biz.ErrImportState
	}
	if batch.ErrorCount > 0 {
		return nil, biz.ErrImportHasErrors
	}

	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		created, skipped := 0, 0
		for _, row := range rows {
			exists, err := studentExistsTx(tx, row.Name, row.Phone)
			if err != nil {
				return err
			}
			if exists {
				skipped++
				continue
			}
			studentID := biz.NewID()
			grade := row.Grade
			if grade == "" {
				grade = "未填"
			}
			tags := tagsJSON(row.Tags)
			var next any
			if row.NextFollowUp != "" {
				if t, err := time.ParseInLocation("2006-01-02", row.NextFollowUp, time.Local); err == nil {
					next = t
				}
			}
			if err := tx.Exec(
				`INSERT INTO students (id,name,grade,school,group_code,status,tags,next_follow_up_at,note,created_by,import_batch_id)
				 VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
				studentID, row.Name, grade, row.School, row.GroupCode, row.Status, tags, next, row.Note, staffID, id,
			).Error; err != nil {
				return err
			}
			if row.Phone != "" {
				guardianID, err := ensureGuardianTx(tx, row, staffID, id)
				if err != nil {
					return err
				}
				if err := tx.Exec(
					`INSERT IGNORE INTO student_guardians (id,student_id,guardian_id,relation,is_primary)
					 VALUES (?,?,?,?,?)`,
					biz.NewID(), studentID, guardianID, "parent", 1,
				).Error; err != nil {
					return err
				}
			}
			created++
		}
		return tx.Exec(
			`UPDATE import_batches SET status='committed', created_rows=?, skipped_rows=?, committed_at=NOW(3) WHERE id=?`,
			created, skipped, id,
		).Error
	})
	if err != nil {
		return nil, err
	}
	batch, _, err = r.GetImportBatch(ctx, id)
	return batch, err
}

func studentExistsTx(tx *gorm.DB, name, phone string) (bool, error) {
	var count int64
	var err error
	if phone == "" {
		err = tx.Raw("SELECT COUNT(*) FROM students WHERE name=? AND archived_at IS NULL", name).Scan(&count).Error
	} else {
		err = tx.Raw(
			`SELECT COUNT(DISTINCT s.id) FROM students s
			 LEFT JOIN student_guardians sg ON sg.student_id=s.id
			 LEFT JOIN guardians g ON g.id=sg.guardian_id
			 WHERE s.name=? AND s.archived_at IS NULL AND g.phone=?`,
			name, phone,
		).Scan(&count).Error
	}
	return count > 0, err
}

func ensureGuardianTx(tx *gorm.DB, row biz.ImportRow, staffID, batchID string) (string, error) {
	var existing string
	err := tx.Raw("SELECT id FROM guardians WHERE phone=? LIMIT 1", row.Phone).Scan(&existing).Error
	if err != nil {
		return "", err
	}
	if existing != "" {
		if err := tx.Exec(
			`UPDATE guardians SET status='active', approved_by=?, approved_at=NOW(3), bound_at=COALESCE(bound_at,NOW(3)),
			 name=IF(name='',?,name)
			 WHERE id=? AND status IN ('pending','rejected')`,
			staffID, guardianName(row), existing,
		).Error; err != nil {
			return "", err
		}
		return existing, nil
	}
	id := biz.NewID()
	name := guardianName(row)
	err = tx.Exec(
		`INSERT INTO guardians (id,name,phone,status,approved_by,approved_at,bound_at,import_batch_id)
		 VALUES (?,?,?,'active',?,NOW(3),NOW(3),?)`,
		id, name, row.Phone, staffID, batchID,
	).Error
	if err != nil {
		return "", err
	}
	return id, nil
}

func guardianName(row biz.ImportRow) string {
	if row.ParentName != "" {
		return row.ParentName
	}
	return row.Name + "家长"
}

func (r *importRepo) RollbackImportBatch(ctx context.Context, id string) (*biz.ImportBatch, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		batch, _, err := (&importRepo{db: tx}).GetImportBatch(ctx, id)
		if err != nil {
			return err
		}
		if batch.Status != "committed" {
			return biz.ErrImportState
		}
		var ids []string
		if err := tx.Raw("SELECT id FROM students WHERE import_batch_id=?", id).Scan(&ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return biz.ErrImportState
		}
		for _, table := range []string{"follow_ups", "task_assignments", "bookings", "lesson_accounts"} {
			var count int64
			if err := tx.Raw(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE student_id IN (?)", table), ids).Scan(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return biz.ErrImportReferenced
			}
		}
		if err := tx.Exec("DELETE FROM student_guardians WHERE student_id IN (?)", ids).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM students WHERE id IN (?)", ids).Error; err != nil {
			return err
		}
		if err := tx.Exec(
			`DELETE g FROM guardians g
			 WHERE g.import_batch_id=? AND NOT EXISTS (SELECT 1 FROM student_guardians sg WHERE sg.guardian_id=g.id)`,
			id,
		).Error; err != nil {
			return err
		}
		return tx.Exec("UPDATE import_batches SET status='rolled_back', rolled_back_at=NOW(3) WHERE id=?", id).Error
	})
	if err != nil {
		return nil, err
	}
	b, _, err := r.GetImportBatch(ctx, id)
	return b, err
}

func tagsJSON(raw string) string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '，' || r == '、'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if s := strings.TrimSpace(part); s != "" {
			out = append(out, s)
		}
	}
	b, _ := json.Marshal(out)
	return string(b)
}
