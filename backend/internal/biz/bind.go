package biz

import (
	"context"
	"errors"
	"regexp"
	"strings"
)

var (
	ErrInvalidPhone = errors.New("invalid phone")
	ErrBindDisabled = errors.New("bind disabled")
)

var phonePattern = regexp.MustCompile(`^1\d{10}$`)

type BindRepo interface {
	RequestByPhone(ctx context.Context, phone string) (string, error)
}

type BindUsecase struct {
	repo BindRepo
}

func NewBindUsecase(repo BindRepo) *BindUsecase {
	return &BindUsecase{repo: repo}
}

// Request 记录家长的绑定申请；不做自动通过，统一由老师审批。
func (uc *BindUsecase) Request(ctx context.Context, phone string) (string, error) {
	if uc == nil || uc.repo == nil {
		return "", ErrBindDisabled
	}
	phone = strings.TrimSpace(phone)
	if !phonePattern.MatchString(phone) {
		return "", ErrInvalidPhone
	}
	return uc.repo.RequestByPhone(ctx, phone)
}
