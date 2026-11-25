package biz

import (
	"context"
	"errors"
	"testing"
)

// stubCRM 只实现家长密码相关方法；其余方法由嵌入的 nil 接口占位（调用会 panic，测试里不会触发）。
type stubCRM struct {
	CRMRepo
	hash          map[string]string
	approveHash   string
	approvedTo    string
	setHash       string
	setFor        string
	touched       string
	passwordCalls int
}

func (s *stubCRM) ApproveGuardian(_ context.Context, guardianID, studentID, actorID, passwordHash string) error {
	s.approveHash = passwordHash
	s.approvedTo = guardianID
	return nil
}

func (s *stubCRM) SetGuardianPassword(_ context.Context, guardianID, passwordHash string) error {
	s.setFor = guardianID
	s.setHash = passwordHash
	s.passwordCalls++
	return nil
}

func (s *stubCRM) GuardianPasswordHash(_ context.Context, guardianID string) (string, error) {
	if h, ok := s.hash[guardianID]; ok {
		return h, nil
	}
	return "", ErrGuardianNotFound
}

func (s *stubCRM) TouchGuardianLogin(_ context.Context, guardianID string) error {
	s.touched = guardianID
	return nil
}

func newGuardianUsecase(repo CRMRepo) *CRMUsecase {
	return NewCRMUsecase(repo, fakeHasher{})
}

func TestValidGuardianPassword(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"12345", false},
		{"123456", true},
		{"abcdefghijklmnopqrstuvwxyz123456", true},
		{"abcdefghijklmnopqrstuvwxyz1234567", false},
	}
	for _, c := range cases {
		if got := validGuardianPassword(c.in); got != c.want {
			t.Fatalf("validGuardianPassword(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestNewTemporaryPassword(t *testing.T) {
	a, b := NewTemporaryPassword(), NewTemporaryPassword()
	if len(a) != 8 || len(b) != 8 {
		t.Fatalf("temporary password length = %d/%d, want 8", len(a), len(b))
	}
	if a == b {
		t.Fatalf("two generated passwords should differ, both %q", a)
	}
	for _, r := range a {
		if r == '0' || r == 'O' || r == '1' || r == 'l' || r == 'I' {
			t.Fatalf("temporary password contains ambiguous char %q", r)
		}
	}
}

func TestApproveGuardianHashesPassword(t *testing.T) {
	repo := &stubCRM{}
	uc := newGuardianUsecase(repo)
	actor := &Staff{ID: "staff-1", Role: RoleOwner}
	if err := uc.ApproveGuardian(context.Background(), actor, "g-1", "s-1", "abc"); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("short password error = %v, want ErrWeakPassword", err)
	}
	if err := uc.ApproveGuardian(context.Background(), actor, "g-1", "s-1", "abc123"); err != nil {
		t.Fatalf("ApproveGuardian: %v", err)
	}
	if repo.approvedTo != "g-1" || repo.approveHash != "hash:abc123" {
		t.Fatalf("approve got id=%q hash=%q", repo.approvedTo, repo.approveHash)
	}
}

func TestChangeGuardianPassword(t *testing.T) {
	ctx := context.Background()
	repo := &stubCRM{hash: map[string]string{"g-1": "hash:old123"}}
	uc := newGuardianUsecase(repo)

	// 旧密码错误。
	if err := uc.ChangeGuardianPassword(ctx, "g-1", "wrong", "newpass1"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("wrong old password error = %v, want ErrWrongPassword", err)
	}
	// 新密码太短。
	if err := uc.ChangeGuardianPassword(ctx, "g-1", "old123", "123"); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("weak new password error = %v, want ErrWeakPassword", err)
	}
	// 正确改密。
	if err := uc.ChangeGuardianPassword(ctx, "g-1", "old123", "newpass1"); err != nil {
		t.Fatalf("ChangeGuardianPassword: %v", err)
	}
	if repo.setHash != "hash:newpass1" {
		t.Fatalf("stored hash = %q, want hash:newpass1", repo.setHash)
	}
	// 首次设置：当前没有密码时不要求旧密码。
	repo2 := &stubCRM{hash: map[string]string{"g-2": ""}}
	uc2 := newGuardianUsecase(repo2)
	if err := uc2.ChangeGuardianPassword(ctx, "g-2", "", "first123"); err != nil {
		t.Fatalf("first-time set: %v", err)
	}
	// 账号不存在。
	if err := uc.ChangeGuardianPassword(ctx, "missing", "", "first123"); !errors.Is(err, ErrGuardianNotFound) {
		t.Fatalf("missing guardian error = %v, want ErrGuardianNotFound", err)
	}
}

func TestVerifyGuardianPassword(t *testing.T) {
	ctx := context.Background()
	repo := &stubCRM{}
	uc := newGuardianUsecase(repo)

	if err := uc.VerifyGuardianPassword(ctx, &GuardianRequest{}, "x"); !errors.Is(err, ErrPasswordNotSet) {
		t.Fatalf("no password error = %v, want ErrPasswordNotSet", err)
	}
	g := &GuardianRequest{ID: "g-1", PasswordHash: "hash:secret1"}
	if err := uc.VerifyGuardianPassword(ctx, g, "bad"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("bad password error = %v, want ErrWrongPassword", err)
	}
	if err := uc.VerifyGuardianPassword(ctx, g, "secret1"); err != nil {
		t.Fatalf("VerifyGuardianPassword: %v", err)
	}
	if repo.touched != "g-1" {
		t.Fatalf("login timestamp not recorded, got %q", repo.touched)
	}
}

func TestSetInitialGuardianPassword(t *testing.T) {
	ctx := context.Background()
	repo := &stubCRM{hash: map[string]string{"g-1": "hash:already"}}
	uc := newGuardianUsecase(repo)
	if err := uc.SetInitialGuardianPassword(ctx, "g-1", "another1"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("existing password error = %v, want ErrWrongPassword", err)
	}
	repo2 := &stubCRM{hash: map[string]string{"g-2": ""}}
	uc2 := newGuardianUsecase(repo2)
	if err := uc2.SetInitialGuardianPassword(ctx, "g-2", "first123"); err != nil {
		t.Fatalf("SetInitialGuardianPassword: %v", err)
	}
}

func TestDeleteGuardianRequiresOwner(t *testing.T) {
	repo := &stubCRM{}
	uc := newGuardianUsecase(repo)
	if err := uc.DeleteGuardian(context.Background(), &Staff{Role: RoleAssistant}, "g-1"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("assistant delete error = %v, want ErrForbidden", err)
	}
	if err := uc.DeleteGuardian(context.Background(), &Staff{Role: RoleOwner}, ""); !errors.Is(err, ErrGuardianNotFound) {
		t.Fatalf("empty id error = %v, want ErrGuardianNotFound", err)
	}
}

func TestGuardiansRejectsUnknownStatus(t *testing.T) {
	uc := newGuardianUsecase(&stubCRM{})
	if _, err := uc.Guardians(context.Background(), "weird"); err == nil {
		t.Fatal("unknown status should be rejected")
	}
}

func TestResetGuardianPasswordValidates(t *testing.T) {
	repo := &stubCRM{}
	uc := newGuardianUsecase(repo)
	owner := &Staff{ID: "staff-1", Role: RoleOwner}
	if err := uc.ResetGuardianPassword(context.Background(), owner, "g-1", "1"); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("weak reset password error = %v, want ErrWeakPassword", err)
	}
	if err := uc.ResetGuardianPassword(context.Background(), owner, "g-1", "reset123"); err != nil {
		t.Fatalf("ResetGuardianPassword: %v", err)
	}
	if repo.setHash != "hash:reset123" || repo.setFor != "g-1" {
		t.Fatalf("reset got id=%q hash=%q", repo.setFor, repo.setHash)
	}
}
