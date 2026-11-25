package biz

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeBookingRepo struct {
	createdSlot    *Slot
	createSlotErr  error
	batchCalls     int
	batchWeekdays  []int
	batchWeeks     int
	booking        *Booking
	minBalance     float64
	submitErr      error
	approveCalls   int
	rejectReason   string
	completeLedger string
	completeErr    error
	noShowCalls    int
	cancelCalls    int
}

func (f *fakeBookingRepo) ListSlots(context.Context, *time.Time, *time.Time) ([]Slot, error) {
	return nil, nil
}

func (f *fakeBookingRepo) CreateSlot(_ context.Context, slot *Slot) error {
	f.createdSlot = slot
	return f.createSlotErr
}

func (f *fakeBookingRepo) BatchCreateSlots(_ context.Context, _ string, _, _ time.Time, weekdays []int, weeks int) (int, error) {
	f.batchCalls++
	f.batchWeekdays = weekdays
	f.batchWeeks = weeks
	return len(weekdays) * weeks, nil
}

func (f *fakeBookingRepo) UpdateSlot(context.Context, string, time.Time, time.Time, string) (*Slot, error) {
	return nil, nil
}

func (f *fakeBookingRepo) CancelSlot(context.Context, string) error { return nil }

func (f *fakeBookingRepo) ListBookings(context.Context, string, *time.Time, *time.Time) ([]Booking, error) {
	return nil, nil
}

func (f *fakeBookingRepo) PortalSlots(context.Context, string, time.Time) ([]Slot, error) {
	return nil, nil
}

func (f *fakeBookingRepo) PortalBookings(context.Context, string, time.Time) ([]Booking, error) {
	return nil, nil
}

func (f *fakeBookingRepo) CreateBooking(_ context.Context, booking *Booking, minBalance float64) error {
	f.booking = booking
	f.minBalance = minBalance
	if f.submitErr != nil {
		return f.submitErr
	}
	booking.ID = "booking-1"
	return nil
}

func (f *fakeBookingRepo) ApproveBooking(context.Context, string, string, string) error {
	f.approveCalls++
	return nil
}

func (f *fakeBookingRepo) RejectBooking(_ context.Context, _, _ string, reason string) error {
	f.rejectReason = reason
	return nil
}

func (f *fakeBookingRepo) CompleteBooking(context.Context, string, string, float64) (string, error) {
	return f.completeLedger, f.completeErr
}

func (f *fakeBookingRepo) NoShowBooking(context.Context, string, string, string) error {
	f.noShowCalls++
	return nil
}

func (f *fakeBookingRepo) CancelBooking(context.Context, string, string, string) error {
	f.cancelCalls++
	return nil
}

func TestValidateSlotRange(t *testing.T) {
	now := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	cases := []struct {
		name  string
		start time.Time
		end   time.Time
		ok    bool
	}{
		{"60 分钟", now.Add(time.Hour), now.Add(2 * time.Hour), true},
		{"30 分钟下限", now.Add(time.Hour), now.Add(90 * time.Minute), true},
		{"120 分钟上限", now.Add(time.Hour), now.Add(3 * time.Hour), true},
		{"过短", now.Add(time.Hour), now.Add(time.Hour + 10*time.Minute), false},
		{"过长", now.Add(time.Hour), now.Add(4 * time.Hour), false},
		{"结束早于开始", now.Add(2 * time.Hour), now.Add(time.Hour), false},
		{"已过去", now.Add(-2 * time.Hour), now.Add(-time.Hour), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSlotRange(now, tc.start, tc.end)
			if tc.ok && err != nil {
				t.Fatalf("want ok, got %v", err)
			}
			if !tc.ok && !errors.Is(err, ErrInvalidBooking) {
				t.Fatalf("want ErrInvalidBooking, got %v", err)
			}
		})
	}
}

func TestSubmitBookingLocksSlotAndRequiresBalance(t *testing.T) {
	repo := &fakeBookingRepo{}
	uc := NewBookingUsecase(repo, 1)
	uc.now = func() time.Time { return time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC) }

	booking, err := uc.SubmitBooking(context.Background(), "g1", " s1 ", " slot1 ", "  想补物理  ")
	if err != nil {
		t.Fatalf("SubmitBooking: %v", err)
	}
	if booking.Status != BookingPending {
		t.Fatalf("status = %q, want pending", booking.Status)
	}
	if repo.booking.StudentID != "s1" || repo.booking.SlotID != "slot1" {
		t.Fatalf("unexpected trimmed ids: %+v", repo.booking)
	}
	if repo.booking.RequestNote != "想补物理" {
		t.Fatalf("note not trimmed: %q", repo.booking.RequestNote)
	}
	if repo.minBalance != 0.01 {
		t.Fatalf("minBalance = %v, want 0.01", repo.minBalance)
	}

	if _, err := uc.SubmitBooking(context.Background(), "g1", "", "slot1", ""); !errors.Is(err, ErrInvalidBooking) {
		t.Fatalf("empty student error = %v, want ErrInvalidBooking", err)
	}

	repo.submitErr = ErrInsufficientHours
	if _, err := uc.SubmitBooking(context.Background(), "g1", "s1", "slot2", ""); !errors.Is(err, ErrInsufficientHours) {
		t.Fatalf("error = %v, want ErrInsufficientHours", err)
	}
}

func TestRejectRequiresReason(t *testing.T) {
	repo := &fakeBookingRepo{}
	uc := NewBookingUsecase(repo, 1)

	if err := uc.Reject(context.Background(), "b1", "staff1", "   "); !errors.Is(err, ErrInvalidBooking) {
		t.Fatalf("empty reason error = %v, want ErrInvalidBooking", err)
	}
	if err := uc.Reject(context.Background(), "b1", "staff1", "时间冲突"); err != nil {
		t.Fatalf("Reject: %v", err)
	}
	if repo.rejectReason != "时间冲突" {
		t.Fatalf("reason = %q", repo.rejectReason)
	}
}

func TestBatchSlotsValidation(t *testing.T) {
	repo := &fakeBookingRepo{}
	uc := NewBookingUsecase(repo, 1)
	now := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)
	uc.now = func() time.Time { return now }
	start := now.Add(24 * time.Hour)
	end := start.Add(time.Hour)

	if _, err := uc.BatchSlots(context.Background(), "staff1", start, end, []int{8}, 2); !errors.Is(err, ErrInvalidBooking) {
		t.Fatalf("invalid weekday error = %v", err)
	}
	if _, err := uc.BatchSlots(context.Background(), "staff1", start, end, []int{1, 1}, 2); !errors.Is(err, ErrInvalidBooking) {
		t.Fatalf("duplicate weekday error = %v", err)
	}
	if _, err := uc.BatchSlots(context.Background(), "staff1", start, end, []int{1}, 0); !errors.Is(err, ErrInvalidBooking) {
		t.Fatalf("invalid weeks error = %v", err)
	}
	if _, err := uc.BatchSlots(context.Background(), "staff1", start, end, []int{1, 3}, 2); err != nil {
		t.Fatalf("BatchSlots: %v", err)
	}
	if repo.batchWeeks != 2 || len(repo.batchWeekdays) != 2 {
		t.Fatalf("unexpected batch args: weeks=%d weekdays=%v", repo.batchWeeks, repo.batchWeekdays)
	}
}

func TestCompleteUsesSessionHours(t *testing.T) {
	repo := &fakeBookingRepo{completeLedger: "ledger-1"}
	uc := NewBookingUsecase(repo, 1.5)
	if uc.HoursPerSession() != 1.5 {
		t.Fatalf("HoursPerSession = %v", uc.HoursPerSession())
	}
	ledger, err := uc.Complete(context.Background(), "b1", "staff1")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if ledger != "ledger-1" {
		t.Fatalf("ledger = %q", ledger)
	}
	if _, err := uc.Complete(context.Background(), "", "staff1"); !errors.Is(err, ErrBookingNotFound) {
		t.Fatalf("empty id error = %v", err)
	}
}

func TestNewBookingUsecaseDefaultHours(t *testing.T) {
	uc := NewBookingUsecase(&fakeBookingRepo{}, 0)
	if uc.HoursPerSession() != 1 {
		t.Fatalf("default hours = %v, want 1", uc.HoursPerSession())
	}
}
