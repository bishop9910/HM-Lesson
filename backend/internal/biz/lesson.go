package biz

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// 课时台账（ADR-012）：balance_hours 只是流水汇总的缓存，任何变更必须同事务追加一条流水。
const (
	LedgerPurchase = "purchase"
	LedgerConsume  = "consume"
	LedgerAdjust   = "adjust"
	LedgerRefund   = "refund"

	LessonActive    = "active"
	LessonExhausted = "exhausted"
	LessonExpired   = "expired"

	// LowBalanceThreshold 余额低于该值时老师端显示预警（需求 §3.4 AF-12，阈值可配）。
	LowBalanceThreshold = 2.0

	maxLedgerHours = 200.0
)

var (
	ErrLessonAccountNotFound = errors.New("lesson account not found")
	ErrInvalidLedger         = errors.New("invalid lesson ledger")
	ErrInsufficientHours     = errors.New("insufficient lesson hours")
	ErrLedgerReasonRequired  = errors.New("ledger reason required")
)

// LessonAccount 是学员课时账户；余额字段含两位小数。
type LessonAccount struct {
	StudentID      string
	StudentName    string
	Grade          string
	PurchasedHours float64
	UsedHours      float64
	BalanceHours   float64
	Status         string
	ValidUntil     *time.Time
	UpdatedAt      time.Time
}

// LowBalance 标记是否低于预警阈值（前端展示用）。
func (a *LessonAccount) LowBalance() bool {
	return a.BalanceHours <= LowBalanceThreshold
}

// LedgerEntry 是只追加的课时流水，delta 为有符号小时数。
type LedgerEntry struct {
	ID           string
	StudentID    string
	Type         string
	DeltaHours   float64
	BalanceAfter float64
	RefType      string
	RefID        string
	Reason       string
	OperatorID   string
	OperatorName string
	OccurredAt   time.Time
}

// LessonRepo 是课时账户与流水的仓储。
type LessonRepo interface {
	ListAccounts(ctx context.Context) ([]LessonAccount, error)
	GetAccount(ctx context.Context, studentID string) (*LessonAccount, error)
	ListLedger(ctx context.Context, studentID string, from, to *time.Time) ([]LedgerEntry, error)
	// ApplyLedger 在事务内锁定账户行、追加流水并更新缓存余额；余额不足时返回 ErrInsufficientHours。
	ApplyLedger(ctx context.Context, entry *LedgerEntry) (*LessonAccount, error)
}

// LessonUsecase 编排课时充值、调整与查询。
type LessonUsecase struct {
	repo LessonRepo
}

func NewLessonUsecase(repo LessonRepo) *LessonUsecase {
	return &LessonUsecase{repo: repo}
}

// List 返回全部学员的课时账户（含从未充值的学员，余额为 0）。
func (uc *LessonUsecase) List(ctx context.Context) ([]LessonAccount, error) {
	return uc.repo.ListAccounts(ctx)
}

// Detail 返回账户与最近流水。
func (uc *LessonUsecase) Detail(ctx context.Context, studentID string) (*LessonAccount, []LedgerEntry, error) {
	studentID = strings.TrimSpace(studentID)
	if studentID == "" {
		return nil, nil, ErrLessonAccountNotFound
	}
	account, err := uc.repo.GetAccount(ctx, studentID)
	if err != nil {
		return nil, nil, err
	}
	entries, err := uc.repo.ListLedger(ctx, studentID, nil, nil)
	if err != nil {
		return nil, nil, err
	}
	return account, entries, nil
}

// Ledger 按学员与时间范围查询流水；studentID 为空表示全部学员。
func (uc *LessonUsecase) Ledger(ctx context.Context, studentID string, from, to *time.Time) ([]LedgerEntry, error) {
	return uc.repo.ListLedger(ctx, strings.TrimSpace(studentID), from, to)
}

// Purchase 充值课时：写 purchase 流水（delta 为正）。
func (uc *LessonUsecase) Purchase(ctx context.Context, studentID, operatorID string, hours float64, reason string) (*LessonAccount, error) {
	hours = roundHours(hours)
	if hours <= 0 || hours > maxLedgerHours {
		return nil, fmt.Errorf("%w: 充值课时必须在 0~%g 之间", ErrInvalidLedger, maxLedgerHours)
	}
	entry := &LedgerEntry{
		StudentID:  strings.TrimSpace(studentID),
		Type:       LedgerPurchase,
		DeltaHours: hours,
		RefType:    "manual",
		Reason:     trimReason(reason),
		OperatorID: operatorID,
	}
	return uc.repo.ApplyLedger(ctx, entry)
}

// Adjust 手工纠错：原因必填，delta 可正可负（需求 §3.4：不改流水，纠错走 adjust）。
func (uc *LessonUsecase) Adjust(ctx context.Context, studentID, operatorID string, delta float64, reason string) (*LessonAccount, error) {
	delta = roundHours(delta)
	if delta == 0 || math.Abs(delta) > maxLedgerHours {
		return nil, fmt.Errorf("%w: 调整小时数必须非零且不超过 %g", ErrInvalidLedger, maxLedgerHours)
	}
	reason = trimReason(reason)
	if reason == "" {
		return nil, ErrLedgerReasonRequired
	}
	entry := &LedgerEntry{
		StudentID:  strings.TrimSpace(studentID),
		Type:       LedgerAdjust,
		DeltaHours: delta,
		RefType:    "manual",
		Reason:     reason,
		OperatorID: operatorID,
	}
	return uc.repo.ApplyLedger(ctx, entry)
}

func trimReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if runes := []rune(reason); len(runes) > 500 {
		reason = string(runes[:500])
	}
	return reason
}

func roundHours(v float64) float64 {
	return math.Round(v*100) / 100
}
