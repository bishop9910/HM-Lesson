package data

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/bishop9910/hm-lesson/backend/internal/biz"
)

type crmRepo struct {
	db *gorm.DB
}

func NewCRMRepo(db *gorm.DB) biz.CRMRepo { return &crmRepo{db: db} }

type studentRow struct {
	ID             string
	Name           string
	Grade          string
	School         string
	GroupCode      string
	Status         string
	Tags           []byte
	NextFollowUpAt sql.NullTime
	Note           string
	CreatedBy      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (r studentRow) toBiz() biz.Student {
	s := biz.Student{
		ID: r.ID, Name: r.Name, Grade: r.Grade, School: r.School,
		GroupCode: r.GroupCode, Status: r.Status, Note: r.Note,
		CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		Tags: []string{},
	}
	if len(r.Tags) > 0 {
		_ = json.Unmarshal(r.Tags, &s.Tags)
	}
	if r.NextFollowUpAt.Valid {
		t := r.NextFollowUpAt.Time
		s.NextFollowUpAt = &t
	}
	return s
}

const studentColumns = "id,name,grade,school,group_code,status,tags,next_follow_up_at,note,created_by,created_at,updated_at"

func (r *crmRepo) ListStudents(ctx context.Context, f biz.StudentFilter) ([]biz.Student, int, error) {
	where := []string{"archived_at IS NULL"}
	args := []any{}
	if f.Group != "" {
		where = append(where, "group_code = ?")
		args = append(args, f.Group)
	}
	if f.Status != "" {
		where = append(where, "status = ?")
		args = append(args, f.Status)
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		like := "%" + q + "%"
		where = append(where, "(name LIKE ? OR school LIKE ? OR note LIKE ?)")
		args = append(args, like, like, like)
	}
	cond := strings.Join(where, " AND ")
	var total int64
	if err := r.db.WithContext(ctx).Raw("SELECT COUNT(*) FROM students WHERE "+cond, args...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (f.Page - 1) * f.PageSize
	var rows []studentRow
	err := r.db.WithContext(ctx).
		Raw("SELECT "+studentColumns+" FROM students WHERE "+cond+" ORDER BY created_at DESC LIMIT ? OFFSET ?", append(args, f.PageSize, offset)...).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	out := make([]biz.Student, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.toBiz())
	}
	return out, int(total), nil
}

func (r *crmRepo) GetStudent(ctx context.Context, id string) (*biz.Student, error) {
	var row studentRow
	err := r.db.WithContext(ctx).Raw("SELECT "+studentColumns+" FROM students WHERE id = ?", id).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.ID == "" {
		return nil, biz.ErrStudentNotFound
	}
	s := row.toBiz()
	return &s, nil
}

func (r *crmRepo) CreateStudent(ctx context.Context, s *biz.Student) (*biz.Student, error) {
	tags, _ := json.Marshal(s.Tags)
	err := r.db.WithContext(ctx).Exec(
		`INSERT INTO students (id,name,grade,school,group_code,status,tags,next_follow_up_at,note,created_by)
		 VALUES (?,?,?,?,?,?,?,?,?,?)`,
		s.ID, s.Name, s.Grade, s.School, s.GroupCode, s.Status, string(tags), s.NextFollowUpAt, s.Note, s.CreatedBy,
	).Error
	if err != nil {
		return nil, err
	}
	return r.GetStudent(ctx, s.ID)
}

func (r *crmRepo) UpdateStudent(ctx context.Context, id string, in biz.UpdateStudentInput) (*biz.Student, error) {
	updates := map[string]any{}
	if in.Name != nil {
		updates["name"] = *in.Name
	}
	if in.Grade != nil {
		updates["grade"] = *in.Grade
	}
	if in.School != nil {
		updates["school"] = *in.School
	}
	if in.GroupCode != nil {
		updates["group_code"] = *in.GroupCode
	}
	if in.Status != nil {
		updates["status"] = *in.Status
	}
	if in.Tags != nil {
		b, _ := json.Marshal(*in.Tags)
		updates["tags"] = string(b)
	}
	if in.ClearNextFollowUp {
		updates["next_follow_up_at"] = nil
	} else if in.NextFollowUpAt != nil {
		updates["next_follow_up_at"] = *in.NextFollowUpAt
	}
	if in.Note != nil {
		updates["note"] = strings.TrimSpace(*in.Note)
	}
	if len(updates) > 0 {
		res := r.db.WithContext(ctx).Table("students").Where("id = ? AND archived_at IS NULL", id).Updates(updates)
		if res.Error != nil {
			return nil, res.Error
		}
		if res.RowsAffected == 0 {
			return nil, biz.ErrStudentNotFound
		}
	}
	return r.GetStudent(ctx, id)
}

type followUpRow struct {
	ID           string
	StudentID    string
	Method       string
	Content      string
	Concern      string
	NextAction   string
	NextFollowUp sql.NullTime
	CreatedBy    string
	CreatedAt    time.Time
}

func (r *crmRepo) AddFollowUp(ctx context.Context, f *biz.FollowUp) (*biz.FollowUp, error) {
	err := r.db.WithContext(ctx).Exec(
		`INSERT INTO follow_ups (id,student_id,method,content,concern,next_action,next_follow_up_at,created_by)
		 VALUES (?,?,?,?,?,?,?,?)`,
		f.ID, f.StudentID, f.Method, f.Content, f.Concern, f.NextAction, f.NextFollowUp, f.CreatedBy,
	).Error
	if err != nil {
		return nil, err
	}
	if f.NextFollowUp != nil {
		_ = r.db.WithContext(ctx).Table("students").Where("id = ?", f.StudentID).
			Update("next_follow_up_at", *f.NextFollowUp).Error
	}
	return f, nil
}

func (r *crmRepo) ListFollowUps(ctx context.Context, studentID string, limit int) ([]biz.FollowUp, error) {
	var rows []followUpRow
	err := r.db.WithContext(ctx).
		Raw(`SELECT id,student_id,method,content,concern,next_action,next_follow_up_at,created_by,created_at
		     FROM follow_ups WHERE student_id = ? ORDER BY created_at DESC LIMIT ?`, studentID, limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]biz.FollowUp, 0, len(rows))
	for _, row := range rows {
		f := biz.FollowUp{
			ID: row.ID, StudentID: row.StudentID, Method: row.Method, Content: row.Content,
			Concern: row.Concern, NextAction: row.NextAction, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt,
		}
		if row.NextFollowUp.Valid {
			t := row.NextFollowUp.Time
			f.NextFollowUp = &t
		}
		out = append(out, f)
	}
	return out, nil
}

func (r *crmRepo) ListPendingGuardians(ctx context.Context) ([]biz.GuardianRequest, error) {
	var rows []struct {
		ID        string
		Phone     string
		Name      string
		Status    string
		CreatedAt time.Time
	}
	err := r.db.WithContext(ctx).
		Raw(`SELECT g.id,g.phone,g.name,g.status,g.created_at
		     FROM guardians g
		     WHERE g.status = 'pending'
		        OR (g.status = 'active' AND NOT EXISTS (
		              SELECT 1 FROM student_guardians sg WHERE sg.guardian_id = g.id
		            ))
		     ORDER BY g.created_at ASC`).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]biz.GuardianRequest, 0, len(rows))
	for _, row := range rows {
		out = append(out, biz.GuardianRequest{ID: row.ID, Phone: row.Phone, Name: row.Name, Status: row.Status, CreatedAt: row.CreatedAt})
	}
	return out, nil
}

func (r *crmRepo) ApproveGuardian(ctx context.Context, guardianID, studentID, actorID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Raw("SELECT COUNT(*) FROM students WHERE id = ? AND archived_at IS NULL", studentID).Scan(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return biz.ErrStudentNotFound
		}
		res := tx.Exec(
			`UPDATE guardians SET status='active', approved_by=?, approved_at=NOW(3), bound_at=NOW(3), rejected_reason=NULL
			 WHERE id=?
			   AND (status='pending' OR (status='active' AND NOT EXISTS (
			         SELECT 1 FROM student_guardians sg WHERE sg.guardian_id = guardians.id
			       )))`, actorID, guardianID)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return biz.ErrGuardianNotFound
		}
		return tx.Exec(
			`INSERT IGNORE INTO student_guardians (id,student_id,guardian_id,relation,is_primary)
			 VALUES (?,?,?,'parent',0)`, biz.NewID(), studentID, guardianID).Error
	})
}

func (r *crmRepo) RejectGuardian(ctx context.Context, guardianID, reason, actorID string) error {
	res := r.db.WithContext(ctx).Exec(
		`UPDATE guardians SET status='rejected', rejected_reason=?, approved_by=?, approved_at=NOW(3)
		 WHERE id=?
		   AND (status='pending' OR (status='active' AND NOT EXISTS (
		         SELECT 1 FROM student_guardians sg WHERE sg.guardian_id = guardians.id
		       )))`, reason, actorID, guardianID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return biz.ErrGuardianNotFound
	}
	return nil
}

func randomCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func (r *crmRepo) CreateBindingCode(ctx context.Context, studentID, actorID string, ttl time.Duration) (*biz.BindingCode, error) {
	var exists int64
	if err := r.db.WithContext(ctx).Raw("SELECT COUNT(*) FROM students WHERE id = ? AND archived_at IS NULL", studentID).Scan(&exists).Error; err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, biz.ErrStudentNotFound
	}
	for i := 0; i < 4; i++ {
		code, err := randomCode()
		if err != nil {
			return nil, err
		}
		expires := time.Now().UTC().Add(ttl)
		err = r.db.WithContext(ctx).Exec(
			`INSERT INTO binding_codes (id,code,student_id,created_by,expires_at) VALUES (?,?,?,?,?)`,
			biz.NewID(), code, studentID, actorID, expires).Error
		if err == nil {
			return &biz.BindingCode{Code: code, StudentID: studentID, ExpiresAt: expires}, nil
		}
		if !strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return nil, err
		}
	}
	return nil, errors.New("could not allocate binding code")
}

func (r *crmRepo) RedeemBindingCode(ctx context.Context, phone, code string) (*biz.GuardianRequest, []biz.Student, error) {
	var guardian biz.GuardianRequest
	var students []biz.Student
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var bc struct {
			ID        string
			StudentID string
			ExpiresAt time.Time
			UsedAt    sql.NullTime
		}
		err := tx.Raw(`SELECT id,student_id,expires_at,used_at FROM binding_codes WHERE code = ? FOR UPDATE`, code).Scan(&bc).Error
		if err != nil {
			return err
		}
		if bc.ID == "" {
			return biz.ErrInvalidCode
		}
		if bc.UsedAt.Valid {
			return biz.ErrCodeUsed
		}
		if time.Now().UTC().After(bc.ExpiresAt) {
			return biz.ErrCodeExpired
		}
		var student studentRow
		if err := tx.Raw("SELECT "+studentColumns+" FROM students WHERE id = ? AND archived_at IS NULL", bc.StudentID).Scan(&student).Error; err != nil {
			return err
		}
		if student.ID == "" {
			return biz.ErrStudentNotFound
		}
		var existing struct {
			ID     string
			Status string
		}
		_ = tx.Raw(`SELECT id,status FROM guardians WHERE phone = ? ORDER BY FIELD(status,'active','pending','rejected','disabled'), created_at LIMIT 1`, phone).Scan(&existing).Error
		if existing.ID != "" && existing.Status == "disabled" {
			return biz.ErrGuardianDisabled
		}
		gid := existing.ID
		if gid == "" {
			gid = biz.NewID()
			if err := tx.Exec(`INSERT INTO guardians (id,phone,status) VALUES (?,?, 'active')`, gid, phone).Error; err != nil {
				return err
			}
		} else if err := tx.Exec(`UPDATE guardians SET status='active', bound_at=NOW(3), rejected_reason=NULL WHERE id=?`, gid).Error; err != nil {
			return err
		}
		if err := tx.Exec(`INSERT IGNORE INTO student_guardians (id,student_id,guardian_id,relation,is_primary) VALUES (?,?,?,'parent',0)`,
			biz.NewID(), bc.StudentID, gid).Error; err != nil {
			return err
		}
		if err := tx.Exec(`UPDATE binding_codes SET used_at=NOW(3), used_by_guardian_id=? WHERE id=?`, gid, bc.ID).Error; err != nil {
			return err
		}
		guardian = biz.GuardianRequest{ID: gid, Phone: phone, Status: "active"}
		students = append(students, student.toBiz())
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return &guardian, students, nil
}

func (r *crmRepo) GuardianByPhone(ctx context.Context, phone string) (*biz.GuardianRequest, error) {
	var row struct {
		ID        string
		Phone     string
		Name      string
		Status    string
		CreatedAt time.Time
	}
	err := r.db.WithContext(ctx).
		Raw(`SELECT id,phone,name,status,created_at FROM guardians WHERE phone = ?
		     ORDER BY FIELD(status,'active','pending','rejected','disabled'), created_at LIMIT 1`, phone).
		Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.ID == "" {
		return nil, biz.ErrGuardianNotFound
	}
	return &biz.GuardianRequest{ID: row.ID, Phone: row.Phone, Name: row.Name, Status: row.Status, CreatedAt: row.CreatedAt}, nil
}

func (r *crmRepo) PortalHome(ctx context.Context, guardianID string) (*biz.PortalHome, error) {
	var g struct {
		Name  string
		Phone string
	}
	if err := r.db.WithContext(ctx).Raw(`SELECT name,phone FROM guardians WHERE id = ? AND status='active'`, guardianID).Scan(&g).Error; err != nil {
		return nil, err
	}
	if g.Phone == "" {
		return nil, biz.ErrGuardianNotFound
	}
	var rows []studentRow
	err := r.db.WithContext(ctx).Raw(
		`SELECT `+prefixColumns("s.")+` FROM students s
		 JOIN student_guardians sg ON sg.student_id = s.id
		 WHERE sg.guardian_id = ? AND s.archived_at IS NULL ORDER BY s.created_at DESC`, guardianID).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := &biz.PortalHome{GuardianName: g.Name, GuardianPhone: g.Phone, Students: []biz.Student{}}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		out.Students = append(out.Students, row.toBiz())
		ids = append(ids, row.ID)
	}
	if len(ids) > 0 {
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
		args := make([]any, 0, len(ids))
		for _, id := range ids {
			args = append(args, id)
		}
		_ = r.db.WithContext(ctx).Raw(
			`SELECT COUNT(*) FROM task_assignments WHERE status='pending' AND student_id IN (`+placeholders+`)`, args...).
			Scan(&out.PendingTasks).Error
		var balance sql.NullFloat64
		_ = r.db.WithContext(ctx).Raw(
			`SELECT COALESCE(SUM(balance_hours),0) FROM lesson_accounts WHERE student_id IN (`+placeholders+`)`, args...).
			Scan(&balance).Error
		if balance.Valid {
			out.BalanceHours = balance.Float64
		}
	}
	return out, nil
}

func (r *crmRepo) PortalStudent(ctx context.Context, guardianID, studentID string) (*biz.PortalStudent, error) {
	var allowed int
	if err := r.db.WithContext(ctx).Raw(
		`SELECT COUNT(*) FROM student_guardians sg
		 JOIN guardians g ON g.id = sg.guardian_id
		 WHERE sg.guardian_id = ? AND sg.student_id = ? AND g.status='active'`,
		guardianID, studentID,
	).Scan(&allowed).Error; err != nil {
		return nil, err
	}
	if allowed == 0 {
		return nil, biz.ErrStudentNotFound
	}
	var row studentRow
	if err := r.db.WithContext(ctx).Raw(`SELECT `+studentColumns+` FROM students WHERE id = ? AND archived_at IS NULL`, studentID).Scan(&row).Error; err != nil {
		return nil, err
	}
	if row.ID == "" {
		return nil, biz.ErrStudentNotFound
	}
	out := &biz.PortalStudent{Student: row.toBiz(), Tasks: []biz.PortalTask{}, Lessons: []biz.PortalLesson{}}
	var tasks []struct {
		ID     string
		Title  string
		Status string
		DueAt  sql.NullTime
	}
	if err := r.db.WithContext(ctx).Raw(
		`SELECT ta.id, t.title, ta.status, t.due_at
		 FROM task_assignments ta JOIN tasks t ON t.id = ta.task_id
		 WHERE ta.student_id = ? ORDER BY ta.status='pending' DESC, t.due_at IS NULL, t.due_at ASC LIMIT 50`,
		studentID,
	).Scan(&tasks).Error; err != nil {
		return nil, err
	}
	for _, t := range tasks {
		item := biz.PortalTask{ID: t.ID, Title: t.Title, Status: t.Status}
		if t.DueAt.Valid {
			v := t.DueAt.Time
			item.DueAt = &v
		}
		out.Tasks = append(out.Tasks, item)
	}
	var account struct {
		Balance   sql.NullFloat64
		ExpiresAt sql.NullTime
	}
	if err := r.db.WithContext(ctx).Raw(
		`SELECT balance_hours AS balance, expires_at FROM lesson_accounts WHERE student_id = ?`, studentID,
	).Scan(&account).Error; err == nil {
		if account.Balance.Valid {
			out.BalanceHours = account.Balance.Float64
		}
		if account.ExpiresAt.Valid {
			v := account.ExpiresAt.Time
			out.ExpiresAt = &v
		}
	}
	var lessons []struct {
		ID           string
		Kind         string
		DeltaHours   float64
		BalanceAfter float64
		Note         string
		CreatedAt    time.Time
	}
	if err := r.db.WithContext(ctx).Raw(
		`SELECT id, kind, delta_hours, balance_after, note, created_at
		 FROM lesson_ledger WHERE student_id = ? ORDER BY created_at DESC LIMIT 20`, studentID,
	).Scan(&lessons).Error; err != nil {
		return nil, err
	}
	for _, l := range lessons {
		out.Lessons = append(out.Lessons, biz.PortalLesson{
			ID: l.ID, Kind: l.Kind, DeltaHours: l.DeltaHours,
			BalanceAfter: l.BalanceAfter, Note: l.Note, CreatedAt: l.CreatedAt,
		})
	}
	return out, nil
}

func prefixColumns(prefix string) string {
	cols := strings.Split(studentColumns, ",")
	for i := range cols {
		cols[i] = prefix + cols[i]
	}
	return strings.Join(cols, ",")
}
