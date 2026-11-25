package biz

import (
	"context"
	"errors"
	"strings"
	"time"
)

// 班课状态（docs/backend-requirements.md §3.5）。
const (
	ClassActive   = "active"
	ClassArchived = "archived"
)

// 出勤状态。
const (
	AttendancePresent = "present"
	AttendanceAbsent  = "absent"
	AttendanceLeave   = "leave"
)

// AttendanceStatuses 是合法出勤值，顺序用于展示。
var AttendanceStatuses = []string{AttendancePresent, AttendanceAbsent, AttendanceLeave}

var (
	// ErrClassNotFound 班级不存在。
	ErrClassNotFound = errors.New("班级不存在")
	// ErrClassMemberNotFound 学员不在班级里。
	ErrClassMemberNotFound = errors.New("班级成员不存在")
	// ErrSessionNotFound 课次不存在。
	ErrSessionNotFound = errors.New("课次不存在")
	// ErrInvalidClass 班级参数不合法。
	ErrInvalidClass = errors.New("班级信息不合法")
	// ErrInvalidSession 课次参数不合法。
	ErrInvalidSession = errors.New("课次信息不合法")
	// ErrClassState 班级状态不允许该操作。
	ErrClassState = errors.New("班级状态不允许该操作")
	// ErrStudentNotActive 只有在读学员可以加入班级。
	ErrStudentNotActive = errors.New("只有状态为在读的学员可以加入班级")
)

// Class 班级。
type Class struct {
	ID           string
	Name         string
	Subject      string
	StartDate    *time.Time
	EndDate      *time.Time
	ScheduleNote string
	Status       string
	CreatedBy    string
	CreatedAt    time.Time
	MemberCount  int
	SessionCount int
}

// ClassMember 班级成员。
type ClassMember struct {
	StudentID   string
	StudentName string
	Grade       string
	GroupCode   string
	JoinedAt    time.Time
	LeftAt      *time.Time
}

// AttendanceRow 一条出勤记录。
type AttendanceRow struct {
	StudentID   string
	StudentName string
	Status      string
}

// ClassSession 一次上课记录。
type ClassSession struct {
	ID          string
	ClassID     string
	SessionDate time.Time
	Content     string
	CreatedBy   string
	CreatedAt   time.Time
	Attendance  []AttendanceRow
}

// ClassInput 班级创建/更新入参。
type ClassInput struct {
	Name         string
	Subject      string
	StartDate    *time.Time
	EndDate      *time.Time
	ScheduleNote string
	Status       string
}

// SessionInput 课次创建入参。
type SessionInput struct {
	SessionDate time.Time
	Content     string
	Attendance  map[string]string
}

// ClassRepo 班课仓储。
type ClassRepo interface {
	ListClasses(ctx context.Context, status string) ([]Class, error)
	GetClass(ctx context.Context, classID string) (*Class, error)
	CreateClass(ctx context.Context, class *Class) error
	UpdateClass(ctx context.Context, class *Class) error
	ListMembers(ctx context.Context, classID string) ([]ClassMember, error)
	AddMembers(ctx context.Context, classID string, studentIDs []string) (int, error)
	RemoveMember(ctx context.Context, classID, studentID string) error
	ListSessions(ctx context.Context, classID string) ([]ClassSession, error)
	CreateSession(ctx context.Context, session *ClassSession) error
	DeleteSession(ctx context.Context, sessionID string) error
}

// ClassUsecase 班课用例。
type ClassUsecase struct {
	repo ClassRepo
}

// NewClassUsecase 构造班课用例。
func NewClassUsecase(repo ClassRepo) *ClassUsecase {
	return &ClassUsecase{repo: repo}
}

// List 班级列表，status 为空表示全部。
func (uc *ClassUsecase) List(ctx context.Context, status string) ([]Class, error) {
	status = strings.TrimSpace(status)
	if status != "" && status != ClassActive && status != ClassArchived {
		return nil, ErrInvalidClass
	}
	return uc.repo.ListClasses(ctx, status)
}

// Get 班级详情。
func (uc *ClassUsecase) Get(ctx context.Context, classID string) (*Class, error) {
	return uc.repo.GetClass(ctx, strings.TrimSpace(classID))
}

// Create 创建班级。
func (uc *ClassUsecase) Create(ctx context.Context, actorID string, in ClassInput) (*Class, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" || len([]rune(name)) > 128 {
		return nil, ErrInvalidClass
	}
	if in.StartDate != nil && in.EndDate != nil && in.EndDate.Before(*in.StartDate) {
		return nil, ErrInvalidClass
	}
	subject := strings.TrimSpace(in.Subject)
	if subject == "" {
		subject = "physics"
	}
	note := strings.TrimSpace(in.ScheduleNote)
	if len([]rune(note)) > 255 {
		return nil, ErrInvalidClass
	}
	now := time.Now().UTC()
	class := &Class{
		ID:           NewID(),
		Name:         name,
		Subject:      subject,
		StartDate:    in.StartDate,
		EndDate:      in.EndDate,
		ScheduleNote: note,
		Status:       ClassActive,
		CreatedBy:    actorID,
		CreatedAt:    now,
	}
	if err := uc.repo.CreateClass(ctx, class); err != nil {
		return nil, err
	}
	return class, nil
}

// Update 更新班级；只有 active 班级可改。
func (uc *ClassUsecase) Update(ctx context.Context, classID string, in ClassInput) (*Class, error) {
	class, err := uc.repo.GetClass(ctx, strings.TrimSpace(classID))
	if err != nil {
		return nil, err
	}
	if class.Status != ClassActive {
		return nil, ErrClassState
	}
	if name := strings.TrimSpace(in.Name); name != "" {
		if len([]rune(name)) > 128 {
			return nil, ErrInvalidClass
		}
		class.Name = name
	}
	if subject := strings.TrimSpace(in.Subject); subject != "" {
		class.Subject = subject
	}
	if in.StartDate != nil {
		class.StartDate = in.StartDate
	}
	if in.EndDate != nil {
		class.EndDate = in.EndDate
	}
	if class.StartDate != nil && class.EndDate != nil && class.EndDate.Before(*class.StartDate) {
		return nil, ErrInvalidClass
	}
	if in.ScheduleNote != "" {
		note := strings.TrimSpace(in.ScheduleNote)
		if len([]rune(note)) > 255 {
			return nil, ErrInvalidClass
		}
		class.ScheduleNote = note
	}
	if err := uc.repo.UpdateClass(ctx, class); err != nil {
		return nil, err
	}
	return class, nil
}

// Archive 归档班级（不删除，保留历史出勤）。
func (uc *ClassUsecase) Archive(ctx context.Context, classID string) error {
	class, err := uc.repo.GetClass(ctx, strings.TrimSpace(classID))
	if err != nil {
		return err
	}
	if class.Status == ClassArchived {
		return nil
	}
	class.Status = ClassArchived
	return uc.repo.UpdateClass(ctx, class)
}

// Members 班级成员。
func (uc *ClassUsecase) Members(ctx context.Context, classID string) ([]ClassMember, error) {
	return uc.repo.ListMembers(ctx, strings.TrimSpace(classID))
}

// AddMembers 批量加入学员；只允许在读学员，重复加入忽略。
func (uc *ClassUsecase) AddMembers(ctx context.Context, classID string, studentIDs []string) (int, error) {
	class, err := uc.repo.GetClass(ctx, strings.TrimSpace(classID))
	if err != nil {
		return 0, err
	}
	if class.Status != ClassActive {
		return 0, ErrClassState
	}
	ids := normalizeIDs(studentIDs)
	if len(ids) == 0 {
		return 0, ErrInvalidClass
	}
	return uc.repo.AddMembers(ctx, class.ID, ids)
}

// RemoveMember 移除成员（标记退出时间）。
func (uc *ClassUsecase) RemoveMember(ctx context.Context, classID, studentID string) error {
	return uc.repo.RemoveMember(ctx, strings.TrimSpace(classID), strings.TrimSpace(studentID))
}

// Sessions 上课记录。
func (uc *ClassUsecase) Sessions(ctx context.Context, classID string) ([]ClassSession, error) {
	return uc.repo.ListSessions(ctx, strings.TrimSpace(classID))
}

// CreateSession 记录一次上课：日期、要点、逐人出勤。
func (uc *ClassUsecase) CreateSession(ctx context.Context, actorID, classID string, in SessionInput) (*ClassSession, error) {
	class, err := uc.repo.GetClass(ctx, strings.TrimSpace(classID))
	if err != nil {
		return nil, err
	}
	if class.Status != ClassActive {
		return nil, ErrClassState
	}
	if in.SessionDate.IsZero() {
		return nil, ErrInvalidSession
	}
	content := strings.TrimSpace(in.Content)
	if len([]rune(content)) > 1000 {
		return nil, ErrInvalidSession
	}
	for _, status := range in.Attendance {
		if !validAttendance(status) {
			return nil, ErrInvalidSession
		}
	}
	rows := make([]AttendanceRow, 0, len(in.Attendance))
	for studentID, status := range in.Attendance {
		rows = append(rows, AttendanceRow{StudentID: studentID, Status: status})
	}
	session := &ClassSession{
		ID:          NewID(),
		ClassID:     class.ID,
		SessionDate: in.SessionDate,
		Content:     content,
		CreatedBy:   actorID,
		CreatedAt:   time.Now().UTC(),
		Attendance:  rows,
	}
	if err := uc.repo.CreateSession(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

// DeleteSession 删除课次。
func (uc *ClassUsecase) DeleteSession(ctx context.Context, sessionID string) error {
	return uc.repo.DeleteSession(ctx, strings.TrimSpace(sessionID))
}

func validAttendance(status string) bool {
	for _, item := range AttendanceStatuses {
		if status == item {
			return true
		}
	}
	return false
}

// AttendanceLabel 出勤状态中文label。
func AttendanceLabel(status string) string {
	switch status {
	case AttendancePresent:
		return "出勤"
	case AttendanceAbsent:
		return "缺课"
	case AttendanceLeave:
		return "请假"
	default:
		return status
	}
}
