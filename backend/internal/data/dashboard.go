package data

import (
	"context"

	"gorm.io/gorm"

	"github.com/bishop9910/hm-lesson/backend/internal/biz"
)

type dashboardRepo struct {
	db *gorm.DB
}

func NewDashboardRepo(db *gorm.DB) biz.DashboardRepo {
	return &dashboardRepo{db: db}
}

func (r *dashboardRepo) Summary(ctx context.Context) (biz.DashboardSummary, error) {
	var out biz.DashboardSummary

	// 五个独立 COUNT 合并为一次往返；全部基于真实业务表，没有数据时返回 0。
	row := r.db.WithContext(ctx).Raw(`
SELECT
  (SELECT COUNT(*) FROM students) AS students_total,
  (SELECT COUNT(*) FROM submissions WHERE status = 'submitted') AS submissions_pending,
  (SELECT COUNT(*) FROM bookings WHERE status = 'pending') AS bookings_pending,
  (SELECT COUNT(*) FROM lesson_accounts WHERE balance_hours <= 2) AS low_balance_students,
  (SELECT COUNT(*) FROM staff WHERE status = 'active') AS staff_total
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
