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

// TaskRepo 实现任务读写。
type TaskRepo struct {
	db *gorm.DB
}

func NewTaskRepo(db *gorm.DB) *TaskRepo { return &TaskRepo{db: db} }

const taskSelect = `
SELECT t.id,t.title,t.description,t.type,t.status,t.due_at,t.published_at,t.created_at,
 (SELECT COUNT(*) FROM task_assignments a WHERE a.task_id=t.id) AS assigned_count,
 (SELECT COUNT(DISTINCT a.student_id) FROM task_assignments a WHERE a.task_id=t.id AND a.status IN ('submitted','reviewed','returned')) AS submitted_count,
 (SELECT COUNT(*) FROM task_assignments a WHERE a.task_id=t.id AND a.status='reviewed') AS reviewed_count,
 (SELECT COUNT(*) FROM task_assignments a WHERE a.task_id=t.id AND a.status='submitted') AS pending_review,
 (SELECT COUNT(*) FROM task_assignments a WHERE a.task_id=t.id AND a.status='pending' AND t.due_at IS NOT NULL AND t.due_at < NOW(3)) AS overdue_count
FROM tasks t`

func scanTask(row interface{ Scan(...any) error }) (*biz.Task, error) {
	var t biz.Task
	var due, published sql.NullTime
	if err := row.Scan(&t.ID, &t.Title, &t.Description, &t.Type, &t.Status, &due, &published, &t.CreatedAt,
		&t.AssignedCount, &t.SubmittedCount, &t.ReviewedCount, &t.PendingReview, &t.OverdueCount); err != nil {
		return nil, err
	}
	if due.Valid {
		v := due.Time
		t.DueAt = &v
	}
	if published.Valid {
		v := published.Time
		t.PublishedAt = &v
	}
	return &t, nil
}

func (r *TaskRepo) ListTasks(ctx context.Context, status, q string, limit, offset int) ([]biz.Task, int, error) {
	where := []string{"1=1"}
	args := []any{}
	if status != "" {
		where = append(where, "t.status=?")
		args = append(args, status)
	}
	if q != "" {
		where = append(where, "t.title LIKE ?")
		args = append(args, "%"+q+"%")
	}
	clause := " WHERE " + strings.Join(where, " AND ")
	var total int64
	if err := r.db.WithContext(ctx).Raw("SELECT COUNT(*) FROM tasks t"+clause, args...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	rows, err := r.db.WithContext(ctx).Raw(taskSelect+clause+" ORDER BY t.created_at DESC LIMIT ? OFFSET ?", append(args, limit, offset)...).Rows()
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]biz.Task, 0)
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *t)
	}
	return out, int(total), rows.Err()
}

func (r *TaskRepo) CreateTask(ctx context.Context, t *biz.Task) error {
	var due any
	if t.DueAt != nil {
		due = *t.DueAt
	}
	return r.db.WithContext(ctx).Exec(
		`INSERT INTO tasks (id,title,description,type,status,due_at,created_by,created_at)
		 VALUES (?,?,?,?,?,?,?,NOW(3))`,
		t.ID, t.Title, t.Description, t.Type, t.Status, due, t.CreatedBy,
	).Error
}

func (r *TaskRepo) GetTask(ctx context.Context, id string) (*biz.Task, error) {
	t, err := scanTask(r.db.WithContext(ctx).Raw(taskSelect+" WHERE t.id=?", id).Row())
	if err == sql.ErrNoRows {
		return nil, biz.ErrTaskNotFound
	}
	return t, err
}

func (r *TaskRepo) SetTaskStatus(ctx context.Context, id, from, to string) error {
	res := r.db.WithContext(ctx).Exec("UPDATE tasks SET status=? WHERE id=? AND status=?", to, id, from)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return biz.ErrTaskState
	}
	return nil
}

// DeleteTask 删除草稿或已结束任务；已结束任务连同 assignment 与提交历史一起删除。
func (r *TaskRepo) DeleteTask(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var status string
		err := tx.Raw("SELECT status FROM tasks WHERE id=? FOR UPDATE", id).Row().Scan(&status)
		if err == sql.ErrNoRows {
			return biz.ErrTaskNotFound
		}
		if err != nil {
			return err
		}
		if status != biz.TaskDraft && status != biz.TaskClosed {
			return biz.ErrTaskState
		}
		if err := tx.Exec("DELETE FROM submissions WHERE assignment_id IN (SELECT id FROM task_assignments WHERE task_id=?)", id).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM task_assignments WHERE task_id=?", id).Error; err != nil {
			return err
		}
		res := tx.Exec("DELETE FROM tasks WHERE id=?", id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return biz.ErrTaskNotFound
		}
		return nil
	})
}

func (r *TaskRepo) PublishTask(ctx context.Context, taskID string, studentIDs []string) (int, error) {
	count := 0
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var status string
		var due sql.NullTime
		if err := tx.Raw("SELECT status,due_at FROM tasks WHERE id=? FOR UPDATE", taskID).Row().Scan(&status, &due); err != nil {
			if err == sql.ErrNoRows {
				return biz.ErrTaskNotFound
			}
			return err
		}
		if status != biz.TaskDraft {
			return biz.ErrTaskState
		}
		if !due.Valid || !due.Time.After(time.Now()) {
			return biz.ErrInvalidTask
		}
		var existing int
		if err := tx.Raw("SELECT COUNT(*) FROM students WHERE id IN (?) AND archived_at IS NULL", studentIDs).Scan(&existing).Error; err != nil {
			return err
		}
		if existing != len(studentIDs) {
			return biz.ErrInvalidTask
		}
		for _, sid := range studentIDs {
			if err := tx.Exec(
				"INSERT INTO task_assignments (id,task_id,student_id,status,created_at) VALUES (?,?,?,?,NOW(3))",
				biz.NewID(), taskID, sid, biz.AssignmentPending,
			).Error; err != nil {
				return err
			}
			count++
		}
		return tx.Exec("UPDATE tasks SET status=?,published_at=NOW(3) WHERE id=?", biz.TaskPublished, taskID).Error
	})
	return count, err
}

func scanAssignment(row interface{ Scan(...any) error }) (*biz.TaskAssignment, error) {
	var a biz.TaskAssignment
	var due, submitted, reviewed sql.NullTime
	var score sql.NullFloat64
	if err := row.Scan(&a.ID, &a.TaskID, &a.TaskTitle, &a.TaskType, &a.TaskDesc, &a.StudentID, &a.StudentName,
		&a.Status, &due, &submitted, &reviewed, &score, &a.Comment, &a.ReturnReason, &a.CreatedAt); err != nil {
		return nil, err
	}
	if due.Valid {
		v := due.Time
		a.DueAt = &v
	}
	if submitted.Valid {
		v := submitted.Time
		a.SubmittedAt = &v
	}
	if reviewed.Valid {
		v := reviewed.Time
		a.ReviewedAt = &v
	}
	if score.Valid {
		v := score.Float64
		a.Score = &v
	}
	return &a, nil
}

const assignmentSelect = `
SELECT a.id,a.task_id,t.title,t.type,t.description,a.student_id,s.name,a.status,t.due_at,
 a.submitted_at,a.reviewed_at,a.score,a.comment,a.return_reason,a.created_at
FROM task_assignments a
JOIN tasks t ON t.id=a.task_id
JOIN students s ON s.id=a.student_id`

func (r *TaskRepo) ListAssignments(ctx context.Context, taskID string) ([]biz.TaskAssignment, error) {
	rows, err := r.db.WithContext(ctx).Raw(assignmentSelect+" WHERE a.task_id=? ORDER BY s.name", taskID).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]biz.TaskAssignment, 0)
	byID := map[string]int{}
	for rows.Next() {
		a, err := scanAssignment(rows)
		if err != nil {
			return nil, err
		}
		byID[a.ID] = len(out)
		out = append(out, *a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	subs, err := r.submissionsByTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	for id, list := range subs {
		idx, ok := byID[id]
		if !ok {
			continue
		}
		out[idx].History = list
		if len(list) > 0 {
			latest := list[len(list)-1]
			out[idx].Submission = &latest
		}
	}
	return out, nil
}

func (r *TaskRepo) submissionsByTask(ctx context.Context, taskID string) (map[string][]biz.Submission, error) {
	rows, err := r.db.WithContext(ctx).Raw(`
SELECT s.id,s.assignment_id,s.version,s.text_content,s.file_ids,s.submitted_at,s.is_late
FROM submissions s JOIN task_assignments a ON a.id=s.assignment_id
WHERE a.task_id=? ORDER BY s.assignment_id,s.version`, taskID).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]biz.Submission{}
	for rows.Next() {
		var s biz.Submission
		var aid string
		var raw []byte
		var late bool
		if err := rows.Scan(&s.ID, &aid, &s.Version, &s.Text, &raw, &s.SubmittedAt, &late); err != nil {
			return nil, err
		}
		s.IsLate = late
		s.FileIDs = decodeFileIDs(raw)
		out[aid] = append(out[aid], s)
	}
	return out, rows.Err()
}

func decodeFileIDs(raw []byte) []string {
	out := []string{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	if out == nil {
		out = []string{}
	}
	return out
}

func (r *TaskRepo) ReviewQueue(ctx context.Context, limit int) ([]biz.TaskAssignment, error) {
	rows, err := r.db.WithContext(ctx).Raw(assignmentSelect+" WHERE a.status='submitted' ORDER BY t.due_at ASC LIMIT ?", limit).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]biz.TaskAssignment, 0)
	for rows.Next() {
		a, err := scanAssignment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

func (r *TaskRepo) ReviewAssignment(ctx context.Context, assignmentID, reviewerID string, score *float64, comment, returnReason string) error {
	status := biz.AssignmentReviewed
	if returnReason != "" {
		status = biz.AssignmentReturned
	}
	var scoreArg any
	if score != nil {
		scoreArg = *score
	}
	res := r.db.WithContext(ctx).Exec(`
UPDATE task_assignments
SET status=?,score=?,comment=?,return_reason=?,reviewed_at=NOW(3),updated_at=NOW(3)
WHERE id=? AND status='submitted'`, status, scoreArg, comment, returnReason, assignmentID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		var exists int
		if err := r.db.WithContext(ctx).Raw("SELECT COUNT(*) FROM task_assignments WHERE id=?", assignmentID).Scan(&exists).Error; err != nil {
			return err
		}
		if exists == 0 {
			return biz.ErrAssignmentNotFound
		}
		return biz.ErrAlreadyReviewed
	}
	return nil
}

func (r *TaskRepo) portalAssignments(ctx context.Context, guardianID, extra string, args ...any) ([]biz.TaskAssignment, error) {
	where := ` WHERE sg.guardian_id=? AND g.status='active'`
	baseArgs := []any{guardianID}
	if extra != "" {
		where += " AND " + extra
		baseArgs = append(baseArgs, args...)
	}
	rows, err := r.db.WithContext(ctx).Raw(assignmentSelect+`
JOIN student_guardians sg ON sg.student_id=a.student_id
JOIN guardians g ON g.id=sg.guardian_id`+where+` ORDER BY t.published_at DESC, t.created_at DESC`, baseArgs...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]biz.TaskAssignment, 0)
	for rows.Next() {
		a, err := scanAssignment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

func (r *TaskRepo) PortalTasks(ctx context.Context, guardianID string) ([]biz.TaskAssignment, error) {
	return r.portalAssignments(ctx, guardianID, "")
}

func (r *TaskRepo) PortalArchive(ctx context.Context, guardianID string) ([]biz.TaskAssignment, error) {
	return r.portalAssignments(ctx, guardianID, "a.status='reviewed'")
}

func (r *TaskRepo) PortalTask(ctx context.Context, guardianID, assignmentID string) (*biz.TaskAssignment, error) {
	list, err := r.portalAssignments(ctx, guardianID, "a.id=?", assignmentID)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, biz.ErrAssignmentNotFound
	}
	a := list[0]
	subs, err := r.submissionsByAssignment(ctx, assignmentID)
	if err != nil {
		return nil, err
	}
	a.History = subs
	if len(subs) > 0 {
		latest := subs[len(subs)-1]
		a.Submission = &latest
	}
	return &a, nil
}

func (r *TaskRepo) submissionsByAssignment(ctx context.Context, assignmentID string) ([]biz.Submission, error) {
	rows, err := r.db.WithContext(ctx).Raw(`
SELECT id,version,text_content,file_ids,submitted_at,is_late FROM submissions
WHERE assignment_id=? ORDER BY version`, assignmentID).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []biz.Submission{}
	for rows.Next() {
		var s biz.Submission
		var raw []byte
		var late bool
		if err := rows.Scan(&s.ID, &s.Version, &s.Text, &raw, &s.SubmittedAt, &late); err != nil {
			return nil, err
		}
		s.IsLate = late
		s.FileIDs = decodeFileIDs(raw)
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *TaskRepo) Submit(ctx context.Context, guardianID, assignmentID, text string, fileIDs []string) (*biz.Submission, error) {
	var sub *biz.Submission
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var status string
		var due sql.NullTime
		err := tx.Raw(`
SELECT a.status,t.due_at FROM task_assignments a
JOIN tasks t ON t.id=a.task_id
JOIN student_guardians sg ON sg.student_id=a.student_id
JOIN guardians g ON g.id=sg.guardian_id
WHERE a.id=? AND sg.guardian_id=? AND g.status='active'
FOR UPDATE`, assignmentID, guardianID).Row().Scan(&status, &due)
		if err == sql.ErrNoRows {
			return biz.ErrAssignmentNotFound
		}
		if err != nil {
			return err
		}
		if status != biz.AssignmentPending && status != biz.AssignmentReturned {
			return biz.ErrTaskState
		}
		var next int
		if err := tx.Raw("SELECT COALESCE(MAX(version),0)+1 FROM submissions WHERE assignment_id=?", assignmentID).Scan(&next).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		late := due.Valid && now.After(due.Time)
		raw, _ := json.Marshal(fileIDs)
		id := biz.NewID()
		if err := tx.Exec(`
INSERT INTO submissions (id,assignment_id,version,text_content,file_ids,submitted_by,submitted_at,is_late)
VALUES (?,?,?,?,?,?,NOW(3),?)`, id, assignmentID, next, text, string(raw), guardianID, late).Error; err != nil {
			return err
		}
		if err := tx.Exec(`
UPDATE task_assignments SET status='submitted',submitted_at=NOW(3),reviewed_at=NULL,score=NULL,comment='',return_reason='',updated_at=NOW(3)
WHERE id=?`, assignmentID).Error; err != nil {
			return err
		}
		sub = &biz.Submission{ID: id, Version: next, Text: text, FileIDs: fileIDs, SubmittedAt: now, IsLate: late}
		return nil
	})
	return sub, err
}

func (r *TaskRepo) ListTemplates(ctx context.Context) ([]biz.TaskTemplate, error) {
	var rows []struct {
		ID             string    `gorm:"column:id"`
		Title          string    `gorm:"column:title"`
		Description    string    `gorm:"column:description"`
		Type           string    `gorm:"column:type"`
		DueOffsetHours int       `gorm:"column:due_offset_hours"`
		CreatedBy      string    `gorm:"column:created_by"`
		CreatedAt      time.Time `gorm:"column:created_at"`
	}
	if err := r.db.WithContext(ctx).Raw(`
SELECT id,title,description,type,due_offset_hours,created_by,created_at
FROM task_templates ORDER BY created_at DESC LIMIT 100`).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]biz.TaskTemplate, 0, len(rows))
	for _, row := range rows {
		out = append(out, biz.TaskTemplate{
			ID: row.ID, Title: row.Title, Description: row.Description, Type: row.Type,
			DueOffsetHours: row.DueOffsetHours, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt,
		})
	}
	return out, nil
}

func (r *TaskRepo) CreateTemplate(ctx context.Context, t *biz.TaskTemplate) error {
	return r.db.WithContext(ctx).Exec(`
INSERT INTO task_templates (id,title,description,type,due_offset_hours,created_by)
VALUES (?,?,?,?,?,?)`,
		t.ID, t.Title, t.Description, t.Type, t.DueOffsetHours, t.CreatedBy).Error
}

func (r *TaskRepo) UpdateTemplate(ctx context.Context, t *biz.TaskTemplate) error {
	res := r.db.WithContext(ctx).Exec(`
UPDATE task_templates SET title=?,description=?,type=?,due_offset_hours=? WHERE id=?`,
		t.Title, t.Description, t.Type, t.DueOffsetHours, t.ID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return biz.ErrTaskNotFound
	}
	var row struct {
		ID             string    `gorm:"column:id"`
		Title          string    `gorm:"column:title"`
		Description    string    `gorm:"column:description"`
		Type           string    `gorm:"column:type"`
		DueOffsetHours int       `gorm:"column:due_offset_hours"`
		CreatedBy      string    `gorm:"column:created_by"`
		CreatedAt      time.Time `gorm:"column:created_at"`
	}
	if err := r.db.WithContext(ctx).Raw(`
SELECT id,title,description,type,due_offset_hours,created_by,created_at
FROM task_templates WHERE id=?`, t.ID).Scan(&row).Error; err != nil {
		return err
	}
	t.Title, t.Description, t.Type = row.Title, row.Description, row.Type
	t.DueOffsetHours, t.CreatedBy, t.CreatedAt = row.DueOffsetHours, row.CreatedBy, row.CreatedAt
	return nil
}

func (r *TaskRepo) DeleteTemplate(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).Exec("DELETE FROM task_templates WHERE id=?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return biz.ErrTaskNotFound
	}
	return nil
}
