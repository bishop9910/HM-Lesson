package biz

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrDashboardUnavailable = errors.New("dashboard unavailable")

// DashboardSummary 是教师工作台/首页展示的真实汇总数据。
// 只做聚合读取，不包含任何演示数据。
type DashboardSummary struct {
	StudentsTotal      int64
	SubmissionsPending int64
	BookingsPending    int64
	LowBalanceStudents int64
	StaffTotal         int64
}

// BoardGroupCount 分组人数。
type BoardGroupCount struct {
	Code  string
	Name  string
	Count int64
}

// BoardDelivery 交付质量（区间内）。
type BoardDelivery struct {
	Tasks         int64
	Assignments   int64
	Submissions   int64
	Overdue       int64
	PendingReview int64
	SubmitRate    float64
}

// BoardOneOnOne 一对一运营。
type BoardOneOnOne struct {
	PendingBookings int64
	WeekSessions    int64
	MonthConsumed   float64
}

// BoardTodo 待办。
type BoardTodo struct {
	FollowUpsDue    int64
	OverdueApproved int64
	Unconfirmed     int64
	LowBalance      int64
}

// Board 是 M9 看板的四组指标（docs/backend-requirements.md §3.8）。
type Board struct {
	Groups         []BoardGroupCount
	NewStudents    int64
	NewLeads       int64
	Converted      int64
	ConversionRate float64
	Delivery       BoardDelivery
	OneOnOne       BoardOneOnOne
	Todo           BoardTodo
	From           time.Time
	To             time.Time
	GeneratedAt    time.Time
}

// DashboardRepo 汇总读取接口。
type DashboardRepo interface {
	Summary(ctx context.Context) (DashboardSummary, error)
	Board(ctx context.Context, from, to time.Time) (Board, error)
}

// DashboardUsecase 看板用例，带 60 秒进程内缓存（无 Redis 的降级实现）。
type DashboardUsecase struct {
	repo       DashboardRepo
	mu         sync.Mutex
	cached     *Board
	cachedAt   time.Time
	cachedFrom time.Time
	cachedTo   time.Time
	now        func() time.Time
}

// NewDashboardUsecase 构造看板用例。
func NewDashboardUsecase(repo DashboardRepo) *DashboardUsecase {
	return &DashboardUsecase{repo: repo, now: time.Now}
}

// Summary 首页汇总。
func (uc *DashboardUsecase) Summary(ctx context.Context) (DashboardSummary, error) {
	if uc == nil || uc.repo == nil {
		return DashboardSummary{}, ErrDashboardUnavailable
	}
	return uc.repo.Summary(ctx)
}

// BoardCacheTTL 是看板聚合结果的缓存时间。
const BoardCacheTTL = 60 * time.Second

// Board 返回四组指标；同一区间 60 秒内复用缓存。
func (uc *DashboardUsecase) Board(ctx context.Context, from, to time.Time) (Board, error) {
	if uc == nil || uc.repo == nil {
		return Board{}, ErrDashboardUnavailable
	}
	now := uc.now()
	uc.mu.Lock()
	if uc.cached != nil && now.Sub(uc.cachedAt) < BoardCacheTTL &&
		uc.cachedFrom.Equal(from) && uc.cachedTo.Equal(to) {
		cached := *uc.cached
		uc.mu.Unlock()
		return cached, nil
	}
	uc.mu.Unlock()

	board, err := uc.repo.Board(ctx, from, to)
	if err != nil {
		return Board{}, err
	}
	board.From = from
	board.To = to
	board.GeneratedAt = now.UTC()

	uc.mu.Lock()
	cached := board
	uc.cached = &cached
	uc.cachedAt = now
	uc.cachedFrom = from
	uc.cachedTo = to
	uc.mu.Unlock()
	return board, nil
}

// BoardGroupName 是四个客户分组的展示名。
func BoardGroupName(code string) string {
	switch code {
	case GroupClassActive:
		return "班课在读"
	case GroupOneOnOne:
		return "一对一在读"
	case GroupIntent:
		return "待转化意向"
	case GroupVisitor:
		return "咨询访客"
	default:
		return code
	}
}
