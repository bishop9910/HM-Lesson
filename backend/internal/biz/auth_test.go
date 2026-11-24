package biz

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"
)

type fakeHasher struct{}

func (fakeHasher) Hash(plain string) (string, error) { return "hash:" + plain, nil }
func (fakeHasher) Verify(encoded, plain string) bool { return encoded == "hash:"+plain }

type fakeIssuer struct{}

func (fakeIssuer) Issue(subject, role, name, jti string) (string, time.Time, error) {
	return "token:" + subject + ":" + role, time.Now().Add(time.Hour), nil
}

type fakeStaffRepo struct {
	rows []*Staff
}

func (r *fakeStaffRepo) Count(context.Context) (int64, error) { return int64(len(r.rows)), nil }

func (r *fakeStaffRepo) Create(_ context.Context, s *Staff) error {
	for _, row := range r.rows {
		if row.Phone == s.Phone {
			return ErrPhoneTaken
		}
	}
	copied := *s
	r.rows = append(r.rows, &copied)
	return nil
}

func (r *fakeStaffRepo) Update(_ context.Context, s *Staff) error {
	for i, row := range r.rows {
		if row.ID == s.ID {
			copied := *s
			r.rows[i] = &copied
			return nil
		}
	}
	return ErrStaffNotFound
}

func (r *fakeStaffRepo) ByPhone(_ context.Context, phone string) (*Staff, error) {
	for _, row := range r.rows {
		if row.Phone == phone {
			copied := *row
			return &copied, nil
		}
	}
	return nil, ErrStaffNotFound
}

func (r *fakeStaffRepo) ByID(_ context.Context, id string) (*Staff, error) {
	for _, row := range r.rows {
		if row.ID == id {
			copied := *row
			return &copied, nil
		}
	}
	return nil, ErrStaffNotFound
}

func (r *fakeStaffRepo) List(context.Context) ([]*Staff, error) {
	out := make([]*Staff, 0, len(r.rows))
	for _, row := range r.rows {
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Phone < out[j].Phone })
	return out, nil
}

func newTestUsecase() (*AuthUsecase, *fakeStaffRepo) {
	repo := &fakeStaffRepo{}
	return NewAuthUsecase(repo, fakeHasher{}, fakeIssuer{}), repo
}

func seedOwner(t *testing.T, repo *fakeStaffRepo) *Staff {
	t.Helper()
	owner := &Staff{ID: "s-owner", Name: "王老师", Phone: "13800000000", PasswordHash: "hash:secret123", Role: RoleOwner, Status: StatusActive}
	if err := repo.Create(context.Background(), owner); err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	return owner
}

func TestLogin(t *testing.T) {
	uc, repo := newTestUsecase()
	seedOwner(t, repo)

	staff, tok, exp, err := uc.Login(context.Background(), "13800000000", "secret123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if staff.Role != RoleOwner || tok != "token:s-owner:owner" || exp.Before(time.Now()) {
		t.Fatalf("unexpected login result: %+v %q %v", staff, tok, exp)
	}

	cases := []struct {
		name  string
		phone string
		pass  string
	}{
		{"unknown phone", "13900000000", "secret123"},
		{"wrong password", "13800000000", "wrong"},
		{"blank", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, _, err := uc.Login(context.Background(), tc.phone, tc.pass); !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("error = %v, want ErrInvalidCredentials", err)
			}
		})
	}
}

func TestLoginDisabled(t *testing.T) {
	uc, repo := newTestUsecase()
	owner := seedOwner(t, repo)
	repo.rows[0].Status = StatusDisabled

	if _, _, _, err := uc.Login(context.Background(), owner.Phone, "secret123"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("error = %v, want ErrInvalidCredentials", err)
	}
}

func TestCreateStaff(t *testing.T) {
	uc, repo := newTestUsecase()
	owner := seedOwner(t, repo)

	created, err := uc.CreateStaff(context.Background(), owner, CreateStaffInput{
		Name: "李老师", Phone: "13900000000", Password: "password123",
	})
	if err != nil {
		t.Fatalf("CreateStaff: %v", err)
	}
	if created.Role != RoleAssistant || created.Status != StatusActive || created.CreatedBy != owner.ID {
		t.Fatalf("unexpected staff: %+v", created)
	}
	if created.PasswordHash == "password123" {
		t.Fatal("password stored in plaintext")
	}

	if _, err := uc.CreateStaff(context.Background(), owner, CreateStaffInput{
		Name: "重复", Phone: "13900000000", Password: "password123",
	}); !errors.Is(err, ErrPhoneTaken) {
		t.Fatalf("duplicate phone error = %v, want ErrPhoneTaken", err)
	}

	if _, err := uc.CreateStaff(context.Background(), owner, CreateStaffInput{
		Name: "短密码", Phone: "13700000000", Password: "short",
	}); err == nil {
		t.Fatal("CreateStaff accepted a short password")
	}

	assistant := created
	if _, err := uc.CreateStaff(context.Background(), assistant, CreateStaffInput{
		Name: "越权", Phone: "13600000000", Password: "password123",
	}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("assistant create error = %v, want ErrForbidden", err)
	}
}

func TestListStaffRequiresOwner(t *testing.T) {
	uc, repo := newTestUsecase()
	owner := seedOwner(t, repo)
	if _, err := uc.ListStaff(context.Background(), owner); err != nil {
		t.Fatalf("owner ListStaff: %v", err)
	}
	if _, err := uc.ListStaff(context.Background(), &Staff{Role: RoleAssistant}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("assistant ListStaff error = %v, want ErrForbidden", err)
	}
}

func TestUpdateStaff(t *testing.T) {
	uc, repo := newTestUsecase()
	owner := seedOwner(t, repo)
	assistant, err := uc.CreateStaff(context.Background(), owner, CreateStaffInput{
		Name: "李老师", Phone: "13900000000", Password: "password123",
	})
	if err != nil {
		t.Fatalf("CreateStaff: %v", err)
	}

	name := "李老师（改）"
	role := RoleOwner
	status := StatusDisabled
	password := "newpassword"
	updated, err := uc.UpdateStaff(context.Background(), owner, assistant.ID, UpdateStaffInput{
		Name: &name, Role: &role, Status: &status, Password: &password,
	})
	if err != nil {
		t.Fatalf("UpdateStaff: %v", err)
	}
	if updated.Name != name || updated.Role != role || updated.Status != status || updated.PasswordHash != "hash:"+password {
		t.Fatalf("unexpected updated staff: %+v", updated)
	}

	badStatus := "unknown"
	if _, err := uc.UpdateStaff(context.Background(), owner, assistant.ID, UpdateStaffInput{Status: &badStatus}); err == nil {
		t.Fatal("UpdateStaff accepted an unknown status")
	}
	short := "short"
	if _, err := uc.UpdateStaff(context.Background(), owner, assistant.ID, UpdateStaffInput{Password: &short}); err == nil {
		t.Fatal("UpdateStaff accepted a short password")
	}
	if _, err := uc.UpdateStaff(context.Background(), &Staff{Role: RoleAssistant}, assistant.ID, UpdateStaffInput{}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("assistant update error = %v, want ErrForbidden", err)
	}

	selfAssistant := RoleAssistant
	if _, err := uc.UpdateStaff(context.Background(), owner, owner.ID, UpdateStaffInput{Role: &selfAssistant}); err == nil {
		t.Fatal("owner demoted themselves without another owner")
	}
	selfDisabled := StatusDisabled
	if _, err := uc.UpdateStaff(context.Background(), owner, owner.ID, UpdateStaffInput{Status: &selfDisabled}); err == nil {
		t.Fatal("owner disabled their own account")
	}
}

func TestBootstrap(t *testing.T) {
	uc, repo := newTestUsecase()

	staff, ok, err := uc.Bootstrap(context.Background(), "", "", "")
	if err != nil || ok || staff != nil {
		t.Fatalf("empty bootstrap = %+v %v %v, want nil false nil", staff, ok, err)
	}

	staff, ok, err = uc.Bootstrap(context.Background(), "", "13800000000", "password123")
	if err != nil || !ok || staff == nil {
		t.Fatalf("bootstrap = %+v %v %v", staff, ok, err)
	}
	if staff.Role != RoleOwner || staff.Name != "老师" {
		t.Fatalf("unexpected bootstrap staff: %+v", staff)
	}

	if _, ok, err := uc.Bootstrap(context.Background(), "再来一个", "13900000000", "password123"); err != nil || ok {
		t.Fatalf("second bootstrap = ok:%v err:%v, want false nil", ok, err)
	}
	if len(repo.rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(repo.rows))
	}
}

func TestRandomIDIsUUIDv7(t *testing.T) {
	id := randomID()
	if len(id) != 36 || strings.Count(id, "-") != 4 {
		t.Fatalf("id = %q, want UUID shape", id)
	}
	if id[14] != '7' {
		t.Fatalf("version nibble = %c, want 7", id[14])
	}
	if !strings.ContainsRune("89ab", rune(id[19])) {
		t.Fatalf("variant nibble = %c, want 8/9/a/b", id[19])
	}
	if id == randomID() {
		t.Fatal("randomID returned the same value twice")
	}
}
