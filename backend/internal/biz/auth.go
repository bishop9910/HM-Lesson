// Package biz 放业务用例。auth.go 负责多老师账号、登录与角色授权。
package biz

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidCredentials = errors.New("手机号或密码错误")
	ErrStaffDisabled      = errors.New("账号已停用")
	ErrPhoneTaken         = errors.New("手机号已被使用")
	ErrForbidden          = errors.New("没有权限执行该操作")
	ErrStaffNotFound      = errors.New("老师不存在")
)

const (
	RoleOwner     = "owner"
	RoleAssistant = "assistant"

	StatusActive   = "active"
	StatusDisabled = "disabled"
)

type Staff struct {
	ID           string
	Name         string
	Phone        string
	PasswordHash string
	Role         string
	Status       string
	CreatedBy    string
}

// StaffRepo 是账号存储接口；data 层实现，biz 不感知 SQL。
type StaffRepo interface {
	Count(ctx context.Context) (int64, error)
	Create(ctx context.Context, s *Staff) error
	Update(ctx context.Context, s *Staff) error
	ByPhone(ctx context.Context, phone string) (*Staff, error)
	ByID(ctx context.Context, id string) (*Staff, error)
	List(ctx context.Context) ([]*Staff, error)
}

type PasswordHasher interface {
	Hash(plain string) (string, error)
	Verify(encoded, plain string) bool
}

type TokenIssuer interface {
	Issue(subject, role, name, jti string) (string, time.Time, error)
}

type AuthUsecase struct {
	repo   StaffRepo
	hasher PasswordHasher
	tokens TokenIssuer
	now    func() time.Time
}

func NewAuthUsecase(repo StaffRepo, hasher PasswordHasher, tokens TokenIssuer) *AuthUsecase {
	return &AuthUsecase{repo: repo, hasher: hasher, tokens: tokens, now: time.Now}
}

// Login 校验手机号+密码；停用账号一律按凭据错误处理，避免枚举。
func (uc *AuthUsecase) Login(ctx context.Context, phone, plain string) (*Staff, string, time.Time, error) {
	phone = strings.TrimSpace(phone)
	if phone == "" || plain == "" {
		return nil, "", time.Time{}, ErrInvalidCredentials
	}
	staff, err := uc.repo.ByPhone(ctx, phone)
	if err != nil {
		if errors.Is(err, ErrStaffNotFound) {
			return nil, "", time.Time{}, ErrInvalidCredentials
		}
		return nil, "", time.Time{}, err
	}
	if staff.Status != StatusActive || !uc.hasher.Verify(staff.PasswordHash, plain) {
		return nil, "", time.Time{}, ErrInvalidCredentials
	}
	jti := randomID()
	tok, exp, err := uc.tokens.Issue(staff.ID, staff.Role, staff.Name, jti)
	if err != nil {
		return nil, "", time.Time{}, err
	}
	return staff, tok, exp, nil
}

func (uc *AuthUsecase) Me(ctx context.Context, staffID string) (*Staff, error) {
	staff, err := uc.repo.ByID(ctx, staffID)
	if err != nil {
		return nil, err
	}
	if staff.Status != StatusActive {
		return nil, ErrStaffDisabled
	}
	return staff, nil
}

type CreateStaffInput struct {
	Name     string
	Phone    string
	Password string
	Role     string
}

// CreateStaff 仅 owner 可建老师；assistant 一律拒绝。
func (uc *AuthUsecase) CreateStaff(ctx context.Context, actor *Staff, in CreateStaffInput) (*Staff, error) {
	if actor == nil || actor.Role != RoleOwner {
		return nil, ErrForbidden
	}
	name := strings.TrimSpace(in.Name)
	phone := strings.TrimSpace(in.Phone)
	if name == "" || phone == "" || len(in.Password) < 8 {
		return nil, errors.New("姓名、手机号必填，密码至少 8 位")
	}
	role := in.Role
	if role == "" {
		role = RoleAssistant
	}
	if role != RoleOwner && role != RoleAssistant {
		return nil, errors.New("角色只能是 owner 或 assistant")
	}
	if existing, err := uc.repo.ByPhone(ctx, phone); err == nil && existing != nil {
		return nil, ErrPhoneTaken
	} else if err != nil && !errors.Is(err, ErrStaffNotFound) {
		return nil, err
	}
	hash, err := uc.hasher.Hash(in.Password)
	if err != nil {
		return nil, err
	}
	staff := &Staff{
		ID:           randomID(),
		Name:         name,
		Phone:        phone,
		PasswordHash: hash,
		Role:         role,
		Status:       StatusActive,
		CreatedBy:    actor.ID,
	}
	if err := uc.repo.Create(ctx, staff); err != nil {
		return nil, err
	}
	return staff, nil
}

func (uc *AuthUsecase) ListStaff(ctx context.Context, actor *Staff) ([]*Staff, error) {
	if actor == nil || actor.Role != RoleOwner {
		return nil, ErrForbidden
	}
	return uc.repo.List(ctx)
}

type UpdateStaffInput struct {
	Name     *string
	Role     *string
	Status   *string
	Password *string
}

func (uc *AuthUsecase) UpdateStaff(ctx context.Context, actor *Staff, id string, in UpdateStaffInput) (*Staff, error) {
	if actor == nil || actor.Role != RoleOwner {
		return nil, ErrForbidden
	}
	target, err := uc.repo.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// 防止唯一负责人把自己降级/停用后无人能管理系统。
	if target.ID == actor.ID {
		if in.Role != nil && *in.Role != RoleOwner {
			return nil, errors.New("不能把自己降级为非负责人")
		}
		if in.Status != nil && *in.Status != StatusActive {
			return nil, errors.New("不能停用自己的账号")
		}
	}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return nil, errors.New("姓名不能为空")
		}
		target.Name = name
	}
	if in.Role != nil {
		if *in.Role != RoleOwner && *in.Role != RoleAssistant {
			return nil, errors.New("角色只能是 owner 或 assistant")
		}
		target.Role = *in.Role
	}
	if in.Status != nil {
		if *in.Status != StatusActive && *in.Status != StatusDisabled {
			return nil, errors.New("状态只能是 active 或 disabled")
		}
		target.Status = *in.Status
	}
	if in.Password != nil {
		if len(*in.Password) < 8 {
			return nil, errors.New("密码至少 8 位")
		}
		hash, err := uc.hasher.Hash(*in.Password)
		if err != nil {
			return nil, err
		}
		target.PasswordHash = hash
	}
	if err := uc.repo.Update(ctx, target); err != nil {
		return nil, err
	}
	return target, nil
}

func (uc *AuthUsecase) Bootstrap(ctx context.Context, name, phone, plain string) (*Staff, bool, error) {
	count, err := uc.repo.Count(ctx)
	if err != nil {
		return nil, false, err
	}
	if count > 0 {
		return nil, false, nil
	}
	if strings.TrimSpace(phone) == "" || plain == "" {
		return nil, false, nil
	}
	hash, err := uc.hasher.Hash(plain)
	if err != nil {
		return nil, false, err
	}
	staff := &Staff{
		ID:           randomID(),
		Name:         strings.TrimSpace(name),
		Phone:        strings.TrimSpace(phone),
		PasswordHash: hash,
		Role:         RoleOwner,
		Status:       StatusActive,
	}
	if staff.Name == "" {
		staff.Name = "老师"
	}
	if err := uc.repo.Create(ctx, staff); err != nil {
		return nil, false, err
	}
	return staff, true, nil
}
