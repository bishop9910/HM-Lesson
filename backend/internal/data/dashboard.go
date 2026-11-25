package data

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/bishop9910/hm-lesson/backend/internal/biz"
)

type dashboardRepo struct {
	db  *gorm.DB
	now func() time.Time
}

func NewDashboardRepo(db *gorm.DB) biz.DashboardRepo {
	return &dashboardRepo{db: db, now: time.Now}
}

func (r *dashboardRepo) Summary(ctx context.Context) (biz.DashboardSummary, error) {
	var out biz.DashboardSummary

	// 五个独立 COUNT 合并为一次往返；全部基于真实业务表，没有数据时返回 0。
	row := r.db.WithContext(ctx).Raw(`
SELECT
  (SELECT COUNT(*) FROM students) AS students_total,
  (SELECT COUNT(*) FROM task_assignments WHERE status = 'submitted') AS submissions_pending,
  (SELECT COUNT(*) FROM bookings WHERE status = 'pending') AS bookings_pending,
  (SELECT COUNT(*) FROM lesson_accounts WHERE balance_hours <= 2) AS low_balance_students,
  (SELECT COUNT(*) FROM staff WHERE status = 'active' AND hidden = 0) AS staff_total
`).Row()

	if err := row.Scan(
		&out.StudentsTotal,
		&out.SubmissionsPending,
		&out.BookingsPending,
		&out.LowBalanceStudents,
		&out.StaffTotal,
	); err != nil {
		return biz.DashboardSummary{}, err
	}
	return out, nil
}

// Board 聚合 M9 四组指标（docs/backend-requirements.md §3.8）。
//
// 转化率口径说明：客户分组没有历史快照表，这里用「区间内新增的学员中，当前落在
// class_active/one_on_one 的人数」除以「区间内新增的 intent/visitor 人数」近似，
// 口径随分组变更而漂移，看板只作为运营参考（文档已记录该限制）。
func (r *dashboardRepo) Board(ctx context.Context, from, to time.Time) (biz.Board, error) {
	var out biz.Board
	now := r.now().UTC()

	var rows []struct {
		GroupCode string
		Total     int64
	}
	if err := r.db.WithContext(ctx).Raw(`
SELECT group_code, COUNT(*) AS total
FROM students
WHERE archived_at IS NULL
GROUP BY group_code
`).Scan(&rows).Error; err != nil {
		return biz.Board{}, err
	}
	byGroup := make(map[string]int64, len(rows))
	for _, item := range rows {
		byGroup[item.GroupCode] = item.Total
	}
	for _, code := range []string{biz.GroupClassActive, biz.GroupOneOnOne, biz.GroupIntent, biz.GroupVisitor} {
		out.Groups = append(out.Groups, biz.BoardGroupCount{
			Code:  code,
			Name:  biz.BoardGroupName(code),
			Count: byGroup[code],
		})
	}

	var head struct {
		NewStudents int64
		NewLeads    int64
		Converted   int64
	}
	if err := r.db.WithContext(ctx).Raw(`
SELECT
  COUNT(*) AS new_students,
  SUM(CASE WHEN group_code IN ('intent','visitor') THEN 1 ELSE 0 END) AS new_leads,
  SUM(CASE WHEN group_code IN ('class_active','one_on_one') THEN 1 ELSE 0 END) AS converted
FROM students
WHERE archived_at IS NULL AND created_at >= ? AND created_at < ?
`, from, to).Scan(&head).Error; err != nil {
		return biz.Board{}, err
	}
	out.NewStudents = head.NewStudents
	out.NewLeads = head.NewLeads
	out.Converted = head.Converted
	if head.NewLeads > 0 {
		out.ConversionRate = float64(head.Converted) / float64(head.NewLeads)
	}

	var delivery struct {
		Tasks         int64
		Assignments   int64
		Submissions   int64
		Overdue       int64
		PendingReview int64
	}
	if err := r.db.WithContext(ctx).Raw(`
SELECT
  (SELECT COUNT(*) FROM tasks WHERE created_at >= ? AND created_at < ?) AS tasks,
  (SELECT COUNT(*) FROM task_assignments ta JOIN tasks t ON t.id = ta.task_id
     WHERE t.created_at >= ? AND t.created_at < ?) AS assignments,
  (SELECT COUNT(*) FROM task_assignments ta JOIN tasks t ON t.id = ta.task_id
     WHERE t.created_at >= ? AND t.created_at < ? AND ta.status <> 'pending') AS submissions,
  (SELECT COUNT(*) FROM task_assignments ta JOIN tasks t ON t.id = ta.task_id
     WHERE t.created_at >= ? AND t.created_at < ? AND ta.status = 'pending' AND t.due_at < ?) AS overdue,
  (SELECT COUNT(*) FROM task_assignments ta JOIN tasks t ON t.id = ta.task_id
     WHERE t.created_at >= ? AND t.created_at < ? AND ta.status = 'submitted') AS pending_review
`, from, to, from, to, from, to, from, to, now, from, to).Scan(&delivery).Error; err != nil {
		return biz.Board{}, err
	}
	out.Delivery = biz.BoardDelivery{
		Tasks:         delivery.Tasks,
		Assignments:   delivery.Assignments,
		Submissions:   delivery.Submissions,
		Overdue:       delivery.Overdue,
		PendingReview: delivery.PendingReview,
	}
	if delivery.Assignments > 0 {
		out.Delivery.SubmitRate = float64(delivery.Submissions) / float64(delivery.Assignments)
	}

	// 本周（周一 00:00 起 7 天）与本月（自然月）固定区间，与 from/to 无关。
	weekStart := startOfWeek(now)
	weekEnd := weekStart.AddDate(0, 0, 7)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0)

	var oneOnOne struct {
		PendingBookings int64
		WeekSessions    int64
		MonthConsumed   float64
	}
	if err := r.db.WithContext(ctx).Raw(`
SELECT
  (SELECT COUNT(*) FROM bookings WHERE status = 'pending') AS pending_bookings,
  (SELECT COUNT(*) FROM bookings b JOIN slots s ON s.id = b.slot_id
     WHERE b.status = 'approved' AND s.start_at >= ? AND s.start_at < ?) AS week_sessions,
  (SELECT COALESCE(SUM(-delta_hours), 0) FROM lesson_ledger
     WHERE type = 'consume' AND occurred_at >= ? AND occurred_at < ?) AS month_consumed
`, weekStart, weekEnd, monthStart, monthEnd).Scan(&oneOnOne).Error; err != nil {
		return biz.Board{}, err
	}
	out.OneOnOne = biz.BoardOneOnOne{
		PendingBookings: oneOnOne.PendingBookings,
		WeekSessions:    oneOnOne.WeekSessions,
		MonthConsumed:   oneOnOne.MonthConsumed,
	}

	var todo struct {
		FollowUpsDue    int64
		OverdueApproved int64
		Unconfirmed     int64
		LowBalance      int64
	}
	if err := r.db.WithContext(ctx).Raw(`
SELECT
  (SELECT COUNT(*) FROM students
     WHERE archived_at IS NULL AND next_follow_up_at IS NOT NULL AND next_follow_up_at <= ?) AS follow_ups_due,
  (SELECT COUNT(*) FROM bookings b JOIN slots s ON s.id = b.slot_id
     WHERE b.status = 'approved' AND s.end_at < ?) AS overdue_approved,
  (SELECT COUNT(*) FROM notice_receipts nr JOIN notices n ON n.id = nr.notice_id
     WHERE nr.status = 'pending' AND n.status = 'sent' AND n.need_receipt = 1) AS unconfirmed,
  (SELECT COUNT(*) FROM lesson_accounts a JOIN students s ON s.id = a.student_id
     WHERE s.archived_at IS NULL AND a.balance_hours <= 2) AS low_balance
`, now, now).Scan(&todo).Error; err != nil {
		return biz.Board{}, err
	}
	out.Todo = biz.BoardTodo{
		FollowUpsDue:    todo.FollowUpsDue,
		OverdueApproved: todo.OverdueApproved,
		Unconfirmed:     todo.Unconfirmed,
		LowBalance:      todo.LowBalance,
	}
	return out, nil
}

func startOfWeek(now time.Time) time.Time {
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	offset := (int(day.Weekday()) + 6) % 7
	return day.AddDate(0, 0, -offset)
}
