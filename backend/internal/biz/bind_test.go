package biz

import (
	"context"
	"errors"
	"testing"
)

type fakeBindRepo struct {
	status string
	err    error
	phone  string
}

func (f *fakeBindRepo) RequestByPhone(_ context.Context, phone string) (string, error) {
	f.phone = phone
	return f.status, f.err
}

func TestBindRequestValidatesPhone(t *testing.T) {
	repo := &fakeBindRepo{status: "pending"}
	uc := NewBindUsecase(repo)

	if _, err := uc.Request(context.Background(), "123"); !errors.Is(err, ErrInvalidPhone) {
		t.Fatalf("want ErrInvalidPhone, got %v", err)
	}
	if _, err := uc.Request(context.Background(), " 18852390970 "); err != nil {
		t.Fatalf("valid phone rejected: %v", err)
	}
	if repo.phone != "18852390970" {
		t.Fatalf("phone not trimmed: %q", repo.phone)
	}
}

func TestBindRequestWithoutRepo(t *testing.T) {
	uc := NewBindUsecase(nil)
	if _, err := uc.Request(context.Background(), "18852390970"); !errors.Is(err, ErrBindDisabled) {
		t.Fatalf("want ErrBindDisabled, got %v", err)
	}
}
