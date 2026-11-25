package biz

import (
	"context"
	"errors"
	"testing"
)

// stubDeleteCRM 只实现学员删除相关方法；其余方法由嵌入的 nil 接口占位。
type stubDeleteCRM struct {
	CRMRepo
	stats     StudentDeleteStats
	deleted   string
	statsCall int
}

func (s *stubDeleteCRM) StudentDeleteStats(_ context.Context, studentID string) (StudentDeleteStats, error) {
	s.statsCall++
	return s.stats, nil
}

func (s *stubDeleteCRM) DeleteStudent(_ context.Context, studentID string) error {
	s.deleted = studentID
	return nil
}

func TestDeleteStudentRequiresOwner(t *testing.T) {
	repo := &stubDeleteCRM{}
	uc := NewCRMUsecase(repo, nil)
	if _, err := uc.DeleteStudent(context.Background(), &Staff{Role: RoleAssistant}, "s1", true); !errors.Is(err, ErrForbidden) {
		t.Fatalf("assistant should be rejected, got %v", err)
	}
	if repo.statsCall != 0 || repo.deleted != "" {
		t.Fatalf("repo must not be touched: %+v", repo)
	}
}

func TestDeleteStudentBlocksWhenLedgerExists(t *testing.T) {
	repo := &stubDeleteCRM{stats: StudentDeleteStats{TaskAssignments: 3, Bookings: 1, LessonLedger: 2, Guardians: 1}}
	uc := NewCRMUsecase(repo, nil)
	stats, err := uc.DeleteStudent(context.Background(), &Staff{Role: RoleOwner}, "s1", false)
	if !errors.Is(err, ErrStudentHasLedger) {
		t.Fatalf("want ErrStudentHasLedger, got %v", err)
	}
	if stats.LessonLedger != 2 {
		t.Fatalf("stats should be returned for confirmation, got %+v", stats)
	}
	if repo.deleted != "" {
		t.Fatal("student must not be deleted without force")
	}
}

func TestDeleteStudentForceRemovesStudent(t *testing.T) {
	repo := &stubDeleteCRM{stats: StudentDeleteStats{LessonLedger: 5}}
	uc := NewCRMUsecase(repo, nil)
	stats, err := uc.DeleteStudent(context.Background(), &Staff{Role: RoleOwner}, "s9", true)
	if err != nil {
		t.Fatalf("force delete: %v", err)
	}
	if repo.deleted != "s9" {
		t.Fatalf("repo.DeleteStudent not called: %q", repo.deleted)
	}
	if stats.LessonLedger != 5 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestDeleteStudentWithoutLedgerNeedsNoForce(t *testing.T) {
	repo := &stubDeleteCRM{stats: StudentDeleteStats{TaskAssignments: 1}}
	uc := NewCRMUsecase(repo, nil)
	if _, err := uc.DeleteStudent(context.Background(), &Staff{Role: RoleOwner}, "s2", false); err != nil {
		t.Fatalf("clean student should delete without force: %v", err)
	}
	if repo.deleted != "s2" {
		t.Fatalf("repo.DeleteStudent not called: %q", repo.deleted)
	}
}

// fakeSystemStaffRepo 供 EnsureSystem 测试：ByPhone 可返回不存在，Create/Update 记录调用。
type fakeSystemStaffRepo struct {
	StaffRepo
	current *Staff
	created *Staff
	updated *Staff
}

func (f *fakeSystemStaffRepo) ByPhone(_ context.Context, phone string) (*Staff, error) {
	if f.current == nil {
		return nil, ErrStaffNotFound
	}
	if f.current.Phone != phone {
		return nil, ErrStaffNotFound
	}
	return f.current, nil
}

func (f *fakeSystemStaffRepo) Create(_ context.Context, s *Staff) error {
	f.created = s
	return nil
}

func (f *fakeSystemStaffRepo) Update(_ context.Context, s *Staff) error {
	f.updated = s
	return nil
}

// fakeHasher 复用 auth_test.go 的定义（hash: 前缀 + 明文），这里不再重复声明。

func TestEnsureSystemCreatesHiddenOwner(t *testing.T) {
	repo := &fakeSystemStaffRepo{}
	uc := NewAuthUsecase(repo, fakeHasher{}, nil)
	staff, created, err := uc.EnsureSystem(context.Background(), "s3cret-key")
	if err != nil {
		t.Fatalf("EnsureSystem: %v", err)
	}
	if !created || staff == nil {
		t.Fatal("system account should be created")
	}
	if repo.created == nil {
		t.Fatal("repo.Create not called")
	}
	if !repo.created.Hidden {
		t.Fatal("system account must be hidden")
	}
	if repo.created.Role != RoleOwner || repo.created.Status != StatusActive {
		t.Fatalf("system account must be active owner: %+v", repo.created)
	}
	if repo.created.Phone != SystemPhone {
		t.Fatalf("system account username must be fixed: %q", repo.created.Phone)
	}
	if repo.created.PasswordHash != "hash:s3cret-key" {
		t.Fatalf("password must be hashed, got %q", repo.created.PasswordHash)
	}
}

func TestEnsureSystemRepairsFlagsWithoutOverwritingPassword(t *testing.T) {
	repo := &fakeSystemStaffRepo{current: &Staff{
		ID: "sys-1", Name: "x", Phone: SystemPhone, Role: RoleAssistant,
		Status: StatusDisabled, Hidden: false, PasswordHash: "hash:kept",
	}}
	uc := NewAuthUsecase(repo, fakeHasher{}, nil)
	staff, created, err := uc.EnsureSystem(context.Background(), "env-secret")
	if err != nil || created {
		t.Fatalf("should repair existing account: created=%v err=%v", created, err)
	}
	if !staff.Hidden || staff.Role != RoleOwner || staff.Status != StatusActive {
		t.Fatalf("flags not repaired: %+v", staff)
	}
	if staff.PasswordHash != "hash:kept" {
		t.Fatalf("existing password must not be overwritten: %q", staff.PasswordHash)
	}
	if repo.updated == nil {
		t.Fatal("repo.Update not called")
	}
}

func TestEnsureSystemSetsMissingPasswordFromEnv(t *testing.T) {
	repo := &fakeSystemStaffRepo{current: &Staff{
		ID: "sys-2", Phone: SystemPhone, Role: RoleOwner, Status: StatusActive, Hidden: true,
	}}
	uc := NewAuthUsecase(repo, fakeHasher{}, nil)
	staff, _, err := uc.EnsureSystem(context.Background(), "from-env")
	if err != nil {
		t.Fatalf("EnsureSystem: %v", err)
	}
	if staff.PasswordHash != "hash:from-env" {
		t.Fatalf("empty password should be filled from env: %q", staff.PasswordHash)
	}
}
