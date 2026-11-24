package biz

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrStudentNotFound  = errors.New("student not found")
	ErrGuardianNotFound = errors.New("guardian not found")
	ErrInvalidStudent   = errors.New("invalid student")
	ErrInvalidFollowUp  = errors.New("invalid follow up")
	ErrInvalidCode      = errors.New("invalid binding code")
	ErrCodeExpired      = errors.New("binding code expired")
	ErrCodeUsed         = errors.New("binding code used")
	ErrGuardianDisabled = errors.New("guardian disabled")
	ErrBindPending      = errors.New("guardian pending")
)

var (
	studentGroups = map[string]bool{"class_active": true, "one_on_one": true, "intent": true, "visitor": true}
	studentStates = map[string]bool{"active": true, "graduated": true, "lost": true}
	followMethods = map[string]bool{"wechat": true, "phone": true, "offline": true, "other": true}
)

type Student struct {
	ID             string
	Name           string
	Grade          string
	School         string
	GroupCode      string
	Status         string
	Tags           []string
	NextFollowUpAt *time.Time
	Note           string
	CreatedBy      string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type FollowUp struct {
	ID           string
	StudentID    string
	Method       string
	Content      string
	Concern      string
	NextAction   string
	NextFollowUp *time.Time
	CreatedBy    string
	CreatedAt    time.Time
}

type GuardianRequest struct {
	ID        string
	Phone     string
	Name      string
	Status    string
	CreatedAt time.Time
}

type BindingCode struct {
	Code      string
	StudentID string
	ExpiresAt time.Time
}

type PortalHome struct {
	Students      []Student
	PendingTasks  int
	BalanceHours  float64
	GuardianName  string
	GuardianPhone string
}

// PortalTask 是家长端展示的待办作业摘要。
type PortalTask struct {
	ID     string
	Title  string
	Status string
	DueAt  *time.Time
}

// PortalLesson 是家长端展示的课时流水。
type PortalLesson struct {
	ID           string
	Kind         string
	DeltaHours   float64
	BalanceAfter float64
	Note         string
	CreatedAt    time.Time
}

// PortalStudent 是家长端单个学员的详情聚合。
type PortalStudent struct {
	Student      Student
	Tasks        []PortalTask
	BalanceHours float64
	ExpiresAt    *time.Time
	Lessons      []PortalLesson
}

type StudentFilter struct {
	Group    string
	Status   string
	Query    string
	Page     int
	PageSize int
}

type UpdateStudentInput struct {
	Name              *string
	Grade             *string
	School            *string
	GroupCode         *string
	Status            *string
	Tags              *[]string
	NextFollowUpAt    *time.Time
	ClearNextFollowUp bool
	Note              *string
}

type CreateStudentInput struct {
	Name           string
	Grade          string
	School         string
	GroupCode      string
	Status         string
	Tags           []string
	NextFollowUpAt *time.Time
	Note           string
}

// Notification 是家长端站内通知，用于作业发布/批阅/打回弹窗。
type Notification struct {
	ID        string
	Kind      string
	Title     string
	Body      string
	RefType   string
	RefID     string
	CreatedAt time.Time
	ReadAt    *time.Time
}

type CRMRepo interface {
	ListStudents(ctx context.Context, f StudentFilter) ([]Student, int, error)
	GetStudent(ctx context.Context, id string) (*Student, error)
	CreateStudent(ctx context.Context, s *Student) (*Student, error)
	UpdateStudent(ctx context.Context, id string, in UpdateStudentInput) (*Student, error)
	AddFollowUp(ctx context.Context, f *FollowUp) (*FollowUp, error)
	ListFollowUps(ctx context.Context, studentID string, limit int) ([]FollowUp, error)
	ListPendingGuardians(ctx context.Context) ([]GuardianRequest, error)
	ApproveGuardian(ctx context.Context, guardianID, studentID, actorID string) error
	RejectGuardian(ctx context.Context, guardianID, reason, actorID string) error
	CreateBindingCode(ctx context.Context, studentID, actorID string, ttl time.Duration) (*BindingCode, error)
	RedeemBindingCode(ctx context.Context, phone, code string) (*GuardianRequest, []Student, error)
	GuardianByPhone(ctx context.Context, phone string) (*GuardianRequest, error)
	PortalHome(ctx context.Context, guardianID string) (*PortalHome, error)
	PortalStudent(ctx context.Context, guardianID, studentID string) (*PortalStudent, error)
	UnreadNotifications(ctx context.Context, guardianID string, limit int) ([]Notification, error)
	MarkNotificationRead(ctx context.Context, guardianID, id string) error
	MarkAllNotificationsRead(ctx context.Context, guardianID string) error
}

type CRMUsecase struct {
	repo CRMRepo
}

func NewCRMUsecase(repo CRMRepo) *CRMUsecase {
	return &CRMUsecase{repo: repo}
}

func (uc *CRMUsecase) ListStudents(ctx context.Context, f StudentFilter) ([]Student, int, error) {
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.PageSize <= 0 || f.PageSize > 50 {
		f.PageSize = 20
	}
	if f.Group != "" && !studentGroups[f.Group] {
		return nil, 0, ErrInvalidStudent
	}
	if f.Status != "" && !studentStates[f.Status] {
		return nil, 0, ErrInvalidStudent
	}
	return uc.repo.ListStudents(ctx, f)
}

func (uc *CRMUsecase) GetStudent(ctx context.Context, id string) (*Student, error) {
	return uc.repo.GetStudent(ctx, id)
}

func (uc *CRMUsecase) CreateStudent(ctx context.Context, actor *Staff, in CreateStudentInput) (*Student, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Grade = strings.TrimSpace(in.Grade)
	in.School = strings.TrimSpace(in.School)
	if in.Name == "" || len([]rune(in.Name)) > 64 || in.Grade == "" {
		return nil, ErrInvalidStudent
	}
	if in.GroupCode == "" {
		in.GroupCode = "visitor"
	}
	if in.Status == "" {
		in.Status = "active"
	}
	if !studentGroups[in.GroupCode] || !studentStates[in.Status] {
		return nil, ErrInvalidStudent
	}
	if len(in.Tags) > 8 {
		return nil, ErrInvalidStudent
	}
	now := time.Now().UTC()
	return uc.repo.CreateStudent(ctx, &Student{
		ID: NewID(), Name: in.Name, Grade: in.Grade, School: in.School,
		GroupCode: in.GroupCode, Status: in.Status, Tags: in.Tags,
		NextFollowUpAt: in.NextFollowUpAt, Note: strings.TrimSpace(in.Note),
		CreatedBy: actor.ID, CreatedAt: now, UpdatedAt: now,
	})
}

func (uc *CRMUsecase) UpdateStudent(ctx context.Context, id string, in UpdateStudentInput) (*Student, error) {
	if in.Name != nil {
		v := strings.TrimSpace(*in.Name)
		if v == "" || len([]rune(v)) > 64 {
			return nil, ErrInvalidStudent
		}
		in.Name = &v
	}
	if in.GroupCode != nil && !studentGroups[*in.GroupCode] {
		return nil, ErrInvalidStudent
	}
	if in.Status != nil && !studentStates[*in.Status] {
		return nil, ErrInvalidStudent
	}
	if in.Tags != nil && len(*in.Tags) > 8 {
		return nil, ErrInvalidStudent
	}
	return uc.repo.UpdateStudent(ctx, id, in)
}

func (uc *CRMUsecase) AddFollowUp(ctx context.Context, actor *Staff, studentID, method, content, concern, nextAction string, next *time.Time) (*FollowUp, error) {
	if !followMethods[method] {
		return nil, ErrInvalidFollowUp
	}
	content = strings.TrimSpace(content)
	if content == "" || len([]rune(content)) > 2000 {
		return nil, ErrInvalidFollowUp
	}
	return uc.repo.AddFollowUp(ctx, &FollowUp{
		ID: NewID(), StudentID: studentID, Method: method, Content: content,
		Concern: strings.TrimSpace(concern), NextAction: strings.TrimSpace(nextAction),
		NextFollowUp: next, CreatedBy: actor.ID, CreatedAt: time.Now().UTC(),
	})
}

func (uc *CRMUsecase) ListFollowUps(ctx context.Context, studentID string, limit int) ([]FollowUp, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	return uc.repo.ListFollowUps(ctx, studentID, limit)
}

func (uc *CRMUsecase) PendingGuardians(ctx context.Context) ([]GuardianRequest, error) {
	return uc.repo.ListPendingGuardians(ctx)
}

func (uc *CRMUsecase) ApproveGuardian(ctx context.Context, actor *Staff, guardianID, studentID string) error {
	return uc.repo.ApproveGuardian(ctx, guardianID, studentID, actor.ID)
}

func (uc *CRMUsecase) RejectGuardian(ctx context.Context, actor *Staff, guardianID, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return ErrInvalidStudent
	}
	return uc.repo.RejectGuardian(ctx, guardianID, reason, actor.ID)
}

func (uc *CRMUsecase) CreateBindingCode(ctx context.Context, studentID, actorID string) (*BindingCode, error) {
	return uc.repo.CreateBindingCode(ctx, studentID, actorID, 7*24*time.Hour)
}

// GuardianLoginByCode 允许家长用手机号 + 绑定码直接完成绑定并登录。
func (uc *CRMUsecase) GuardianLoginByCode(ctx context.Context, phone, code string) (*GuardianRequest, []Student, error) {
	phone = strings.TrimSpace(phone)
	code = strings.TrimSpace(code)
	if !phonePattern.MatchString(phone) || len(code) != 6 {
		return nil, nil, ErrInvalidCode
	}
	return uc.repo.RedeemBindingCode(ctx, phone, code)
}

func (uc *CRMUsecase) GuardianByPhone(ctx context.Context, phone string) (*GuardianRequest, error) {
	return uc.repo.GuardianByPhone(ctx, phone)
}

func (uc *CRMUsecase) PortalHome(ctx context.Context, guardianID string) (*PortalHome, error) {
	return uc.repo.PortalHome(ctx, guardianID)
}

func (uc *CRMUsecase) PortalStudent(ctx context.Context, guardianID, studentID string) (*PortalStudent, error) {
	if strings.TrimSpace(studentID) == "" {
		return nil, ErrStudentNotFound
	}
	return uc.repo.PortalStudent(ctx, guardianID, studentID)
}

func (uc *CRMUsecase) UnreadNotifications(ctx context.Context, guardianID string, limit int) ([]Notification, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	return uc.repo.UnreadNotifications(ctx, guardianID, limit)
}

func (uc *CRMUsecase) MarkNotificationRead(ctx context.Context, guardianID, id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrGuardianNotFound
	}
	return uc.repo.MarkNotificationRead(ctx, guardianID, id)
}

func (uc *CRMUsecase) MarkAllNotificationsRead(ctx context.Context, guardianID string) error {
	return uc.repo.MarkAllNotificationsRead(ctx, guardianID)
}
