package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/bishop9910/hm-lesson/backend/internal/biz"
	"gorm.io/gorm"
)

// classRepo 班课仓储实现。
type classRepo struct {
	db *gorm.DB
}

// NewClassRepo 构造班课仓储。
func NewClassRepo(db *gorm.DB) biz.ClassRepo {
	return &classRepo{db: db}
}

const classSelect = `SELECT c.id, c.name, c.subject, c.start_date, c.end_date, c.schedule_note,
       c.status, c.created_by, c.created_at,
       (SELECT COUNT(*) FROM class_members m WHERE m.class_id = c.id AND m.left_at IS NULL) AS member_count,
       (SELECT COUNT(*) FROM class_sessions s WHERE s.class_id = c.id) AS session_count
  FROM classes c`

type classRow struct {
	ID           string
	Name         string
	Subject      string
	StartDate    sql.NullTime
	EndDate      sql.NullTime
	ScheduleNote string
	Status       string
	CreatedBy    string
	CreatedAt    time.Time
	MemberCount  int
	SessionCount int
}

func (r classRow) toBiz() biz.Class {
	out := biz.Class{
		ID:           r.ID,
		Name:         r.Name,
		Subject:      r.Subject,
		ScheduleNote: r.ScheduleNote,
		Status:       r.Status,
		CreatedBy:    r.CreatedBy,
		CreatedAt:    r.CreatedAt.UTC(),
		MemberCount:  r.MemberCount,
		SessionCount: r.SessionCount,
	}
	if r.StartDate.Valid {
		t := r.StartDate.Time.UTC()
		out.StartDate = &t
	}
	if r.EndDate.Valid {
		t := r.EndDate.Time.UTC()
		out.EndDate = &t
	}
	return out
}

func (r *classRepo) ListClasses(ctx context.Context, status string) ([]biz.Class, error) {
	query := classSelect
	args := []any{}
	if status != "" {
		query += " WHERE c.status = ?"
		args = append(args, status)
	}
	query += " ORDER BY c.created_at DESC LIMIT 200"
	var rows []classRow
	if err := r.db.WithContext(ctx).Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]biz.Class, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.toBiz())
	}
	return out, nil
}

func (r *classRepo) GetClass(ctx context.Context, classID string) (*biz.Class, error) {
	var rows []classRow
	if err := r.db.WithContext(ctx).Raw(classSelect+" WHERE c.id = ?", classID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, biz.ErrClassNotFound
	}
	class := rows[0].toBiz()
	return &class, nil
}

func (r *classRepo) CreateClass(ctx context.Context, class *biz.Class) error {
	res := r.db.WithContext(ctx).Exec(
		`INSERT INTO classes (id, name, subject, start_date, end_date, schedule_note, status, created_by, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		class.ID, class.Name, class.Subject, dateOrNull(class.StartDate), dateOrNull(class.EndDate),
		class.ScheduleNote, class.Status, class.CreatedBy, class.CreatedAt, class.CreatedAt,
	)
	if res.Error != nil {
		if isFKViolation(res.Error) {
			return biz.ErrClassNotFound
		}
		return res.Error
	}
	return nil
}

func (r *classRepo) UpdateClass(ctx context.Context, class *biz.Class) error {
	res := r.db.WithContext(ctx).Exec(
		`UPDATE classes SET name = ?, subject = ?, start_date = ?, end_date = ?, schedule_note = ?, status = ?
		  WHERE id = ?`,
		class.Name, class.Subject, dateOrNull(class.StartDate), dateOrNull(class.EndDate),
		class.ScheduleNote, class.Status, class.ID,
	)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return biz.ErrClassNotFound
	}
	return nil
}

type classMemberRow struct {
	StudentID   string
	StudentName string
	Grade       string
	GroupCode   string
	JoinedAt    time.Time
	LeftAt      sql.NullTime
}

func (r *classRepo) ListMembers(ctx context.Context, classID string) ([]biz.ClassMember, error) {
	var rows []classMemberRow
	err := r.db.WithContext(ctx).Raw(
		`SELECT m.student_id, s.name AS student_name, s.grade, s.group_code, m.joined_at, m.left_at
		   FROM class_members m
		   JOIN students s ON s.id = m.student_id
		  WHERE m.class_id = ?
		  ORDER BY m.left_at IS NULL DESC, m.joined_at ASC`, classID,
	).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]biz.ClassMember, 0, len(rows))
	for _, row := range rows {
		item := biz.ClassMember{
			StudentID:   row.StudentID,
			StudentName: row.StudentName,
			Grade:       row.Grade,
			GroupCode:   row.GroupCode,
			JoinedAt:    row.JoinedAt.UTC(),
		}
		if row.LeftAt.Valid {
			t := row.LeftAt.Time.UTC()
			item.LeftAt = &t
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *classRepo) AddMembers(ctx context.Context, classID string, studentIDs []string) (int, error) {
	added := 0
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return 0, tx.Error
	}
	defer func() { _ = tx.Rollback() }()
	now := time.Now().UTC()
	for _, studentID := range studentIDs {
		var status string
		var archived sql.NullTime
		row := tx.Raw(`SELECT status, archived_at FROM students WHERE id = ?`, studentID).Row()
		if err := row.Scan(&status, &archived); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return 0, biz.ErrStudentNotFound
			}
			return 0, err
		}
		if status != "active" || archived.Valid {
			return 0, biz.ErrStudentNotActive
		}
		res := tx.Exec(
			`INSERT IGNORE INTO class_members (id, class_id, student_id, joined_at, created_at)
			 VALUES (?, ?, ?, ?, ?)`,
			biz.NewID(), classID, studentID, now.Format("2006-01-02"), now,
		)
		if res.Error != nil {
			if isFKViolation(res.Error) {
				return 0, biz.ErrClassNotFound
			}
			return 0, res.Error
		}
		if res.RowsAffected > 0 {
			added++
		}
	}
	if err := tx.Commit().Error; err != nil {
		return 0, err
	}
	return added, nil
}

func (r *classRepo) RemoveMember(ctx context.Context, classID, studentID string) error {
	res := r.db.WithContext(ctx).Exec(
		`UPDATE class_members SET left_at = ? WHERE class_id = ? AND student_id = ? AND left_at IS NULL`,
		time.Now().UTC().Format("2006-01-02"), classID, studentID,
	)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return biz.ErrClassMemberNotFound
	}
	return nil
}

type classSessionRow struct {
	ID          string
	ClassID     string
	SessionDate time.Time
	Content     string
	CreatedBy   string
	CreatedAt   time.Time
	Attendance  []byte
}

func (r *classRepo) ListSessions(ctx context.Context, classID string) ([]biz.ClassSession, error) {
	var rows []classSessionRow
	err := r.db.WithContext(ctx).Raw(
		`SELECT s.id, s.class_id, s.session_date, s.content, s.created_by, s.created_at, s.attendance
		   FROM class_sessions s
		  WHERE s.class_id = ?
		  ORDER BY s.session_date DESC, s.created_at DESC
		  LIMIT 200`, classID,
	).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	// 出勤快照只存学员 id，名字按班级成员补齐。
	members, err := r.ListMembers(ctx, classID)
	if err != nil {
		return nil, err
	}
	names := make(map[string]string, len(members))
	for _, member := range members {
		names[member.StudentID] = member.StudentName
	}
	out := make([]biz.ClassSession, 0, len(rows))
	for _, row := range rows {
		item := biz.ClassSession{
			ID:          row.ID,
			ClassID:     row.ClassID,
			SessionDate: row.SessionDate.UTC(),
			Content:     row.Content,
			CreatedBy:   row.CreatedBy,
			CreatedAt:   row.CreatedAt.UTC(),
		}
		item.Attendance = decodeAttendance(row.Attendance, names)
		out = append(out, item)
	}
	return out, nil
}

func (r *classRepo) CreateSession(ctx context.Context, session *biz.ClassSession) error {
	payload, err := encodeAttendance(session.Attendance)
	if err != nil {
		return err
	}
	res := r.db.WithContext(ctx).Exec(
		`INSERT INTO class_sessions (id, class_id, session_date, attendance, content, created_by, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.ClassID, session.SessionDate, payload, session.Content, session.CreatedBy, session.CreatedAt,
	)
	if res.Error != nil {
		if isFKViolation(res.Error) {
			return biz.ErrClassNotFound
		}
		return res.Error
	}
	return nil
}

func (r *classRepo) DeleteSession(ctx context.Context, sessionID string) error {
	res := r.db.WithContext(ctx).Exec(`DELETE FROM class_sessions WHERE id = ?`, sessionID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return biz.ErrSessionNotFound
	}
	return nil
}

func dateOrNull(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format("2006-01-02")
}

// encodeAttendance 把出勤记录压成 {"student_id":"present"} 的 JSON。
func encodeAttendance(rows []biz.AttendanceRow) (string, error) {
	if len(rows) == 0 {
		// MySQL 5.7 的 JSON 列不接受空字符串（3140），没有出勤时写空对象。
		return "{}", nil
	}
	payload := make(map[string]string, len(rows))
	for _, row := range rows {
		payload[row.StudentID] = row.Status
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

// decodeAttendance 还原出勤记录，并按成员表补学员姓名。
func decodeAttendance(raw []byte, names map[string]string) []biz.AttendanceRow {
	if len(raw) == 0 {
		return nil
	}
	payload := map[string]string{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	ids := make([]string, 0, len(payload))
	for id := range payload {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]biz.AttendanceRow, 0, len(ids))
	for _, id := range ids {
		out = append(out, biz.AttendanceRow{StudentID: id, StudentName: names[id], Status: payload[id]})
	}
	return out
}
