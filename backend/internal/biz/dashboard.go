package biz

import (
	"context"
	"errors"
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

type DashboardRepo interface {
	Summary(ctx context.Context) (DashboardSummary, error)
}

type DashboardUsecase struct {
	repo DashboardRepo
}

func NewDashboardUsecase(repo DashboardRepo) *DashboardUsecase {
	return &DashboardUsecase{repo: repo}
}

func (uc *DashboardUsecase) Summary(ctx context.Context) (DashboardSummary, error) {
	if uc == nil || uc.repo == nil {
		return DashboardSummary{}, ErrDashboardUnavailable
	}
	return uc.repo.Summary(ctx)
}
