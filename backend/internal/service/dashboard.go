package service

import (
	"net/http"
	"time"

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

type boardGroupDTO struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type boardDeliveryDTO struct {
	Tasks         int64   `json:"tasks"`
	Assignments   int64   `json:"assignments"`
	Submissions   int64   `json:"submissions"`
	Overdue       int64   `json:"overdue"`
	PendingReview int64   `json:"pending_review"`
	SubmitRate    float64 `json:"submit_rate"`
}

type boardOneOnOneDTO struct {
	PendingBookings int64   `json:"pending_bookings"`
	WeekSessions    int64   `json:"week_sessions"`
	MonthConsumed   float64 `json:"month_consumed"`
}

type boardTodoDTO struct {
	FollowUpsDue    int64 `json:"follow_ups_due"`
	OverdueApproved int64 `json:"overdue_approved"`
	Unconfirmed     int64 `json:"unconfirmed"`
	LowBalance      int64 `json:"low_balance"`
}

type boardDTO struct {
	Groups         []boardGroupDTO  `json:"groups"`
	NewStudents    int64            `json:"new_students"`
	NewLeads       int64            `json:"new_leads"`
	Converted      int64            `json:"converted"`
	ConversionRate float64          `json:"conversion_rate"`
	Delivery       boardDeliveryDTO `json:"delivery"`
	OneOnOne       boardOneOnOneDTO `json:"one_on_one"`
	Todo           boardTodoDTO     `json:"todo"`
	From           string           `json:"from"`
	To             string           `json:"to"`
	GeneratedAt    string           `json:"generated_at"`
}

// DashboardBoard 返回 M9 看板四组指标；默认区间为本月自然月。
func (s *Server) DashboardBoard(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	if s.dashboard == nil {
		return fail(ctx, http.StatusServiceUnavailable, "DASHBOARD_UNAVAILABLE", "汇总服务未就绪")
	}
	now := time.Now().UTC()
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)
	if raw := ctx.Query().Get("from"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return fail(ctx, http.StatusBadRequest, "VALIDATION_FAILED", "from 必须是 RFC3339 时间")
		}
		from = parsed.UTC()
	}
	if raw := ctx.Query().Get("to"); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return fail(ctx, http.StatusBadRequest, "VALIDATION_FAILED", "to 必须是 RFC3339 时间")
		}
		to = parsed.UTC()
	}
	if !to.After(from) {
		return fail(ctx, http.StatusBadRequest, "VALIDATION_FAILED", "to 必须晚于 from")
	}
	board, err := s.dashboard.Board(ctx, from, to)
	if err != nil {
		return fail(ctx, http.StatusInternalServerError, "INTERNAL", "读取看板失败")
	}
	groups := make([]boardGroupDTO, 0, len(board.Groups))
	for _, group := range board.Groups {
		groups = append(groups, boardGroupDTO{Code: group.Code, Name: group.Name, Count: group.Count})
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"board": boardDTO{
			Groups:         groups,
			NewStudents:    board.NewStudents,
			NewLeads:       board.NewLeads,
			Converted:      board.Converted,
			ConversionRate: board.ConversionRate,
			Delivery: boardDeliveryDTO{
				Tasks:         board.Delivery.Tasks,
				Assignments:   board.Delivery.Assignments,
				Submissions:   board.Delivery.Submissions,
				Overdue:       board.Delivery.Overdue,
				PendingReview: board.Delivery.PendingReview,
				SubmitRate:    board.Delivery.SubmitRate,
			},
			OneOnOne: boardOneOnOneDTO{
				PendingBookings: board.OneOnOne.PendingBookings,
				WeekSessions:    board.OneOnOne.WeekSessions,
				MonthConsumed:   board.OneOnOne.MonthConsumed,
			},
			Todo: boardTodoDTO{
				FollowUpsDue:    board.Todo.FollowUpsDue,
				OverdueApproved: board.Todo.OverdueApproved,
				Unconfirmed:     board.Todo.Unconfirmed,
				LowBalance:      board.Todo.LowBalance,
			},
			From:        board.From.Format("2006-01-02T15:04:05Z"),
			To:          board.To.Format("2006-01-02T15:04:05Z"),
			GeneratedAt: board.GeneratedAt.Format("2006-01-02T15:04:05Z"),
		},
	})
}
