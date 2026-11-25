package biz

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// 时段与预约状态（需求 §3.3，ADR-011：提交即锁时段）。
const (
	SlotOpen      = "open"
	SlotLocked    = "locked"
	SlotCompleted = "completed"
	SlotCancelled = "cancelled"

	BookingPending   = "pending"
	BookingApproved  = "approved"
	BookingRejected  = "rejected"
	BookingCompleted = "completed"
	BookingCancelled = "cancelled"
	BookingNoShow    = "no_show"

	AttendanceAttended = "attended"
	AttendanceNoShow   = "no_show"

	minSlotMinutes = 30
	maxSlotMinutes = 120
	maxSlotWeeks   = 8
)

var (
	ErrSlotNotFound     = errors.New("slot not found")
	ErrSlotState        = errors.New("slot state conflict")
	ErrSlotTaken        = errors.New("slot already taken")
	ErrSlotOverlap      = errors.New("slot overlaps existing slot")
	ErrBookingNotFound  = errors.New("booking not found")
	ErrBookingState     = errors.New("booking state conflict")
	ErrInvalidBooking   = errors.New("invalid booking")
	ErrBookingForbidden = errors.New("booking forbidden")
)

// Slot 是可预约时段；列表里会带上占用它的预约信息。
type Slot struct {
	ID            string
	StaffID       string
	StaffName     string
	StartAt       time.Time
	EndAt         time.Time
	Status        string
	Note          string
	BookingID     string
	StudentName   string
	BookingStatus string
}

// Booking 是一条预约记录。
type Booking struct {
	ID             string
	SlotID         string
	StudentID      string
	StudentName    string
	Grade          string
	GuardianID     string
	GuardianName   string
	Status         string
	RequestNote    string
	ReviewNote     string
	CancelReason   string
	Attendance     string
	LessonLedgerID string
	ReviewedBy     string
	ReviewerName   string
	StartAt        time.Time
	EndAt          time.Time
	SubmittedAt    time.Time
	ReviewedAt     *time.Time
	CompletedAt    *time.Time
}

// BookingRepo 是时段与预约的仓储；所有跨表校验都在事务内完成。
type BookingRepo interface {
	ListSlots(ctx context.Context, from, to *time.Time) ([]Slot, error)
	CreateSlot(ctx context.Context, slot *Slot) error
	// BatchCreateSlots 按“时间点 + 星期几 + 周数”批量开放时段。
	BatchCreateSlots(ctx context.Context, staffID string, startAt, endAt time.Time, weekdays []int, weeks int) (int, error)
	UpdateSlot(ctx context.Context, slotID string, startAt, endAt time.Time, note string) (*Slot, error)
	CancelSlot(ctx context.Context, slotID string) error
	ListBookings(ctx context.Context, status string, from, to *time.Time) ([]Booking, error)
	PortalSlots(ctx context.Context, guardianID string, now time.Time) ([]Slot, error)
	PortalBookings(ctx context.Context, guardianID string, now time.Time) ([]Booking, error)
	// CreateBooking 事务内校验归属/余额/时段/重叠，并把时段置 locked；唯一约束冲突返回 ErrSlotTaken。
	CreateBooking(ctx context.Context, booking *Booking, minBalance float64) error
	ApproveBooking(ctx context.Context, bookingID, staffID, note string) error
	RejectBooking(ctx context.Context, bookingID, staffID, reason string) error
	// CompleteBooking 事务内写 consume 流水、回写 lesson_ledger_id、预约与时段置 completed；幂等。
	CompleteBooking(ctx context.Context, bookingID, staffID string, hours float64) (string, error)
	NoShowBooking(ctx context.Context, bookingID, staffID, note string) error
	CancelBooking(ctx context.Context, guardianID, bookingID, reason string) error
}

// BookingUsecase 编排时段开放、预约提交与审核。
type BookingUsecase struct {
	repo            BookingRepo
	now             func() time.Time
	hoursPerSession float64
}

func NewBookingUsecase(repo BookingRepo, hoursPerSession float64) *BookingUsecase {
	if hoursPerSession <= 0 {
		hoursPerSession = 1
	}
	return &BookingUsecase{repo: repo, now: time.Now, hoursPerSession: hoursPerSession}
}

// HoursPerSession 返回“一次课默认扣多少课时”。
func (uc *BookingUsecase) HoursPerSession() float64 { return uc.hoursPerSession }

// Slots 返回时段列表；from/to 均可空。
func (uc *BookingUsecase) Slots(ctx context.Context, from, to *time.Time) ([]Slot, error) {
	return uc.repo.ListSlots(ctx, from, to)
}

// CreateSlot 由老师开放单个时段；时长限制 30~120 分钟且不能早于当前时间。
func (uc *BookingUsecase) CreateSlot(ctx context.Context, staffID string, startAt, endAt time.Time, note string) (*Slot, error) {
	if err := validateSlotRange(uc.now(), startAt, endAt); err != nil {
		return nil, err
	}
	note = trimText(note, 500)
	slot := &Slot{StaffID: staffID, StartAt: startAt, EndAt: endAt, Status: SlotOpen, Note: note}
	if err := uc.repo.CreateSlot(ctx, slot); err != nil {
		return nil, err
	}
	return slot, nil
}

// BatchSlots 按周模板批量开放时段；weekdays 使用 1=周一 … 7=周日。
func (uc *BookingUsecase) BatchSlots(ctx context.Context, staffID string, startAt, endAt time.Time, weekdays []int, weeks int) (int, error) {
	if err := validateSlotRange(uc.now(), startAt, endAt); err != nil {
		return 0, err
	}
	if weeks < 1 || weeks > maxSlotWeeks {
		return 0, fmt.Errorf("%w: 周数必须在 1~%d 之间", ErrInvalidBooking, maxSlotWeeks)
	}
	seen := map[int]bool{}
	var cleaned []int
	for _, d := range weekdays {
		if d < 1 || d > 7 || seen[d] {
			return 0, fmt.Errorf("%w: 星期取值必须为 1~7 且不重复", ErrInvalidBooking)
		}
		seen[d] = true
		cleaned = append(cleaned, d)
	}
	if len(cleaned) == 0 {
		return 0, fmt.Errorf("%w: 至少选择一个星期", ErrInvalidBooking)
	}
	// 起点时间早于今天的部分跳过，避免生成过去的时段。
	return uc.repo.BatchCreateSlots(ctx, staffID, startAt, endAt, cleaned, weeks)
}

// UpdateSlot 修改未被占用的时段。
func (uc *BookingUsecase) UpdateSlot(ctx context.Context, slotID string, startAt, endAt time.Time, note string) (*Slot, error) {
	if strings.TrimSpace(slotID) == "" {
		return nil, ErrSlotNotFound
	}
	if err := validateSlotRange(uc.now(), startAt, endAt); err != nil {
		return nil, err
	}
	return uc.repo.UpdateSlot(ctx, slotID, startAt, endAt, trimText(note, 500))
}

// CancelSlot 取消未被占用的时段。
func (uc *BookingUsecase) CancelSlot(ctx context.Context, slotID string) error {
	if strings.TrimSpace(slotID) == "" {
		return ErrSlotNotFound
	}
	return uc.repo.CancelSlot(ctx, slotID)
}

// Bookings 返回老师端预约列表；status 为空表示全部。
func (uc *BookingUsecase) Bookings(ctx context.Context, status string, from, to *time.Time) ([]Booking, error) {
	switch status {
	case "", BookingPending, BookingApproved, BookingRejected, BookingCompleted, BookingCancelled, BookingNoShow:
	default:
		return nil, fmt.Errorf("%w: 未知的预约状态 %q", ErrInvalidBooking, status)
	}
	return uc.repo.ListBookings(ctx, status, from, to)
}

// PortalSlots 返回家长可预约的时段（open 且未开始，含自己已占用的）。
func (uc *BookingUsecase) PortalSlots(ctx context.Context, guardianID string) ([]Slot, error) {
	return uc.repo.PortalSlots(ctx, guardianID, uc.now())
}

// PortalBookings 返回家长的预约记录。
func (uc *BookingUsecase) PortalBookings(ctx context.Context, guardianID string) ([]Booking, error) {
	return uc.repo.PortalBookings(ctx, guardianID, uc.now())
}

// SubmitBooking 家长提交预约：提交即锁时段，余额 ≤0 禁止提交（需求 §3.3.2）。
func (uc *BookingUsecase) SubmitBooking(ctx context.Context, guardianID, studentID, slotID, note string) (*Booking, error) {
	studentID = strings.TrimSpace(studentID)
	slotID = strings.TrimSpace(slotID)
	if studentID == "" || slotID == "" {
		return nil, fmt.Errorf("%w: 学员与时段不能为空", ErrInvalidBooking)
	}
	booking := &Booking{
		SlotID:      slotID,
		StudentID:   studentID,
		GuardianID:  guardianID,
		Status:      BookingPending,
		RequestNote: trimText(note, 1000),
	}
	// 余额必须严格大于 0（阈值只用于预警，不阻止提交）。
	if err := uc.repo.CreateBooking(ctx, booking, 0.01); err != nil {
		return nil, err
	}
	return booking, nil
}

// Approve 审核通过。
func (uc *BookingUsecase) Approve(ctx context.Context, bookingID, staffID, note string) error {
	if strings.TrimSpace(bookingID) == "" {
		return ErrBookingNotFound
	}
	return uc.repo.ApproveBooking(ctx, bookingID, staffID, trimText(note, 1000))
}

// Reject 驳回，理由必填。
func (uc *BookingUsecase) Reject(ctx context.Context, bookingID, staffID, reason string) error {
	reason = trimText(reason, 1000)
	if reason == "" {
		return fmt.Errorf("%w: 驳回必须填写理由", ErrInvalidBooking)
	}
	if strings.TrimSpace(bookingID) == "" {
		return ErrBookingNotFound
	}
	return uc.repo.RejectBooking(ctx, bookingID, staffID, reason)
}

// Complete 标记出勤并消课；默认扣 hoursPerSession 课时，重复调用幂等（返回已有流水 ID）。
func (uc *BookingUsecase) Complete(ctx context.Context, bookingID, staffID string) (string, error) {
	if strings.TrimSpace(bookingID) == "" {
		return "", ErrBookingNotFound
	}
	return uc.repo.CompleteBooking(ctx, bookingID, staffID, uc.hoursPerSession)
}

// MarkNoShow 标记未到课：不自动扣课时（Q8 已确认）。
func (uc *BookingUsecase) MarkNoShow(ctx context.Context, bookingID, staffID, note string) error {
	if strings.TrimSpace(bookingID) == "" {
		return ErrBookingNotFound
	}
	return uc.repo.NoShowBooking(ctx, bookingID, staffID, trimText(note, 500))
}

// Cancel 家长撤回自己的预约（仅 pending；已通过的由老师处理）。
func (uc *BookingUsecase) Cancel(ctx context.Context, guardianID, bookingID, reason string) error {
	if strings.TrimSpace(bookingID) == "" {
		return ErrBookingNotFound
	}
	return uc.repo.CancelBooking(ctx, guardianID, bookingID, trimText(reason, 500))
}

func validateSlotRange(now, startAt, endAt time.Time) error {
	if startAt.IsZero() || endAt.IsZero() {
		return fmt.Errorf("%w: 开始与结束时间不能为空", ErrInvalidBooking)
	}
	if !endAt.After(startAt) {
		return fmt.Errorf("%w: 结束时间必须晚于开始时间", ErrInvalidBooking)
	}
	minutes := endAt.Sub(startAt).Minutes()
	if math.Abs(minutes-math.Round(minutes)) > 0.001 {
		return fmt.Errorf("%w: 时段必须按分钟对齐", ErrInvalidBooking)
	}
	if minutes < minSlotMinutes || minutes > maxSlotMinutes {
		return fmt.Errorf("%w: 时段时长必须在 %d~%d 分钟之间", ErrInvalidBooking, minSlotMinutes, maxSlotMinutes)
	}
	if !startAt.After(now.Add(-time.Minute)) {
		return fmt.Errorf("%w: 不能开放已经开始的时间", ErrInvalidBooking)
	}
	return nil
}

func trimText(s string, max int) string {
	s = strings.TrimSpace(s)
	if runes := []rune(s); len(runes) > max {
		s = string(runes[:max])
	}
	return s
}
