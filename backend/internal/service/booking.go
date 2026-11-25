package service

import (
	"errors"
	"net/http"
	"strings"
	"time"

	khttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/bishop9910/hm-lesson/backend/internal/biz"
)

// 时段与预约接口（M3）：教师端 /api/v1/slots、/api/v1/bookings，家长端 /api/v1/portal/*。
type slotDTO struct {
	ID            string `json:"id"`
	StaffID       string `json:"staff_id"`
	StaffName     string `json:"staff_name"`
	StartAt       string `json:"start_at"`
	EndAt         string `json:"end_at"`
	Status        string `json:"status"`
	Note          string `json:"note"`
	BookingID     string `json:"booking_id"`
	StudentName   string `json:"student_name"`
	BookingStatus string `json:"booking_status"`
}

type bookingDTO struct {
	ID             string  `json:"id"`
	SlotID         string  `json:"slot_id"`
	StudentID      string  `json:"student_id"`
	StudentName    string  `json:"student_name"`
	Grade          string  `json:"grade"`
	GuardianID     string  `json:"guardian_id"`
	GuardianName   string  `json:"guardian_name"`
	Status         string  `json:"status"`
	RequestNote    string  `json:"request_note"`
	ReviewNote     string  `json:"review_note"`
	CancelReason   string  `json:"cancel_reason"`
	Attendance     string  `json:"attendance"`
	LessonLedgerID string  `json:"lesson_ledger_id"`
	ReviewerName   string  `json:"reviewer_name"`
	StartAt        string  `json:"start_at"`
	EndAt          string  `json:"end_at"`
	SubmittedAt    string  `json:"submitted_at"`
	ReviewedAt     *string `json:"reviewed_at"`
	CompletedAt    *string `json:"completed_at"`
}

func toSlotDTO(s biz.Slot) slotDTO {
	return slotDTO{
		ID:            s.ID,
		StaffID:       s.StaffID,
		StaffName:     s.StaffName,
		StartAt:       s.StartAt.UTC().Format("2006-01-02T15:04:05Z"),
		EndAt:         s.EndAt.UTC().Format("2006-01-02T15:04:05Z"),
		Status:        s.Status,
		Note:          s.Note,
		BookingID:     s.BookingID,
		StudentName:   s.StudentName,
		BookingStatus: s.BookingStatus,
	}
}

func toBookingDTO(b biz.Booking) bookingDTO {
	return bookingDTO{
		ID:             b.ID,
		SlotID:         b.SlotID,
		StudentID:      b.StudentID,
		StudentName:    b.StudentName,
		Grade:          b.Grade,
		GuardianID:     b.GuardianID,
		GuardianName:   b.GuardianName,
		Status:         b.Status,
		RequestNote:    b.RequestNote,
		ReviewNote:     b.ReviewNote,
		CancelReason:   b.CancelReason,
		Attendance:     b.Attendance,
		LessonLedgerID: b.LessonLedgerID,
		ReviewerName:   b.ReviewerName,
		StartAt:        b.StartAt.UTC().Format("2006-01-02T15:04:05Z"),
		EndAt:          b.EndAt.UTC().Format("2006-01-02T15:04:05Z"),
		SubmittedAt:    b.SubmittedAt.UTC().Format("2006-01-02T15:04:05Z"),
		ReviewedAt:     fmtTime(b.ReviewedAt),
		CompletedAt:    fmtTime(b.CompletedAt),
	}
}

func (s *Server) requireBookings(ctx khttp.Context) (*biz.BookingUsecase, error) {
	if s.bookings == nil {
		return nil, fail(ctx, http.StatusServiceUnavailable, "BOOKING_DISABLED", "数据库未配置")
	}
	return s.bookings, nil
}

// ListSlots 教师端时段列表，支持 from/to 过滤。
func (s *Server) ListSlots(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	uc, err := s.requireBookings(ctx)
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
	items, err := uc.Slots(ctx, from, to)
	if err != nil {
		return mapBookingError(ctx, err)
	}
	out := make([]slotDTO, 0, len(items))
	for _, item := range items {
		out = append(out, toSlotDTO(item))
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": out})
}

type createSlotRequest struct {
	StartAt string `json:"start_at"`
	EndAt   string `json:"end_at"`
	Note    string `json:"note"`
}

// CreateSlot 开放单个时段。
func (s *Server) CreateSlot(ctx khttp.Context) error {
	staff, err := s.requireStaff(ctx, false)
	if err != nil {
		return err
	}
	uc, err := s.requireBookings(ctx)
	if err != nil {
		return err
	}
	var req createSlotRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "INVALID_BODY", "请求体格式错误")
	}
	startAt, endAt, err := parseSlotRange(req.StartAt, req.EndAt)
	if err != nil {
		return fail(ctx, http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
	}
	slot, err := uc.CreateSlot(ctx, staff.ID, startAt, endAt, req.Note)
	if err != nil {
		return mapBookingError(ctx, err)
	}
	return ctx.JSON(http.StatusCreated, map[string]any{"slot": toSlotDTO(*slot)})
}

type batchSlotsRequest struct {
	StartAt  string `json:"start_at"`
	EndAt    string `json:"end_at"`
	Weekdays []int  `json:"weekdays"`
	Weeks    int    `json:"weeks"`
}

// BatchSlots 按周模板批量开放时段（weekdays 1=周一 … 7=周日）。
func (s *Server) BatchSlots(ctx khttp.Context) error {
	staff, err := s.requireStaff(ctx, false)
	if err != nil {
		return err
	}
	uc, err := s.requireBookings(ctx)
	if err != nil {
		return err
	}
	var req batchSlotsRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "INVALID_BODY", "请求体格式错误")
	}
	startAt, endAt, err := parseSlotRange(req.StartAt, req.EndAt)
	if err != nil {
		return fail(ctx, http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
	}
	created, err := uc.BatchSlots(ctx, staff.ID, startAt, endAt, req.Weekdays, req.Weeks)
	if err != nil {
		return mapBookingError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"created": created})
}

// UpdateSlot 修改未被占用的时段。
func (s *Server) UpdateSlot(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	uc, err := s.requireBookings(ctx)
	if err != nil {
		return err
	}
	var req createSlotRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "INVALID_BODY", "请求体格式错误")
	}
	startAt, endAt, err := parseSlotRange(req.StartAt, req.EndAt)
	if err != nil {
		return fail(ctx, http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
	}
	slot, err := uc.UpdateSlot(ctx, ctx.Vars().Get("id"), startAt, endAt, req.Note)
	if err != nil {
		return mapBookingError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"slot": toSlotDTO(*slot)})
}

// CancelSlot 取消未被占用的时段。
func (s *Server) CancelSlot(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	uc, err := s.requireBookings(ctx)
	if err != nil {
		return err
	}
	if err := uc.CancelSlot(ctx, ctx.Vars().Get("id")); err != nil {
		return mapBookingError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"ok": true})
}

// ListBookings 教师端预约列表；status 为空表示全部。
func (s *Server) ListBookings(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	return s.listBookings(ctx)
}

// CalendarBookings 日历视图（按 from/to 过滤时段）。
func (s *Server) CalendarBookings(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	return s.listBookings(ctx)
}

func (s *Server) listBookings(ctx khttp.Context) error {
	uc, err := s.requireBookings(ctx)
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
	items, err := uc.Bookings(ctx, strings.TrimSpace(ctx.Query().Get("status")), from, to)
	if err != nil {
		return mapBookingError(ctx, err)
	}
	out := make([]bookingDTO, 0, len(items))
	for _, item := range items {
		out = append(out, toBookingDTO(item))
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": out})
}

type reviewBookingRequest struct {
	Note   string `json:"note"`
	Reason string `json:"reason"`
}

// ApproveBooking 审核通过。
func (s *Server) ApproveBooking(ctx khttp.Context) error {
	staff, err := s.requireStaff(ctx, false)
	if err != nil {
		return err
	}
	uc, err := s.requireBookings(ctx)
	if err != nil {
		return err
	}
	var req reviewBookingRequest
	_ = ctx.Bind(&req)
	if err := uc.Approve(ctx, ctx.Vars().Get("id"), staff.ID, req.Note); err != nil {
		return mapBookingError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"ok": true})
}

// RejectBooking 驳回（必填理由）。
func (s *Server) RejectBooking(ctx khttp.Context) error {
	staff, err := s.requireStaff(ctx, false)
	if err != nil {
		return err
	}
	uc, err := s.requireBookings(ctx)
	if err != nil {
		return err
	}
	var req reviewBookingRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "INVALID_BODY", "请求体格式错误")
	}
	if err := uc.Reject(ctx, ctx.Vars().Get("id"), staff.ID, req.Reason); err != nil {
		return mapBookingError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"ok": true})
}

// CompleteBooking 出勤并消课（幂等，返回消课流水 ID）。
func (s *Server) CompleteBooking(ctx khttp.Context) error {
	staff, err := s.requireStaff(ctx, false)
	if err != nil {
		return err
	}
	uc, err := s.requireBookings(ctx)
	if err != nil {
		return err
	}
	ledgerID, err := uc.Complete(ctx, ctx.Vars().Get("id"), staff.ID)
	if err != nil {
		return mapBookingError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"ok": true, "lesson_ledger_id": ledgerID, "hours": uc.HoursPerSession()})
}

// NoShowBooking 未到课（不扣课时）。
func (s *Server) NoShowBooking(ctx khttp.Context) error {
	staff, err := s.requireStaff(ctx, false)
	if err != nil {
		return err
	}
	uc, err := s.requireBookings(ctx)
	if err != nil {
		return err
	}
	var req reviewBookingRequest
	_ = ctx.Bind(&req)
	if err := uc.MarkNoShow(ctx, ctx.Vars().Get("id"), staff.ID, req.Note); err != nil {
		return mapBookingError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"ok": true})
}

// PortalSlots 家长端可预约时段。
func (s *Server) PortalSlots(ctx khttp.Context) error {
	guardianID, err := s.requireGuardian(ctx)
	if err != nil {
		return err
	}
	uc, err := s.requireBookings(ctx)
	if err != nil {
		return err
	}
	items, err := uc.PortalSlots(ctx, guardianID)
	if err != nil {
		return mapBookingError(ctx, err)
	}
	out := make([]slotDTO, 0, len(items))
	for _, item := range items {
		out = append(out, toSlotDTO(item))
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": out})
}

type portalBookingRequest struct {
	SlotID    string `json:"slot_id"`
	StudentID string `json:"student_id"`
	Note      string `json:"note"`
	Reason    string `json:"reason"`
}

// PortalCreateBooking 提交预约（提交即锁时段）。
func (s *Server) PortalCreateBooking(ctx khttp.Context) error {
	guardianID, err := s.requireGuardian(ctx)
	if err != nil {
		return err
	}
	uc, err := s.requireBookings(ctx)
	if err != nil {
		return err
	}
	var req portalBookingRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "INVALID_BODY", "请求体格式错误")
	}
	booking, err := uc.SubmitBooking(ctx, guardianID, req.StudentID, req.SlotID, req.Note)
	if err != nil {
		return mapBookingError(ctx, err)
	}
	return ctx.JSON(http.StatusCreated, map[string]any{"booking": toBookingDTO(*booking)})
}

// PortalBookings 家长端我的预约。
func (s *Server) PortalBookings(ctx khttp.Context) error {
	guardianID, err := s.requireGuardian(ctx)
	if err != nil {
		return err
	}
	uc, err := s.requireBookings(ctx)
	if err != nil {
		return err
	}
	items, err := uc.PortalBookings(ctx, guardianID)
	if err != nil {
		return mapBookingError(ctx, err)
	}
	out := make([]bookingDTO, 0, len(items))
	for _, item := range items {
		out = append(out, toBookingDTO(item))
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": out})
}

// PortalCancelBooking 家长撤回预约（仅 pending）。
func (s *Server) PortalCancelBooking(ctx khttp.Context) error {
	guardianID, err := s.requireGuardian(ctx)
	if err != nil {
		return err
	}
	uc, err := s.requireBookings(ctx)
	if err != nil {
		return err
	}
	var req portalBookingRequest
	_ = ctx.Bind(&req)
	if err := uc.Cancel(ctx, guardianID, ctx.Vars().Get("id"), req.Reason); err != nil {
		return mapBookingError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"ok": true})
}

// parseSlotRange 解析时段起止时间。
func parseSlotRange(start, end string) (time.Time, time.Time, error) {
	startAt, err := time.Parse(time.RFC3339, strings.TrimSpace(start))
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("start_at 必须是 RFC3339 时间")
	}
	endAt, err := time.Parse(time.RFC3339, strings.TrimSpace(end))
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("end_at 必须是 RFC3339 时间")
	}
	return startAt, endAt, nil
}

func parseOptionalTime(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func mapBookingError(ctx khttp.Context, err error) error {
	switch {
	case errors.Is(err, biz.ErrSlotNotFound):
		return fail(ctx, http.StatusNotFound, "SLOT_NOT_FOUND", "时段不存在")
	case errors.Is(err, biz.ErrBookingNotFound):
		return fail(ctx, http.StatusNotFound, "BOOKING_NOT_FOUND", "预约不存在")
	case errors.Is(err, biz.ErrSlotTaken):
		return fail(ctx, http.StatusConflict, "BOOKING_SLOT_TAKEN", "该时段已被预约")
	case errors.Is(err, biz.ErrSlotOverlap):
		return fail(ctx, http.StatusConflict, "SLOT_OVERLAP", "该时间段与已有课时段重叠")
	case errors.Is(err, biz.ErrSlotState), errors.Is(err, biz.ErrBookingState):
		return fail(ctx, http.StatusConflict, "STATE_CONFLICT", "当前状态不允许该操作")
	case errors.Is(err, biz.ErrInsufficientHours):
		return fail(ctx, http.StatusUnprocessableEntity, "LESSON_HOURS_EXHAUSTED", "课时余额不足，请先充值课时")
	case errors.Is(err, biz.ErrInvalidBooking):
		return fail(ctx, http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
	case errors.Is(err, biz.ErrForbidden):
		return fail(ctx, http.StatusNotFound, "NOT_FOUND", "资源不存在")
	default:
		return fail(ctx, http.StatusInternalServerError, "INTERNAL", "服务器内部错误")
	}
}
