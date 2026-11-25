package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	"github.com/bishop9910/hm-lesson/backend/internal/biz"
)

// bookingRepo 实现时段与预约（M3 一对一预约排课）。
type bookingRepo struct {
	db *gorm.DB
}

func NewBookingRepo(db *gorm.DB) biz.BookingRepo {
	return &bookingRepo{db: db}
}

const slotSelect = `
SELECT s.id, s.staff_id, st.name, s.start_at, s.end_at, s.status, s.note,
       COALESCE(b.id, ''), COALESCE(stu.name, ''), COALESCE(b.status, '')
FROM slots s
JOIN staff st ON st.id = s.staff_id
LEFT JOIN bookings b ON b.slot_id = s.id AND b.active_flag = 1
LEFT JOIN students stu ON stu.id = b.student_id
`

func (r *bookingRepo) ListSlots(ctx context.Context, from, to *time.Time) ([]biz.Slot, error) {
	query := slotSelect + ` WHERE s.status <> 'cancelled'`
	args := []any{}
	if from != nil {
		query += ` AND s.start_at >= ?`
		args = append(args, from.UTC())
	}
	if to != nil {
		query += ` AND s.start_at < ?`
		args = append(args, to.UTC())
	}
	query += ` ORDER BY s.start_at LIMIT 500`

	rows, err := r.db.WithContext(ctx).Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	slots := make([]biz.Slot, 0, 16)
	for rows.Next() {
		var s biz.Slot
		if err := rows.Scan(&s.ID, &s.StaffID, &s.StaffName, &s.StartAt, &s.EndAt, &s.Status, &s.Note,
			&s.BookingID, &s.StudentName, &s.BookingStatus); err != nil {
			return nil, err
		}
		slots = append(slots, s)
	}
	return slots, rows.Err()
}

// CreateSlot 在事务内按老师加锁，避免同一老师的时段重叠。
func (r *bookingRepo) CreateSlot(ctx context.Context, slot *biz.Slot) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockStaff(tx, slot.StaffID); err != nil {
			return err
		}
		if err := ensureNoOverlap(tx, slot.StaffID, "", slot.StartAt, slot.EndAt); err != nil {
			return err
		}
		if slot.ID == "" {
			slot.ID = biz.NewID()
		}
		slot.Status = biz.SlotOpen
		return tx.Exec(`INSERT INTO slots (id, staff_id, start_at, end_at, status, note)
			VALUES (?, ?, ?, ?, ?, ?)`,
			slot.ID, slot.StaffID, slot.StartAt.UTC(), slot.EndAt.UTC(), biz.SlotOpen, slot.Note).Error
	})
}

// BatchCreateSlots 按周模板生成时段；已过期的候选时间点直接跳过。
func (r *bookingRepo) BatchCreateSlots(ctx context.Context, staffID string, startAt, endAt time.Time, weekdays []int, weeks int) (int, error) {
	created := 0
	hour, minute, sec := startAt.Clock()
	now := time.Now().UTC()
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockStaff(tx, staffID); err != nil {
			return err
		}
		// 以 startAt 所在自然周的周一为基准。
		base := time.Date(startAt.Year(), startAt.Month(), startAt.Day(), hour, minute, sec, 0, startAt.Location())
		iso := int(base.Weekday())
		if iso == 0 {
			iso = 7
		}
		monday := base.AddDate(0, 0, -(iso - 1))
		for w := 0; w < weeks; w++ {
			for _, wd := range weekdays {
				day := monday.AddDate(0, 0, 7*w+(wd-1))
				start := time.Date(day.Year(), day.Month(), day.Day(), hour, minute, sec, 0, day.Location())
				end := start.Add(endAt.Sub(startAt))
				if !start.After(now) {
					continue
				}
				if err := ensureNoOverlap(tx, staffID, "", start, end); err != nil {
					if errors.Is(err, biz.ErrSlotOverlap) {
						continue
					}
					return err
				}
				id := biz.NewID()
				if err := tx.Exec(`INSERT INTO slots (id, staff_id, start_at, end_at, status, note)
					VALUES (?, ?, ?, ?, ?, '')`, id, staffID, start.UTC(), end.UTC(), biz.SlotOpen).Error; err != nil {
					return err
				}
				created++
			}
		}
		return nil
	})
	return created, err
}

func (r *bookingRepo) UpdateSlot(ctx context.Context, slotID string, startAt, endAt time.Time, note string) (*biz.Slot, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var staffID, status string
		row := tx.Raw(`SELECT staff_id, status FROM slots WHERE id = ? FOR UPDATE`, slotID).Row()
		if err := row.Scan(&staffID, &status); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return biz.ErrSlotNotFound
			}
			return err
		}
		if status != biz.SlotOpen {
			return biz.ErrSlotState
		}
		if err := ensureNoOverlap(tx, staffID, slotID, startAt, endAt); err != nil {
			return err
		}
		return tx.Exec(`UPDATE slots SET start_at = ?, end_at = ?, note = ? WHERE id = ?`,
			startAt.UTC(), endAt.UTC(), note, slotID).Error
	})
	if err != nil {
		return nil, err
	}
	return r.getSlot(ctx, slotID)
}

func (r *bookingRepo) CancelSlot(ctx context.Context, slotID string) error {
	res := r.db.WithContext(ctx).Exec(`UPDATE slots SET status = ? WHERE id = ? AND status = ?`,
		biz.SlotCancelled, slotID, biz.SlotOpen)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		var exists int64
		if err := r.db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM slots WHERE id = ?`, slotID).Scan(&exists).Error; err != nil {
			return err
		}
		if exists == 0 {
			return biz.ErrSlotNotFound
		}
		return biz.ErrSlotState
	}
	return nil
}

func (r *bookingRepo) getSlot(ctx context.Context, slotID string) (*biz.Slot, error) {
	var s biz.Slot
	err := r.db.WithContext(ctx).Raw(slotSelect+` WHERE s.id = ?`, slotID).Row().
		Scan(&s.ID, &s.StaffID, &s.StaffName, &s.StartAt, &s.EndAt, &s.Status, &s.Note,
			&s.BookingID, &s.StudentName, &s.BookingStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, biz.ErrSlotNotFound
		}
		return nil, err
	}
	return &s, nil
}

const bookingSelect = `
SELECT b.id, b.slot_id, b.student_id, stu.name, COALESCE(stu.grade, ''), b.guardian_id, g.name,
       b.status, b.request_note, b.review_note, b.cancel_reason, COALESCE(b.attendance, ''),
       COALESCE(b.lesson_ledger_id, ''), COALESCE(b.reviewed_by, ''), COALESCE(stf.name, ''),
       s.start_at, s.end_at, b.submitted_at, b.reviewed_at, b.completed_at
FROM bookings b
JOIN slots s ON s.id = b.slot_id
JOIN students stu ON stu.id = b.student_id
JOIN guardians g ON g.id = b.guardian_id
LEFT JOIN staff stf ON stf.id = b.reviewed_by
`

func (r *bookingRepo) ListBookings(ctx context.Context, status string, from, to *time.Time) ([]biz.Booking, error) {
	query := bookingSelect + ` WHERE 1 = 1`
	args := []any{}
	if status != "" {
		query += ` AND b.status = ?`
		args = append(args, status)
	}
	if from != nil {
		query += ` AND s.start_at >= ?`
		args = append(args, from.UTC())
	}
	if to != nil {
		query += ` AND s.start_at < ?`
		args = append(args, to.UTC())
	}
	query += ` ORDER BY b.submitted_at DESC LIMIT 300`
	return r.scanBookings(ctx, query, args...)
}

func (r *bookingRepo) PortalSlots(ctx context.Context, guardianID string, now time.Time) ([]biz.Slot, error) {
	query := slotSelect + `
WHERE (s.status = 'open' AND s.start_at > ?)
   OR (b.guardian_id = ? AND b.status IN ('pending', 'approved'))
ORDER BY s.start_at LIMIT 200`
	rows, err := r.db.WithContext(ctx).Raw(query, now.UTC(), guardianID).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	slots := make([]biz.Slot, 0, 16)
	for rows.Next() {
		var s biz.Slot
		if err := rows.Scan(&s.ID, &s.StaffID, &s.StaffName, &s.StartAt, &s.EndAt, &s.Status, &s.Note,
			&s.BookingID, &s.StudentName, &s.BookingStatus); err != nil {
			return nil, err
		}
		slots = append(slots, s)
	}
	return slots, rows.Err()
}

func (r *bookingRepo) PortalBookings(ctx context.Context, guardianID string, now time.Time) ([]biz.Booking, error) {
	return r.scanBookings(ctx, bookingSelect+` WHERE b.guardian_id = ? ORDER BY s.start_at DESC LIMIT 100`, guardianID)
}

// CreateBooking 提交即锁时段：事务内校验归属、余额、时段与重叠，靠唯一索引做最后防线。
func (r *bookingRepo) CreateBooking(ctx context.Context, booking *biz.Booking, minBalance float64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var slotStatus string
		var startAt, endAt time.Time
		row := tx.Raw(`SELECT status, start_at, end_at FROM slots WHERE id = ? FOR UPDATE`, booking.SlotID).Row()
		if err := row.Scan(&slotStatus, &startAt, &endAt); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return biz.ErrSlotNotFound
			}
			return err
		}
		if slotStatus != biz.SlotOpen {
			return biz.ErrSlotTaken
		}
		if !startAt.After(time.Now().UTC()) {
			return fmt.Errorf("%w: 时段已开始，不能再预约", biz.ErrSlotState)
		}
		if err := ensureStudentAccess(tx, booking.StudentID, booking.GuardianID); err != nil {
			return err
		}
		if err := ensureBalance(tx, booking.StudentID, minBalance); err != nil {
			return err
		}
		var overlap int64
		if err := tx.Raw(`SELECT COUNT(*) FROM bookings b JOIN slots s ON s.id = b.slot_id
			WHERE b.student_id = ? AND b.active_flag = 1 AND s.start_at < ? AND s.end_at > ?`,
			booking.StudentID, endAt, startAt).Scan(&overlap).Error; err != nil {
			return err
		}
		if overlap > 0 {
			return fmt.Errorf("%w: 该学员在同一时间段已有预约", biz.ErrBookingState)
		}
		res := tx.Exec(`UPDATE slots SET status = ? WHERE id = ? AND status = ?`,
			biz.SlotLocked, booking.SlotID, biz.SlotOpen)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return biz.ErrSlotTaken
		}
		booking.ID = biz.NewID()
		booking.SubmittedAt = time.Now().UTC()
		err := tx.Exec(`INSERT INTO bookings (id, slot_id, student_id, guardian_id, status, request_note, submitted_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			booking.ID, booking.SlotID, booking.StudentID, booking.GuardianID, biz.BookingPending,
			booking.RequestNote, booking.SubmittedAt).Error
		if err != nil {
			if isDuplicate(err) {
				return biz.ErrSlotTaken
			}
			return err
		}
		return nil
	})
}

func (r *bookingRepo) ApproveBooking(ctx context.Context, bookingID, staffID, note string) error {
	return r.withBookingTx(ctx, bookingID, func(tx *gorm.DB, b *bookingRow) error {
		if b.status != biz.BookingPending {
			return biz.ErrBookingState
		}
		return tx.Exec(`UPDATE bookings SET status = ?, review_note = ?, reviewed_by = ?, reviewed_at = NOW(3)
			WHERE id = ?`, biz.BookingApproved, note, staffID, bookingID).Error
	})
}

func (r *bookingRepo) RejectBooking(ctx context.Context, bookingID, staffID, reason string) error {
	return r.withBookingTx(ctx, bookingID, func(tx *gorm.DB, b *bookingRow) error {
		if b.status != biz.BookingPending {
			return biz.ErrBookingState
		}
		if err := tx.Exec(`UPDATE bookings SET status = ?, review_note = ?, reviewed_by = ?, reviewed_at = NOW(3)
			WHERE id = ?`, biz.BookingRejected, reason, staffID, bookingID).Error; err != nil {
			return err
		}
		return tx.Exec(`UPDATE slots SET status = ? WHERE id = ? AND status = ?`,
			biz.SlotOpen, b.slotID, biz.SlotLocked).Error
	})
}

// CompleteBooking 销课：同一事务内写 consume 流水、更新账户缓存、回写预约与时段。
func (r *bookingRepo) CompleteBooking(ctx context.Context, bookingID, staffID string, hours float64) (string, error) {
	ledgerID := ""
	err := r.withBookingTx(ctx, bookingID, func(tx *gorm.DB, b *bookingRow) error {
		if b.status == biz.BookingCompleted {
			ledgerID = b.lessonLedgerID
			return nil
		}
		if b.status != biz.BookingApproved {
			return biz.ErrBookingState
		}
		if hours <= 0 {
			hours = 1
		}
		newLedger, err := consumeHours(tx, b.studentID, bookingID, staffID, hours)
		if err != nil {
			return err
		}
		ledgerID = newLedger
		if err := tx.Exec(`UPDATE bookings SET status = ?, attendance = ?, completed_at = NOW(3),
			lesson_ledger_id = ?, reviewed_by = COALESCE(reviewed_by, ?)
			WHERE id = ?`, biz.BookingCompleted, biz.AttendanceAttended, newLedger, staffID, bookingID).Error; err != nil {
			return err
		}
		return tx.Exec(`UPDATE slots SET status = ? WHERE id = ?`, biz.SlotCompleted, b.slotID).Error
	})
	if err != nil {
		return "", err
	}
	return ledgerID, nil
}

func (r *bookingRepo) NoShowBooking(ctx context.Context, bookingID, staffID, note string) error {
	return r.withBookingTx(ctx, bookingID, func(tx *gorm.DB, b *bookingRow) error {
		if b.status != biz.BookingApproved {
			return biz.ErrBookingState
		}
		if err := tx.Exec(`UPDATE bookings SET status = ?, attendance = ?, review_note = ?, completed_at = NOW(3),
			reviewed_by = COALESCE(reviewed_by, ?) WHERE id = ?`,
			biz.BookingNoShow, biz.AttendanceNoShow, note, staffID, bookingID).Error; err != nil {
			return err
		}
		return tx.Exec(`UPDATE slots SET status = ? WHERE id = ?`, biz.SlotCompleted, b.slotID).Error
	})
}

func (r *bookingRepo) CancelBooking(ctx context.Context, guardianID, bookingID, reason string) error {
	return r.withBookingTx(ctx, bookingID, func(tx *gorm.DB, b *bookingRow) error {
		if b.guardianID != guardianID {
			return biz.ErrBookingNotFound
		}
		if b.status != biz.BookingPending {
			return biz.ErrBookingState
		}
		if err := tx.Exec(`UPDATE bookings SET status = ?, cancel_reason = ?, updated_at = NOW(3)
			WHERE id = ?`, biz.BookingCancelled, reason, bookingID).Error; err != nil {
			return err
		}
		return tx.Exec(`UPDATE slots SET status = ? WHERE id = ? AND status = ?`,
			biz.SlotOpen, b.slotID, biz.SlotLocked).Error
	})
}

type bookingRow struct {
	status         string
	slotID         string
	studentID      string
	guardianID     string
	lessonLedgerID string
}

func (r *bookingRepo) withBookingTx(ctx context.Context, bookingID string, fn func(tx *gorm.DB, b *bookingRow) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var b bookingRow
		var ledger sql.NullString
		row := tx.Raw(`SELECT status, slot_id, student_id, guardian_id, lesson_ledger_id
			FROM bookings WHERE id = ? FOR UPDATE`, bookingID).Row()
		if err := row.Scan(&b.status, &b.slotID, &b.studentID, &b.guardianID, &ledger); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return biz.ErrBookingNotFound
			}
			return err
		}
		b.lessonLedgerID = ledger.String
		return fn(tx, &b)
	})
}

func (r *bookingRepo) scanBookings(ctx context.Context, query string, args ...any) ([]biz.Booking, error) {
	rows, err := r.db.WithContext(ctx).Raw(query, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bookings := make([]biz.Booking, 0, 16)
	for rows.Next() {
		var b biz.Booking
		var reviewedAt, completedAt sql.NullTime
		if err := rows.Scan(&b.ID, &b.SlotID, &b.StudentID, &b.StudentName, &b.Grade, &b.GuardianID, &b.GuardianName,
			&b.Status, &b.RequestNote, &b.ReviewNote, &b.CancelReason, &b.Attendance,
			&b.LessonLedgerID, &b.ReviewedBy, &b.ReviewerName,
			&b.StartAt, &b.EndAt, &b.SubmittedAt, &reviewedAt, &completedAt); err != nil {
			return nil, err
		}
		if reviewedAt.Valid {
			t := reviewedAt.Time
			b.ReviewedAt = &t
		}
		if completedAt.Valid {
			t := completedAt.Time
			b.CompletedAt = &t
		}
		bookings = append(bookings, b)
	}
	return bookings, rows.Err()
}

func lockStaff(tx *gorm.DB, staffID string) error {
	var id string
	err := tx.Raw(`SELECT id FROM staff WHERE id = ? FOR UPDATE`, staffID).Row().Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return biz.ErrForbidden
	}
	return err
}

func ensureNoOverlap(tx *gorm.DB, staffID, excludeSlotID string, startAt, endAt time.Time) error {
	query := `SELECT COUNT(*) FROM slots WHERE staff_id = ? AND status <> 'cancelled'
		AND start_at < ? AND end_at > ?`
	args := []any{staffID, endAt.UTC(), startAt.UTC()}
	if excludeSlotID != "" {
		query += ` AND id <> ?`
		args = append(args, excludeSlotID)
	}
	var count int64
	if err := tx.Raw(query, args...).Scan(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return biz.ErrSlotOverlap
	}
	return nil
}

func ensureStudentAccess(tx *gorm.DB, studentID, guardianID string) error {
	var count int64
	if err := tx.Raw(`SELECT COUNT(*) FROM student_guardians sg
		JOIN guardians g ON g.id = sg.guardian_id
		JOIN students s ON s.id = sg.student_id
		WHERE sg.student_id = ? AND sg.guardian_id = ? AND g.status = 'active'
		  AND s.status = 'active' AND s.archived_at IS NULL`,
		studentID, guardianID).Scan(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		// 越权与不存在统一返回 forbidden，避免泄露存在性。
		return biz.ErrForbidden
	}
	return nil
}

// ensureBalance 校验课时余额与有效期（余额 ≤0 禁止预约，需求 §3.3.2 / AF-13）。
func ensureBalance(tx *gorm.DB, studentID string, minBalance float64) error {
	var balance float64
	var status string
	var validUntil sql.NullTime
	err := tx.Raw(`SELECT balance_hours, status, valid_until FROM lesson_accounts WHERE student_id = ?`,
		studentID).Row().Scan(&balance, &status, &validUntil)
	if errors.Is(err, sql.ErrNoRows) {
		return biz.ErrInsufficientHours
	}
	if err != nil {
		return err
	}
	if status == biz.LessonExpired {
		return biz.ErrInsufficientHours
	}
	if validUntil.Valid && validUntil.Time.Before(time.Now()) {
		return biz.ErrInsufficientHours
	}
	if balance < minBalance {
		return biz.ErrInsufficientHours
	}
	return nil
}

// consumeHours 在事务内锁定账户并追加 consume 流水，返回流水 ID。
func consumeHours(tx *gorm.DB, studentID, bookingID, operatorID string, hours float64) (string, error) {
	var balance float64
	err := tx.Raw(`SELECT balance_hours FROM lesson_accounts WHERE student_id = ? FOR UPDATE`, studentID).
		Row().Scan(&balance)
	if errors.Is(err, sql.ErrNoRows) {
		return "", biz.ErrInsufficientHours
	}
	if err != nil {
		return "", err
	}
	if balance < hours {
		return "", biz.ErrInsufficientHours
	}
	balanceAfter := roundHours(balance - hours)
	ledgerID := biz.NewID()
	if err := tx.Exec(`INSERT INTO lesson_ledger (id, student_id, type, delta_hours, balance_after, ref_type, ref_id, reason, operator_id)
		VALUES (?, ?, ?, ?, ?, 'booking', ?, ?, ?)`,
		ledgerID, studentID, biz.LedgerConsume, -hours, balanceAfter, bookingID, "预约消课", operatorID).Error; err != nil {
		if isDuplicate(err) {
			// 同一预约已经扣过：返回已存在的流水。
			var existing string
			if qerr := tx.Raw(`SELECT id FROM lesson_ledger WHERE ref_type = 'booking' AND ref_id = ? AND type = 'consume'`,
				bookingID).Row().Scan(&existing); qerr == nil {
				return existing, nil
			}
		}
		return "", err
	}
	return ledgerID, tx.Exec(`UPDATE lesson_accounts SET used_hours = used_hours + ?, balance_hours = ?,
		status = CASE WHEN ? <= 0 THEN 'exhausted' ELSE 'active' END, updated_at = NOW(3)
		WHERE student_id = ?`, hours, balanceAfter, balanceAfter, studentID).Error
}

func isDuplicate(err error) bool {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1062
	}
	return false
}

// isFKViolation 判断外键约束失败（1452），用于把“学员不存在”映射成领域错误。
func isFKViolation(err error) bool {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1452
	}
	return false
}

func roundHours(v float64) float64 {
	return math.Round(v*100) / 100
}
