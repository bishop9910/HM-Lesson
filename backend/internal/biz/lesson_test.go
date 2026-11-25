package biz

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeLessonRepo struct {
	account    *LessonAccount
	entries    []LedgerEntry
	applied    *LedgerEntry
	applyErr   error
	listLedger []string
}

func (f *fakeLessonRepo) ListAccounts(context.Context) ([]LessonAccount, error) {
	if f.account == nil {
		return nil, nil
	}
	return []LessonAccount{*f.account}, nil
}

func (f *fakeLessonRepo) GetAccount(context.Context, string) (*LessonAccount, error) {
	if f.account == nil {
		return nil, ErrLessonAccountNotFound
	}
	return f.account, nil
}

func (f *fakeLessonRepo) ListLedger(_ context.Context, studentID string, _, _ *time.Time) ([]LedgerEntry, error) {
	f.listLedger = append(f.listLedger, studentID)
	return f.entries, nil
}

func (f *fakeLessonRepo) ApplyLedger(_ context.Context, entry *LedgerEntry) (*LessonAccount, error) {
	f.applied = entry
	if f.applyErr != nil {
		return nil, f.applyErr
	}
	if f.account == nil {
		f.account = &LessonAccount{StudentID: entry.StudentID}
	}
	f.account.BalanceHours += entry.DeltaHours
	return f.account, nil
}

func TestPurchaseValidation(t *testing.T) {
	repo := &fakeLessonRepo{}
	uc := NewLessonUsecase(repo)

	cases := []struct {
		name  string
		hours float64
		ok    bool
	}{
		{"正常", 10, true},
		{"两位小数", 2.55, true},
		{"零", 0, false},
		{"负数", -1, false},
		{"超上限", maxLedgerHours + 1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := uc.Purchase(context.Background(), "s1", "staff1", tc.hours, "秋季课包")
			if tc.ok && err != nil {
				t.Fatalf("want ok, got %v", err)
			}
			if !tc.ok && !errors.Is(err, ErrInvalidLedger) {
				t.Fatalf("want ErrInvalidLedger, got %v", err)
			}
		})
	}
}

func TestAdjustRequiresReasonAndNonZero(t *testing.T) {
	repo := &fakeLessonRepo{}
	uc := NewLessonUsecase(repo)

	if _, err := uc.Adjust(context.Background(), "s1", "staff1", 0, "纠正"); !errors.Is(err, ErrInvalidLedger) {
		t.Fatalf("zero delta error = %v, want ErrInvalidLedger", err)
	}
	if _, err := uc.Adjust(context.Background(), "s1", "staff1", 1, "   "); !errors.Is(err, ErrLedgerReasonRequired) {
		t.Fatalf("empty reason error = %v, want ErrLedgerReasonRequired", err)
	}
	account, err := uc.Adjust(context.Background(), "s1", "staff1", -1.5, "上周多算一次")
	if err != nil {
		t.Fatalf("Adjust: %v", err)
	}
	if repo.applied.Type != LedgerAdjust || repo.applied.DeltaHours != -1.5 {
		t.Fatalf("unexpected ledger: %+v", repo.applied)
	}
	if repo.applied.Reason != "上周多算一次" {
		t.Fatalf("reason = %q", repo.applied.Reason)
	}
	if account.BalanceHours != -1.5 {
		t.Fatalf("balance = %v", account.BalanceHours)
	}
}

func TestPurchaseWritesPositiveLedger(t *testing.T) {
	repo := &fakeLessonRepo{}
	uc := NewLessonUsecase(repo)
	if _, err := uc.Purchase(context.Background(), "s1", "staff1", 12, "秋季 12 次"); err != nil {
		t.Fatalf("Purchase: %v", err)
	}
	if repo.applied.Type != LedgerPurchase || repo.applied.DeltaHours != 12 || repo.applied.RefType != "manual" {
		t.Fatalf("unexpected ledger: %+v", repo.applied)
	}
	if repo.applied.OperatorID != "staff1" {
		t.Fatalf("operator = %q", repo.applied.OperatorID)
	}
}

func TestDetailPropagatesNotFound(t *testing.T) {
	uc := NewLessonUsecase(&fakeLessonRepo{})
	if _, _, err := uc.Detail(context.Background(), "missing"); !errors.Is(err, ErrLessonAccountNotFound) {
		t.Fatalf("error = %v, want ErrLessonAccountNotFound", err)
	}
	if _, _, err := uc.Detail(context.Background(), "   "); !errors.Is(err, ErrLessonAccountNotFound) {
		t.Fatalf("blank id error = %v", err)
	}
}

func TestLowBalanceFlag(t *testing.T) {
	cases := []struct {
		balance float64
		low     bool
	}{
		{0, true},
		{2, true},
		{2.01, false},
		{10, false},
	}
	for _, tc := range cases {
		account := &LessonAccount{BalanceHours: tc.balance}
		if account.LowBalance() != tc.low {
			t.Fatalf("balance %v low = %v, want %v", tc.balance, account.LowBalance(), tc.low)
		}
	}
}
