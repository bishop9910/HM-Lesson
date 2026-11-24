package biz

import (
	"context"
	"errors"
	"testing"
)

type fakeDashboardRepo struct {
	summary DashboardSummary
	err     error
}

func (f *fakeDashboardRepo) Summary(context.Context) (DashboardSummary, error) {
	return f.summary, f.err
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
}
