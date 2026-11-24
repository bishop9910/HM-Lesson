package service

import (
	"net/http"

	khttp "github.com/go-kratos/kratos/v3/transport/http"
)

type dashboardDTO struct {
	StudentsTotal      int64 `json:"students_total"`
	SubmissionsPending int64 `json:"submissions_pending"`
	BookingsPending    int64 `json:"bookings_pending"`
	LowBalanceStudents int64 `json:"low_balance_students"`
	StaffTotal         int64 `json:"staff_total"`
}

// DashboardSummary 返回当前机构的真实计数；仅登录老师可读。
func (s *Server) DashboardSummary(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	if s.dashboard == nil {
		return fail(ctx, http.StatusServiceUnavailable, "DASHBOARD_UNAVAILABLE", "汇总服务未就绪")
	}
	summary, err := s.dashboard.Summary(ctx)
	if err != nil {
		return fail(ctx, http.StatusInternalServerError, "INTERNAL", "读取汇总失败")
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"summary": dashboardDTO{
			StudentsTotal:      summary.StudentsTotal,
			SubmissionsPending: summary.SubmissionsPending,
			BookingsPending:    summary.BookingsPending,
			LowBalanceStudents: summary.LowBalanceStudents,
			StaffTotal:         summary.StaffTotal,
		},
	})
}
