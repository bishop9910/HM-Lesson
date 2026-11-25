package biz

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"
	"time"
)

var (
	ErrStudentNotFound     = errors.New("student not found")
	ErrGuardianNotFound    = errors.New("guardian not found")
	ErrInvalidStudent      = errors.New("invalid student")
	ErrInvalidFollowUp     = errors.New("invalid follow up")
	ErrInvalidCode         = errors.New("invalid binding code")
	ErrCodeExpired         = errors.New("binding code expired")
	ErrCodeUsed            = errors.New("binding code used")
	ErrGuardianDisabled    = errors.New("guardian disabled")
	ErrBindPending         = errors.New("guardian pending")
	ErrWeakPassword        = errors.New("weak password")
	ErrWrongPassword       = errors.New("wrong password")
	ErrPasswordNotSet      = errors.New("password not set")
	ErrGuardianHasStudents = errors.New("guardian still bound to students")
	ErrStudentHasLedger    = errors.New("student has lesson ledger")
	// ErrInvalidBatch 批量操作的 id 列表为空或超过上限。
	ErrInvalidBatch = errors.New("invalid batch ids")
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
	// PasswordHash 只用于登录校验，不出现在任何 DTO 里。
	PasswordHash string
	LastLoginAt  *time.Time
}

// HasPassword 表示家长是否已设置密码（未设置时只能靠绑定码首次进入）。
func (g *GuardianRequest) HasPassword() bool {
	return g != nil && strings.TrimSpace(g.PasswordHash) != ""
}

// GuardianAccount 是教师端家长账号管理列表的一行：一个账号可能对应多个学员。
type GuardianAccount struct {
	ID           string
	Phone        string
	Name         string
	Status       string
	HasPassword  bool
	LastLoginAt  *time.Time
	CreatedAt    time.Time
	StudentIDs   []string
	StudentNames []string
}

// StudentCount 是当前仍处于在读（未归档）状态的学员数。
func (g *GuardianAccount) StudentCount() int {
	return len(g.StudentIDs)
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
	// M8 门户五卡片：待确认回执、待上预约、可见资料。
	PendingReceipts    int
	UpcomingBookings   int
	NextBookingAt      *time.Time
	MaterialsCount     int
	LatestMaterialName string
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

// StudentDeleteStats 汇总删除学员会连带清理的数据，供前端二次确认展示。
type StudentDeleteStats struct {
	TaskAssignments int
	Bookings        int
	LessonLedger    int
	Guardians       int
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
	// StudentDeleteStats 统计删除学员会连带清理的数据量。
	StudentDeleteStats(ctx context.Context, studentID string) (StudentDeleteStats, error)
	// DeleteStudent 物理删除学员档案及其任务/预约/课时/绑定数据。
	DeleteStudent(ctx context.Context, studentID string) error
	AddFollowUp(ctx context.Context, f *FollowUp) (*FollowUp, error)
	ListFollowUps(ctx context.Context, studentID string, limit int) ([]FollowUp, error)
	ListPendingGuardians(ctx context.Context) ([]GuardianRequest, error)
	ApproveGuardian(ctx context.Context, guardianID, studentID, actorID, passwordHash string) error
	RejectGuardian(ctx context.Context, guardianID, reason, actorID string) error
	// ListGuardians 返回全部家长账号（含待审批），按状态与创建时间排序，供 owner 清理。
	ListGuardians(ctx context.Context, status string) ([]GuardianAccount, error)
	// DeleteGuardian 物理删除家长账号及其通知/订阅额度；仍绑定在读学员时拒绝。
	DeleteGuardian(ctx context.Context, guardianID string) error
	SetGuardianPassword(ctx context.Context, guardianID, passwordHash string) error
	// GuardianPasswordHash 返回当前哈希（空串表示未设置），用于改密与首次登录设密。
	GuardianPasswordHash(ctx context.Context, guardianID string) (string, error)
	TouchGuardianLogin(ctx context.Context, guardianID string) error
	CreateBindingCode(ctx context.Context, studentID, actorID string, ttl time.Duration) (*BindingCode, error)
	RedeemBindingCode(ctx context.Context, phone, code string) (*GuardianRequest, []Student, error)
	GuardianByPhone(ctx context.Context, phone string) (*GuardianRequest, error)
	// BindGuardianOpenID 记录网关注入的微信 openid，用于后续订阅消息投递。
	BindGuardianOpenID(ctx context.Context, guardianID, openid string) error
	PortalHome(ctx context.Context, guardianID string) (*PortalHome, error)
	PortalStudent(ctx context.Context, guardianID, studentID string) (*PortalStudent, error)
	UnreadNotifications(ctx context.Context, guardianID string, limit int) ([]Notification, error)
	MarkNotificationRead(ctx context.Context, guardianID, id string) error
	MarkAllNotificationsRead(ctx context.Context, guardianID string) error
}

type CRMUsecase struct {
	repo   CRMRepo
	hasher PasswordHasher
}

func NewCRMUsecase(repo CRMRepo, hasher PasswordHasher) *CRMUsecase {
	if hasher == nil {
		hasher = missingHasher{}
	}
	return &CRMUsecase{repo: repo, hasher: hasher}
}

// missingHasher 在装配漏传 hasher 时直接报错，避免把明文当哈希写进库。
type missingHasher struct{}

func (missingHasher) Hash(string) (string, error) {
	return "", errors.New("密码哈希器未装配")
}
func (missingHasher) Verify(string, string) bool { return false }

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

// DeleteStudent 物理删除学员档案（仅 owner）。
//
// 删除会解除该学员与家长的全部绑定：家长随即进入“未绑定”状态（若已无在读学员则置 disabled），
// 之后可被直接删除或重新绑定新学员。学员若有课时流水，必须由调用方显式 force 才会连同流水一起删除，
// 避免误删账目；前端因此需要二次确认。
func (uc *CRMUsecase) DeleteStudent(ctx context.Context, actor *Staff, studentID string, force bool) (StudentDeleteStats, error) {
	if actor == nil || actor.Role != RoleOwner {
		return StudentDeleteStats{}, ErrForbidden
	}
	stats, err := uc.repo.StudentDeleteStats(ctx, studentID)
	if err != nil {
		return StudentDeleteStats{}, err
	}
	if stats.LessonLedger > 0 && !force {
		return stats, ErrStudentHasLedger
	}
	if err := uc.repo.DeleteStudent(ctx, studentID); err != nil {
		return stats, err
	}
	return stats, nil
}

// MaxBatchDelete 一次批量删除的条数上限（避免单请求超时）。
const MaxBatchDelete = 50

// BatchDeleteResult 批量删除的单条结果。
type BatchDeleteResult struct {
	ID     string `json:"id"`
	OK     bool   `json:"ok"`
	Reason string `json:"reason,omitempty"`
	// Deleted 只在学员删除时返回，用于前端汇总提示。
	Deleted *StudentDeleteStats `json:"deleted,omitempty"`
}

// DeleteStudents 批量删除学员：逐条执行，单条失败不影响其余（结果逐条返回）。
func (uc *CRMUsecase) DeleteStudents(ctx context.Context, actor *Staff, ids []string, force bool) ([]BatchDeleteResult, error) {
	if actor == nil || actor.Role != RoleOwner {
		return nil, ErrForbidden
	}
	ids = normalizeIDs(ids)
	if len(ids) == 0 || len(ids) > MaxBatchDelete {
		return nil, ErrInvalidBatch
	}
	results := make([]BatchDeleteResult, 0, len(ids))
	for _, id := range ids {
		stats, err := uc.DeleteStudent(ctx, actor, id, force)
		item := BatchDeleteResult{ID: id, OK: err == nil}
		if err != nil {
			item.Reason = batchReason(err)
			if errors.Is(err, ErrStudentHasLedger) {
				// 让前端知道是哪几条挡住了，便于二次确认后带 force 重试。
				copied := stats
				item.Deleted = &copied
			}
		} else {
			copied := stats
			item.Deleted = &copied
		}
		results = append(results, item)
	}
	return results, nil
}

// DeleteGuardians 批量删除家长账号：逐条执行，有在读学员的那条会返回 GUARDIAN_HAS_STUDENTS。
func (uc *CRMUsecase) DeleteGuardians(ctx context.Context, actor *Staff, ids []string) ([]BatchDeleteResult, error) {
	if actor == nil || actor.Role != RoleOwner {
		return nil, ErrForbidden
	}
	ids = normalizeIDs(ids)
	if len(ids) == 0 || len(ids) > MaxBatchDelete {
		return nil, ErrInvalidBatch
	}
	results := make([]BatchDeleteResult, 0, len(ids))
	for _, id := range ids {
		err := uc.DeleteGuardian(ctx, actor, id)
		item := BatchDeleteResult{ID: id, OK: err == nil}
		if err != nil {
			item.Reason = batchReason(err)
		}
		results = append(results, item)
	}
	return results, nil
}

func normalizeIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func batchReason(err error) string {
	switch {
	case errors.Is(err, ErrStudentHasLedger):
		return "STUDENT_HAS_LESSON_LEDGER"
	case errors.Is(err, ErrGuardianHasStudents):
		return "GUARDIAN_HAS_STUDENTS"
	case errors.Is(err, ErrStudentNotFound):
		return "STUDENT_NOT_FOUND"
	case errors.Is(err, ErrGuardianNotFound):
		return "GUARDIAN_NOT_FOUND"
	case errors.Is(err, ErrForbidden):
		return "FORBIDDEN"
	default:
		return "INTERNAL"
	}
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

// ApproveGuardian 审批通过并写入初始密码；password 为空时由 service 生成临时密码。
func (uc *CRMUsecase) ApproveGuardian(ctx context.Context, actor *Staff, guardianID, studentID, password string) error {
	password = strings.TrimSpace(password)
	if !validGuardianPassword(password) {
		return ErrWeakPassword
	}
	hash, err := uc.hasher.Hash(password)
	if err != nil {
		return err
	}
	return uc.repo.ApproveGuardian(ctx, guardianID, studentID, actor.ID, hash)
}

// Guardians 列出全部家长账号，供 owner 清理长期未绑定/无学员的账号。
func (uc *CRMUsecase) Guardians(ctx context.Context, status string) ([]GuardianAccount, error) {
	status = strings.TrimSpace(status)
	switch status {
	case "", "pending", "active", "rejected", "disabled":
	default:
		return nil, ErrGuardianNotFound
	}
	return uc.repo.ListGuardians(ctx, status)
}

// DeleteGuardian 删除家长账号；仍绑定在读学员时拒绝，先归档学员再删。
func (uc *CRMUsecase) DeleteGuardian(ctx context.Context, actor *Staff, guardianID string) error {
	if actor == nil || actor.Role != RoleOwner {
		return ErrForbidden
	}
	if strings.TrimSpace(guardianID) == "" {
		return ErrGuardianNotFound
	}
	return uc.repo.DeleteGuardian(ctx, guardianID)
}

// ResetGuardianPassword 由 owner 重置家长密码（家长忘记密码时的兜底）。
func (uc *CRMUsecase) ResetGuardianPassword(ctx context.Context, actor *Staff, guardianID, password string) error {
	if actor == nil || actor.Role != RoleOwner {
		return ErrForbidden
	}
	password = strings.TrimSpace(password)
	if !validGuardianPassword(password) {
		return ErrWeakPassword
	}
	hash, err := uc.hasher.Hash(password)
	if err != nil {
		return err
	}
	return uc.repo.SetGuardianPassword(ctx, guardianID, hash)
}

// ChangeGuardianPassword 家长自助改密；old 为空且当前未设密码时视为首次设置。
func (uc *CRMUsecase) ChangeGuardianPassword(ctx context.Context, guardianID, oldPassword, newPassword string) error {
	newPassword = strings.TrimSpace(newPassword)
	if !validGuardianPassword(newPassword) {
		return ErrWeakPassword
	}
	current, err := uc.repo.GuardianPasswordHash(ctx, guardianID)
	if err != nil {
		return err
	}
	if current != "" {
		if !uc.hasher.Verify(current, strings.TrimSpace(oldPassword)) {
			return ErrWrongPassword
		}
	}
	hash, err := uc.hasher.Hash(newPassword)
	if err != nil {
		return err
	}
	return uc.repo.SetGuardianPassword(ctx, guardianID, hash)
}

// VerifyGuardianPassword 校验家长登录密码；未设置密码时返回 ErrPasswordNotSet。
func (uc *CRMUsecase) VerifyGuardianPassword(ctx context.Context, guardian *GuardianRequest, plain string) error {
	if guardian == nil {
		return ErrGuardianNotFound
	}
	if !guardian.HasPassword() {
		return ErrPasswordNotSet
	}
	if !uc.hasher.Verify(guardian.PasswordHash, strings.TrimSpace(plain)) {
		return ErrWrongPassword
	}
	return uc.repo.TouchGuardianLogin(ctx, guardian.ID)
}

// SetInitialGuardianPassword 首次设密（绑定码登录时使用），当前已有密码则拒绝覆盖。
func (uc *CRMUsecase) SetInitialGuardianPassword(ctx context.Context, guardianID, password string) error {
	password = strings.TrimSpace(password)
	if !validGuardianPassword(password) {
		return ErrWeakPassword
	}
	current, err := uc.repo.GuardianPasswordHash(ctx, guardianID)
	if err != nil {
		return err
	}
	if current != "" {
		return ErrWrongPassword
	}
	hash, err := uc.hasher.Hash(password)
	if err != nil {
		return err
	}
	return uc.repo.SetGuardianPassword(ctx, guardianID, hash)
}

// validGuardianPassword 家长密码规则：6~32 个字符，去掉首尾空格后不能为空。
// staff 用 8 位起（见 auth.go），家长是手机端输入，门槛取 6。
func validGuardianPassword(p string) bool {
	n := len([]rune(p))
	return n >= 6 && n <= 32
}

// NewTemporaryPassword 生成一次性初始密码（去掉易混淆的 0/O/1/l/I）。
func NewTemporaryPassword() string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789"
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		// rand.Read 只在系统熵源不可用时失败；退化到时间派生值也要保证可登录。
		now := time.Now().UnixNano()
		for i := range buf {
			buf[i] = byte(now >> (uint(i) * 5))
		}
	}
	out := make([]byte, len(buf))
	for i, b := range buf {
		out[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(out)
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

// BindGuardianOpenID 采集网关注入的微信 openid；空值直接跳过。
// 失败不影响登录：订阅消息是增强通道，站内通知仍是保底。
func (uc *CRMUsecase) BindGuardianOpenID(ctx context.Context, guardianID, openid string) error {
	openid = strings.TrimSpace(openid)
	if openid == "" || strings.TrimSpace(guardianID) == "" {
		return nil
	}
	return uc.repo.BindGuardianOpenID(ctx, guardianID, openid)
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
