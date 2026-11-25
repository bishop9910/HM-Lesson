package biz

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubClassRepo struct {
	ClassRepo
	created   *Class
	members   []string
	session   *ClassSession
	memberErr error
}

func (s *stubClassRepo) ListClasses(context.Context, string) ([]Class, error) { return nil, nil }

func (s *stubClassRepo) GetClass(context.Context, string) (*Class, error) {
	return &Class{ID: "c1", Status: ClassActive}, nil
}

func (s *stubClassRepo) CreateClass(_ context.Context, class *Class) error {
	s.created = class
	return nil
}

func (s *stubClassRepo) UpdateClass(context.Context, *Class) error { return nil }

func (s *stubClassRepo) ListMembers(context.Context, string) ([]ClassMember, error) {
	return nil, nil
}

func (s *stubClassRepo) AddMembers(_ context.Context, _ string, ids []string) (int, error) {
	if s.memberErr != nil {
		return 0, s.memberErr
	}
	s.members = ids
	return len(ids), nil
}

func (s *stubClassRepo) RemoveMember(context.Context, string, string) error { return nil }

func (s *stubClassRepo) ListSessions(context.Context, string) ([]ClassSession, error) {
	return nil, nil
}

func (s *stubClassRepo) CreateSession(_ context.Context, session *ClassSession) error {
	s.session = session
	return nil
}

func (s *stubClassRepo) DeleteSession(context.Context, string) error { return nil }

func TestCreateClassValidation(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		in   ClassInput
		ok   bool
	}{
		{"默认学科", ClassInput{Name: "高一物理 A 班"}, true},
		{"缺名称", ClassInput{}, false},
		{"日期倒挂", ClassInput{Name: "班", StartDate: &start, EndDate: &end}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uc := NewClassUsecase(&stubClassRepo{})
			_, err := uc.Create(context.Background(), "s1", tc.in)
			if tc.ok && err != nil {
				t.Fatalf("期望成功，得到 %v", err)
			}
			if !tc.ok && !errors.Is(err, ErrInvalidClass) {
				t.Fatalf("期望 ErrInvalidClass，得到 %v", err)
			}
		})
	}
}

func TestCreateClassDefaultsSubject(t *testing.T) {
	repo := &stubClassRepo{}
	uc := NewClassUsecase(repo)
	if _, err := uc.Create(context.Background(), "s1", ClassInput{Name: "班"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if repo.created == nil || repo.created.Subject != "physics" || repo.created.Status != ClassActive {
		t.Fatalf("默认值不对: %+v", repo.created)
	}
	if repo.created.CreatedBy != "s1" || repo.created.ID == "" {
		t.Fatalf("缺少创建人或 id: %+v", repo.created)
	}
}

func TestAddMembersDedupAndRequireActive(t *testing.T) {
	repo := &stubClassRepo{}
	uc := NewClassUsecase(repo)
	if _, err := uc.AddMembers(context.Background(), "c1", []string{"s1", "s1", " "}); err != nil {
		t.Fatalf("AddMembers: %v", err)
	}
	if len(repo.members) != 1 || repo.members[0] != "s1" {
		t.Fatalf("应去重去空: %v", repo.members)
	}
	if _, err := uc.AddMembers(context.Background(), "c1", nil); !errors.Is(err, ErrInvalidClass) {
		t.Fatalf("空 id 应报 ErrInvalidClass，得到 %v", err)
	}
	repo.memberErr = ErrStudentNotActive
	if _, err := uc.AddMembers(context.Background(), "c1", []string{"s2"}); !errors.Is(err, ErrStudentNotActive) {
		t.Fatalf("期望 ErrStudentNotActive，得到 %v", err)
	}
}

func TestCreateSessionValidation(t *testing.T) {
	repo := &stubClassRepo{}
	uc := NewClassUsecase(repo)

	if _, err := uc.CreateSession(context.Background(), "s1", "c1", SessionInput{}); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("缺日期应报错，得到 %v", err)
	}
	if _, err := uc.CreateSession(context.Background(), "s1", "c1", SessionInput{
		SessionDate: time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC),
		Attendance:  map[string]string{"s1": "maybe"},
	}); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("非法出勤应报错，得到 %v", err)
	}
	session, err := uc.CreateSession(context.Background(), "s1", "c1", SessionInput{
		SessionDate: time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC),
		Content:     "  牛顿第二定律  ",
		Attendance:  map[string]string{"s1": AttendancePresent},
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if session.Content != "牛顿第二定律" || session.CreatedBy != "s1" {
		t.Fatalf("会话字段不对: %+v", session)
	}
	if len(session.Attendance) != 1 || session.Attendance[0].Status != AttendancePresent {
		t.Fatalf("出勤未落库: %+v", session.Attendance)
	}
}

func TestAttendanceLabel(t *testing.T) {
	cases := map[string]string{AttendancePresent: "出勤", AttendanceAbsent: "缺课", AttendanceLeave: "请假", "x": "x"}
	for status, want := range cases {
		if got := AttendanceLabel(status); got != want {
			t.Fatalf("AttendanceLabel(%q) = %q, want %q", status, got, want)
		}
	}
}
