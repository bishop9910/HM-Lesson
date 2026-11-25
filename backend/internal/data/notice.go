package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/bishop9910/hm-lesson/backend/internal/biz"
	"gorm.io/gorm"
)

// noticeRepo 通知与回执仓储实现。
type noticeRepo struct {
	db *gorm.DB
}

// NewNoticeRepo 构造通知仓储。
func NewNoticeRepo(db *gorm.DB) biz.NoticeRepo {
	return &noticeRepo{db: db}
}

const noticeSelect = `SELECT n.id, n.title, n.body, n.file_ids, n.audience_type, n.audience,
       n.need_receipt, n.status, n.sent_at, n.created_by, n.created_at,
       st.name AS creator_name,
       (SELECT COUNT(*) FROM notice_receipts r WHERE r.notice_id = n.id) AS receipt_total,
       (SELECT COUNT(*) FROM notice_receipts r WHERE r.notice_id = n.id AND r.status = 'confirmed') AS receipt_done
  FROM notices n
  LEFT JOIN staff st ON st.id = n.created_by`

type noticeRow struct {
	ID           string
	Title        string
	Body         string
	FileIDs      []byte
	AudienceType string
	Audience     []byte
	NeedReceipt  bool
	Status       string
	SentAt       sql.NullTime
	CreatedBy    string
	CreatedAt    time.Time
	CreatorName  sql.NullString
	ReceiptTotal int
	ReceiptDone  int
}

func (r noticeRow) toBiz() biz.Notice {
	out := biz.Notice{
		ID:            r.ID,
		Title:         r.Title,
		Body:          r.Body,
		FileIDs:       decodeStringList(r.FileIDs),
		AudienceType:  r.AudienceType,
		AudienceValue: decodeAudienceValue(r.AudienceType, r.Audience),
		NeedReceipt:   r.NeedReceipt,
		Status:        r.Status,
		CreatedBy:     r.CreatedBy,
		CreatorName:   r.CreatorName.String,
		CreatedAt:     r.CreatedAt.UTC(),
		ReceiptTotal:  r.ReceiptTotal,
		ReceiptDone:   r.ReceiptDone,
	}
	if r.SentAt.Valid {
		t := r.SentAt.Time.UTC()
		out.SentAt = &t
	}
	return out
}

func (r *noticeRepo) ListNotices(ctx context.Context, status string, limit int) ([]biz.Notice, error) {
	query := noticeSelect
	args := []any{}
	if status != "" {
		query += " WHERE n.status = ?"
		args = append(args, status)
	}
	query += " ORDER BY n.created_at DESC LIMIT ?"
	args = append(args, limit)
	var rows []noticeRow
	if err := r.db.WithContext(ctx).Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]biz.Notice, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.toBiz())
	}
	return out, nil
}

func (r *noticeRepo) GetNotice(ctx context.Context, noticeID string) (*biz.Notice, error) {
	var rows []noticeRow
	if err := r.db.WithContext(ctx).Raw(noticeSelect+" WHERE n.id = ?", noticeID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, biz.ErrNoticeNotFound
	}
	notice := rows[0].toBiz()
	return &notice, nil
}

func (r *noticeRepo) CreateNotice(ctx context.Context, notice *biz.Notice) error {
	audience, err := encodeAudience(notice.AudienceType, notice.AudienceValue)
	if err != nil {
		return err
	}
	files, err := encodeStringList(notice.FileIDs)
	if err != nil {
		return err
	}
	res := r.db.WithContext(ctx).Exec(
		`INSERT INTO notices (id, title, body, file_ids, audience, audience_type, need_receipt, status, sent_at, created_by, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?, ?)`,
		notice.ID, notice.Title, notice.Body, files, audience, notice.AudienceType,
		notice.NeedReceipt, notice.Status, notice.CreatedBy, notice.CreatedAt, notice.CreatedAt,
	)
	return res.Error
}

func (r *noticeRepo) UpdateNotice(ctx context.Context, notice *biz.Notice) error {
	audience, err := encodeAudience(notice.AudienceType, notice.AudienceValue)
	if err != nil {
		return err
	}
	files, err := encodeStringList(notice.FileIDs)
	if err != nil {
		return err
	}
	res := r.db.WithContext(ctx).Exec(
		`UPDATE notices SET title = ?, body = ?, file_ids = ?, audience = ?, audience_type = ?, need_receipt = ?
		  WHERE id = ? AND status = 'draft'`,
		notice.Title, notice.Body, files, audience, notice.AudienceType, notice.NeedReceipt, notice.ID,
	)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return biz.ErrNoticeState
	}
	return nil
}

func (r *noticeRepo) DeleteNotice(ctx context.Context, noticeID string) error {
	res := r.db.WithContext(ctx).Exec(`DELETE FROM notices WHERE id = ? AND status = 'draft'`, noticeID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return biz.ErrNoticeNotFound
	}
	return nil
}

// SendNotice 事务内：解析受众 → 建回执 → 写家长消息中心 → 置 sent。
func (r *noticeRepo) SendNotice(ctx context.Context, notice *biz.Notice) (int, error) {
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return 0, tx.Error
	}
	defer func() { _ = tx.Rollback() }()

	guardians, err := resolveAudience(tx, notice)
	if err != nil {
		return 0, err
	}
	if len(guardians) == 0 {
		return 0, biz.ErrNoticeNoAudience
	}
	now := time.Now().UTC()
	created := 0
	for _, guardianID := range guardians {
		res := tx.Exec(
			`INSERT IGNORE INTO notice_receipts (id, notice_id, guardian_id, status, created_at)
			 VALUES (?, ?, ?, 'pending', ?)`,
			biz.NewID(), notice.ID, guardianID, now,
		)
		if res.Error != nil {
			if isFKViolation(res.Error) {
				return 0, biz.ErrNoticeNotFound
			}
			return 0, res.Error
		}
		if res.RowsAffected > 0 {
			created++
		}
		body := notice.Body
		if len([]rune(body)) > 480 {
			body = string([]rune(body)[:480])
		}
		if err := tx.Exec(
			`INSERT INTO notifications (id, guardian_id, student_id, kind, title, body, ref_type, ref_id, created_at)
			 VALUES (?, ?, NULL, 'notice_published', ?, ?, 'notice', ?, ?)`,
			biz.NewID(), guardianID, notice.Title, body, notice.ID, now,
		).Error; err != nil {
			return 0, err
		}
	}
	if err := tx.Exec(
		`UPDATE notices SET status = 'sent', sent_at = ? WHERE id = ? AND status = 'draft'`, now, notice.ID,
	).Error; err != nil {
		return 0, err
	}
	if err := tx.Commit().Error; err != nil {
		return 0, err
	}
	return created, nil
}

type receiptRow struct {
	ID           string
	NoticeID     string
	GuardianID   string
	GuardianName sql.NullString
	Phone        string
	StudentNames sql.NullString
	Status       string
	ConfirmedAt  sql.NullTime
	RemindedAt   sql.NullTime
	CreatedAt    time.Time
}

func (r receiptRow) toBiz() biz.NoticeReceipt {
	out := biz.NoticeReceipt{
		ID:           r.ID,
		NoticeID:     r.NoticeID,
		GuardianID:   r.GuardianID,
		GuardianName: r.GuardianName.String,
		Phone:        r.Phone,
		StudentNames: r.StudentNames.String,
		Status:       r.Status,
		CreatedAt:    r.CreatedAt.UTC(),
	}
	if r.ConfirmedAt.Valid {
		t := r.ConfirmedAt.Time.UTC()
		out.ConfirmedAt = &t
	}
	if r.RemindedAt.Valid {
		t := r.RemindedAt.Time.UTC()
		out.RemindedAt = &t
	}
	return out
}

const receiptSelect = `SELECT r.id, r.notice_id, r.guardian_id, g.name AS guardian_name, g.phone,
       (SELECT GROUP_CONCAT(s.name ORDER BY s.name SEPARATOR '、')
          FROM student_guardians sg JOIN students s ON s.id = sg.student_id
         WHERE sg.guardian_id = r.guardian_id) AS student_names,
       r.status, r.confirmed_at, r.reminded_at, r.created_at
  FROM notice_receipts r
  JOIN guardians g ON g.id = r.guardian_id`

func (r *noticeRepo) ListReceipts(ctx context.Context, noticeID, status string) ([]biz.NoticeReceipt, error) {
	query := receiptSelect + " WHERE r.notice_id = ?"
	args := []any{noticeID}
	if status != "" {
		query += " AND r.status = ?"
		args = append(args, status)
	}
	query += " ORDER BY r.status ASC, r.created_at ASC LIMIT 2000"
	var rows []receiptRow
	if err := r.db.WithContext(ctx).Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]biz.NoticeReceipt, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.toBiz())
	}
	return out, nil
}

func (r *noticeRepo) RemindPending(ctx context.Context, noticeID string) ([]biz.NoticeReceipt, error) {
	var rows []receiptRow
	err := r.db.WithContext(ctx).Raw(receiptSelect+" WHERE r.notice_id = ? AND r.status = 'pending'", noticeID).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	now := time.Now().UTC()
	pending := make([]biz.NoticeReceipt, 0, len(rows))
	for _, row := range rows {
		if err := r.db.WithContext(ctx).Exec(
			`UPDATE notice_receipts SET reminded_at = ? WHERE id = ?`, now, row.ID,
		).Error; err != nil {
			return nil, err
		}
		var title, body string
		if err := r.db.WithContext(ctx).Raw(
			`SELECT title, LEFT(body, 480) FROM notices WHERE id = ?`, noticeID,
		).Row().Scan(&title, &body); err != nil {
			return nil, err
		}
		if err := r.db.WithContext(ctx).Exec(
			`INSERT INTO notifications (id, guardian_id, student_id, kind, title, body, ref_type, ref_id, created_at)
			 VALUES (?, ?, NULL, 'notice_remind', ?, ?, 'notice', ?, ?)`,
			biz.NewID(), row.GuardianID, "催办："+title, body, noticeID, now,
		).Error; err != nil {
			return nil, err
		}
		pending = append(pending, row.toBiz())
	}
	return pending, nil
}

func (r *noticeRepo) ConfirmReceipt(ctx context.Context, noticeID, guardianID string) error {
	res := r.db.WithContext(ctx).Exec(
		`UPDATE notice_receipts SET status = 'confirmed', confirmed_at = ?
		  WHERE notice_id = ? AND guardian_id = ? AND status = 'pending'`,
		time.Now().UTC(), noticeID, guardianID,
	)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		var count int64
		if err := r.db.WithContext(ctx).Raw(
			`SELECT COUNT(*) FROM notice_receipts WHERE notice_id = ? AND guardian_id = ?`, noticeID, guardianID,
		).Scan(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return biz.ErrReceiptNotFound
		}
	}
	return nil
}

func (r *noticeRepo) PortalNotices(ctx context.Context, guardianID string) ([]biz.Notice, error) {
	var rows []noticeRow
	err := r.db.WithContext(ctx).Raw(
		noticeSelect+` JOIN notice_receipts r ON r.notice_id = n.id
		 WHERE r.guardian_id = ? AND n.status = 'sent'
		 ORDER BY n.sent_at DESC LIMIT 100`, guardianID,
	).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]biz.Notice, 0, len(rows))
	for _, row := range rows {
		item := row.toBiz()
		var status string
		var confirmedAt sql.NullTime
		if err := r.db.WithContext(ctx).Raw(
			`SELECT status, confirmed_at FROM notice_receipts WHERE notice_id = ? AND guardian_id = ?`,
			row.ID, guardianID,
		).Row().Scan(&status, &confirmedAt); err == nil {
			item.MyReceiptStatus = status
			item.ReceiptTotal = 1
			if status == biz.ReceiptConfirmed {
				item.ReceiptDone = 1
				if confirmedAt.Valid {
					t := confirmedAt.Time.UTC()
					item.MyConfirmedAt = &t
				}
			}
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *noticeRepo) RenotifyDue(ctx context.Context, before time.Time) ([]biz.NoticeReceipt, error) {
	var rows []receiptRow
	err := r.db.WithContext(ctx).Raw(
		receiptSelect+` JOIN notices n ON n.id = r.notice_id
		 WHERE r.status = 'pending' AND r.renotified_at IS NULL AND r.created_at <= ? AND n.status = 'sent'
		 LIMIT 200`, before,
	).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]biz.NoticeReceipt, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.toBiz())
	}
	return out, nil
}

func (r *noticeRepo) MarkRenotified(ctx context.Context, receiptIDs []string) error {
	if len(receiptIDs) == 0 {
		return nil
	}
	now := time.Now().UTC()
	for _, id := range receiptIDs {
		if err := r.db.WithContext(ctx).Exec(
			`UPDATE notice_receipts SET renotified_at = ? WHERE id = ?`, now, id,
		).Error; err != nil {
			return err
		}
	}
	return nil
}

// resolveAudience 把受众条件展开成家长 id 列表（去重、只保留 active 家长）。
func resolveAudience(tx *gorm.DB, notice *biz.Notice) ([]string, error) {
	base := `SELECT DISTINCT g.id
	           FROM guardians g`
	args := []any{}
	switch notice.AudienceType {
	case biz.AudienceGroup:
		base += ` JOIN student_guardians sg ON sg.guardian_id = g.id
		          JOIN students s ON s.id = sg.student_id`
		base += " WHERE g.status = 'active' AND s.status = 'active' AND s.archived_at IS NULL AND s.group_code = ?"
		args = append(args, notice.AudienceValue)
	case biz.AudienceStudents:
		ids := splitConcat(notice.AudienceValue)
		if len(ids) == 0 {
			return nil, biz.ErrNoticeNoAudience
		}
		base += ` JOIN student_guardians sg ON sg.guardian_id = g.id`
		base += " WHERE g.status = 'active' AND sg.student_id IN (?)"
		args = append(args, ids)
	default:
		base += " WHERE g.status = 'active'"
	}
	base += " ORDER BY g.id"
	var ids []string
	if err := tx.Raw(base, args...).Scan(&ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func encodeAudience(audienceType, value string) (string, error) {
	if audienceType == "" {
		audienceType = biz.AudienceAll
	}
	payload := map[string]string{"type": audienceType, "value": value}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func decodeAudienceValue(audienceType string, raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	payload := map[string]string{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	if audienceType == "" || payload["type"] == audienceType {
		return payload["value"]
	}
	return payload["value"]
}

func encodeStringList(items []string) (string, error) {
	if len(items) == 0 {
		return "", nil
	}
	encoded, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func decodeStringList(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}
