package biz

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeDashboardRepo struct {
	summary DashboardSummary
	board   Board
	err     error
	calls   int
}

func (f *fakeDashboardRepo) Summary(context.Context) (DashboardSummary, error) {
	return f.summary, f.err
}

func (f *fakeDashboardRepo) Board(_ context.Context, from, to time.Time) (Board, error) {
	f.calls++
	board := f.board
	board.From = from
	board.To = to
	return board, f.err
}

func TestDashboardSummary(t *testing.T) {
	want := DashboardSummary{StudentsTotal: 3, SubmissionsPending: 2, BookingsPending: 1, LowBalanceStudents: 1, StaffTotal: 1}
	uc := NewDashboardUsecase(&fakeDashboardRepo{summary: want})
	got, err := uc.Summary(context.Background())
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestDashboardWithoutRepo(t *testing.T) {
	uc := NewDashboardUsecase(nil)
	if _, err := uc.Summary(context.Background()); !errors.Is(err, ErrDashboardUnavailable) {
		t.Fatalf("want ErrDashboardUnavailable, got %v", err)
	}
	if _, err := uc.Board(context.Background(), time.Now(), time.Now().Add(time.Hour)); !errors.Is(err, ErrDashboardUnavailable) {
		t.Fatalf("want ErrDashboardUnavailable, got %v", err)
	}
}

func TestDashboardBoardCache(t *testing.T) {
	now := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	repo := &fakeDashboardRepo{board: Board{NewStudents: 4, NewLeads: 5, Converted: 2}}
	uc := NewDashboardUsecase(repo)
	uc.now = func() time.Time { return now }
	from := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)

	first, err := uc.Board(context.Background(), from, to)
	if err != nil {
		t.Fatalf("Board: %v", err)
	}
	if first.GeneratedAt.IsZero() || !first.From.Equal(from) || !first.To.Equal(to) {
		t.Fatalf("board 区间未回填: %+v", first)
	}
	second, err := uc.Board(context.Background(), from, to)
	if err != nil {
		t.Fatalf("Board again: %v", err)
	}
	if repo.calls != 1 {
		t.Fatalf("60 秒内应命中缓存，repo 调用次数 = %d", repo.calls)
	}
	if second.NewStudents != first.NewStudents {
		t.Fatalf("缓存结果不一致: %+v vs %+v", second, first)
	}

	// 超过 TTL 后重新查询。
	uc.now = func() time.Time { return now.Add(BoardCacheTTL + time.Second) }
	if _, err := uc.Board(context.Background(), from, to); err != nil {
		t.Fatalf("Board after ttl: %v", err)
	}
	if repo.calls != 2 {
		t.Fatalf("超过 TTL 应重新查询，repo 调用次数 = %d", repo.calls)
	}

	// 换区间不走缓存。
	otherFrom := from.AddDate(0, -1, 0)
	if _, err := uc.Board(context.Background(), otherFrom, from); err != nil {
		t.Fatalf("Board other range: %v", err)
	}
	if repo.calls != 3 {
		t.Fatalf("换区间应重新查询，repo 调用次数 = %d", repo.calls)
	}
}

func TestBoardGroupName(t *testing.T) {
	cases := map[string]string{
		GroupClassActive: "班课在读",
		GroupOneOnOne:    "一对一在读",
		GroupIntent:      "待转化意向",
		GroupVisitor:     "咨询访客",
		"unknown":        "unknown",
	}
	for code, want := range cases {
		if got := BoardGroupName(code); got != want {
			t.Fatalf("BoardGroupName(%q) = %q, want %q", code, got, want)
		}
	}
}
