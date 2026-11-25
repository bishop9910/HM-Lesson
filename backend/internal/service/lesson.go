package service

import (
	"errors"
	"net/http"
	"strings"

	khttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/bishop9910/hm-lesson/backend/internal/biz"
)

// 课时台账接口（M4）：教师端 /api/v1/lesson-accounts、/api/v1/lesson-ledger，家长端 /api/v1/portal/lesson-account。
type lessonAccountDTO struct {
	StudentID      string  `json:"student_id"`
	StudentName    string  `json:"student_name"`
	Grade          string  `json:"grade"`
	PurchasedHours float64 `json:"purchased_hours"`
	UsedHours      float64 `json:"used_hours"`
	BalanceHours   float64 `json:"balance_hours"`
	Status         string  `json:"status"`
	LowBalance     bool    `json:"low_balance"`
	ValidUntil     *string `json:"valid_until"`
	UpdatedAt      string  `json:"updated_at"`
}

type ledgerDTO struct {
	ID           string  `json:"id"`
	StudentID    string  `json:"student_id"`
	Type         string  `json:"type"`
	DeltaHours   float64 `json:"delta_hours"`
	BalanceAfter float64 `json:"balance_after"`
	RefType      string  `json:"ref_type"`
	RefID        string  `json:"ref_id"`
	Reason       string  `json:"reason"`
	OperatorName string  `json:"operator_name"`
	OccurredAt   string  `json:"occurred_at"`
}

func toLessonAccountDTO(a biz.LessonAccount) lessonAccountDTO {
	var validUntil *string
	if a.ValidUntil != nil {
		v := a.ValidUntil.UTC().Format("2006-01-02")
		validUntil = &v
	}
	return lessonAccountDTO{
		StudentID:      a.StudentID,
		StudentName:    a.StudentName,
		Grade:          a.Grade,
		PurchasedHours: a.PurchasedHours,
		UsedHours:      a.UsedHours,
		BalanceHours:   a.BalanceHours,
		Status:         a.Status,
		LowBalance:     a.LowBalance(),
		ValidUntil:     validUntil,
		UpdatedAt:      a.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func toLedgerDTO(e biz.LedgerEntry) ledgerDTO {
	return ledgerDTO{
		ID:           e.ID,
		StudentID:    e.StudentID,
		Type:         e.Type,
		DeltaHours:   e.DeltaHours,
		BalanceAfter: e.BalanceAfter,
		RefType:      e.RefType,
		RefID:        e.RefID,
		Reason:       e.Reason,
		OperatorName: e.OperatorName,
		OccurredAt:   e.OccurredAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func (s *Server) requireLessons(ctx khttp.Context) (*biz.LessonUsecase, error) {
	if s.lessons == nil {
		return nil, fail(ctx, http.StatusServiceUnavailable, "LESSON_DISABLED", "数据库未配置")
	}
	return s.lessons, nil
}

// ListLessonAccounts 教师端课时列表（按余额升序，便于发现低余额）。
func (s *Server) ListLessonAccounts(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	uc, err := s.requireLessons(ctx)
	if err != nil {
		return err
	}
	items, err := uc.List(ctx)
	if err != nil {
		return mapLessonError(ctx, err)
	}
	out := make([]lessonAccountDTO, 0, len(items))
	for _, item := range items {
		out = append(out, toLessonAccountDTO(item))
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": out, "threshold": biz.LowBalanceThreshold})
}

// GetLessonAccount 学员课时详情 + 最近流水。
func (s *Server) GetLessonAccount(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	uc, err := s.requireLessons(ctx)
	if err != nil {
		return err
	}
	account, entries, err := uc.Detail(ctx, ctx.Vars().Get("studentID"))
	if err != nil {
		return mapLessonError(ctx, err)
	}
	out := make([]ledgerDTO, 0, len(entries))
	for _, entry := range entries {
		out = append(out, toLedgerDTO(entry))
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"account": toLessonAccountDTO(*account),
		"ledger":  out,
	})
}

type lessonHoursRequest struct {
	Hours      float64 `json:"hours"`
	DeltaHours float64 `json:"delta_hours"`
	Reason     string  `json:"reason"`
}

// PurchaseLessonHours 充值课时（写 purchase 流水）。
func (s *Server) PurchaseLessonHours(ctx khttp.Context) error {
	staff, err := s.requireStaff(ctx, true)
	if err != nil {
		return err
	}
	uc, err := s.requireLessons(ctx)
	if err != nil {
		return err
	}
	var req lessonHoursRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "INVALID_BODY", "请求体格式错误")
	}
	account, err := uc.Purchase(ctx, ctx.Vars().Get("studentID"), staff.ID, req.Hours, req.Reason)
	if err != nil {
		return mapLessonError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"account": toLessonAccountDTO(*account)})
}

// AdjustLessonHours 手工调整课时（原因必填，写 adjust 流水）。
func (s *Server) AdjustLessonHours(ctx khttp.Context) error {
	staff, err := s.requireStaff(ctx, true)
	if err != nil {
		return err
	}
	uc, err := s.requireLessons(ctx)
	if err != nil {
		return err
	}
	var req lessonHoursRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "INVALID_BODY", "请求体格式错误")
	}
	account, err := uc.Adjust(ctx, ctx.Vars().Get("studentID"), staff.ID, req.DeltaHours, req.Reason)
	if err != nil {
		return mapLessonError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"account": toLessonAccountDTO(*account)})
}

// ListLessonLedger 课时流水（可按学员与时间过滤）。
func (s *Server) ListLessonLedger(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	uc, err := s.requireLessons(ctx)
	if err != nil {
		return err
	}
	from, err := parseOptionalTime(ctx.Query().Get("from"))
	if err != nil {
		return fail(ctx, http.StatusBadRequest, "VALIDATION_FAILED", "from 必须是 RFC3339 时间")
	}
	to, err := parseOptionalTime(ctx.Query().Get("to"))
	if err != nil {
		return fail(ctx, http.StatusBadRequest, "VALIDATION_FAILED", "to 必须是 RFC3339 时间")
	}
	entries, err := uc.Ledger(ctx, strings.TrimSpace(ctx.Query().Get("student_id")), from, to)
	if err != nil {
		return mapLessonError(ctx, err)
	}
	out := make([]ledgerDTO, 0, len(entries))
	for _, entry := range entries {
		out = append(out, toLedgerDTO(entry))
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": out})
}

// PortalLessonAccount 家长端课时与近期课表。
func (s *Server) PortalLessonAccount(ctx khttp.Context) error {
	guardianID, err := s.requireGuardian(ctx)
	if err != nil {
		return err
	}
	uc, err := s.requireLessons(ctx)
	if err != nil {
		return err
	}
	if s.crm == nil {
		return fail(ctx, http.StatusServiceUnavailable, "CRM_DISABLED", "数据库未配置")
	}
	home, err := s.crm.PortalHome(ctx, guardianID)
	if err != nil {
		return mapBookingError(ctx, err)
	}
	accounts := make([]lessonAccountDTO, 0, len(home.Students))
	for _, student := range home.Students {
		account, _, err := uc.Detail(ctx, student.ID)
		if err != nil {
			if errors.Is(err, biz.ErrLessonAccountNotFound) {
				continue
			}
			return mapLessonError(ctx, err)
		}
		accounts = append(accounts, toLessonAccountDTO(*account))
	}
	var bookings []bookingDTO
	if s.bookings != nil {
		items, err := s.bookings.PortalBookings(ctx, guardianID)
		if err != nil {
			return mapBookingError(ctx, err)
		}
		for _, item := range items {
			bookings = append(bookings, toBookingDTO(item))
		}
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"students":  accounts,
		"bookings":  bookings,
		"threshold": biz.LowBalanceThreshold,
	})
}

func mapLessonError(ctx khttp.Context, err error) error {
	switch {
	case errors.Is(err, biz.ErrLessonAccountNotFound):
		return fail(ctx, http.StatusNotFound, "LESSON_ACCOUNT_NOT_FOUND", "课时账户不存在")
	case errors.Is(err, biz.ErrInsufficientHours):
		return fail(ctx, http.StatusUnprocessableEntity, "LESSON_HOURS_EXHAUSTED", "课时余额不足")
	case errors.Is(err, biz.ErrLedgerReasonRequired):
		return fail(ctx, http.StatusBadRequest, "REASON_REQUIRED", "请填写调整原因")
	case errors.Is(err, biz.ErrInvalidLedger):
		return fail(ctx, http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
	default:
		return fail(ctx, http.StatusInternalServerError, "INTERNAL", "服务器内部错误")
	}
}
