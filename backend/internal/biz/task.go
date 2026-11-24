package biz

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrTaskNotFound       = errors.New("task not found")
	ErrAssignmentNotFound = errors.New("assignment not found")
	ErrInvalidTask        = errors.New("invalid task")
	ErrTaskState          = errors.New("task state conflict")
	ErrAlreadyReviewed    = errors.New("task already reviewed")
	ErrNothingToSubmit    = errors.New("nothing to submit")
)

const (
	TaskDraft     = "draft"
	TaskPublished = "published"
	TaskClosed    = "closed"

	AssignmentPending   = "pending"
	AssignmentSubmitted = "submitted"
	AssignmentReviewed  = "reviewed"
	AssignmentReturned  = "returned"
)

// Task 是任务主表。
type Task struct {
	ID             string
	Title          string
	Description    string
	Type           string
	Status         string
	DueAt          *time.Time
	PublishedAt    *time.Time
	CreatedAt      time.Time
	CreatedBy      string
	AssignedCount  int
	SubmittedCount int
	ReviewedCount  int
	PendingReview  int
	OverdueCount   int
}

// Submission 是一次作业提交。
type Submission struct {
	ID          string
	Version     int
	Text        string
	FileIDs     []string
	SubmittedAt time.Time
	IsLate      bool
}

// TaskAssignment 是一个学员的一份任务。
type TaskAssignment struct {
	ID            string
	TaskID        string
	TaskTitle     string
	TaskType      string
	TaskDesc      string
	StudentID     string
	StudentName   string
	Status        string
	DueAt         *time.Time
	SubmittedAt   *time.Time
	ReviewedAt    *time.Time
	Score         *float64
	Comment       string
	ReturnReason  string
	ReviewFileIDs []string
	CreatedAt     time.Time
	Submission    *Submission
	History       []Submission
}

// TaskStats 与看板口径一致。
type TaskStats struct {
	Assigned      int     `json:"assigned"`
	Submitted     int     `json:"submitted"`
	Reviewed      int     `json:"reviewed"`
	Returned      int     `json:"returned"`
	Pending       int     `json:"pending"`
	PendingReview int     `json:"pending_review"`
	Overdue       int     `json:"overdue"`
	Rate          float64 `json:"rate"`
}

// TaskDetail 是教师任务详情。
type TaskDetail struct {
	Task        Task             `json:"task"`
	Assignments []TaskAssignment `json:"assignments"`
	Stats       TaskStats        `json:"stats"`
}

// TaskTemplate 保存可复用的任务标题/说明/类型/截止偏移。
type TaskTemplate struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	Type           string    `json:"type"`
	DueOffsetHours int       `json:"due_offset_hours"`
	CreatedBy      string    `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
}

// TaskRepo 是任务持久化接口。
type TaskRepo interface {
	ListTasks(ctx context.Context, status, q string, limit, offset int) ([]Task, int, error)
	CreateTask(ctx context.Context, t *Task) error
	GetTask(ctx context.Context, id string) (*Task, error)
	SetTaskStatus(ctx context.Context, id, from, to string) error
	DeleteTask(ctx context.Context, id string) error
	PublishTask(ctx context.Context, taskID string, studentIDs []string) (int, error)
	ListAssignments(ctx context.Context, taskID string) ([]TaskAssignment, error)
	ReviewQueue(ctx context.Context, limit int) ([]TaskAssignment, error)
	ReviewAssignment(ctx context.Context, assignmentID, reviewerID string, score *float64, comment, returnReason string, reviewFileIDs []string) error

	PortalTasks(ctx context.Context, guardianID string) ([]TaskAssignment, error)
	PortalTask(ctx context.Context, guardianID, assignmentID string) (*TaskAssignment, error)
	PortalArchive(ctx context.Context, guardianID string) ([]TaskAssignment, error)
	Submit(ctx context.Context, guardianID, assignmentID, text string, fileIDs []string) (*Submission, error)

	ListTemplates(ctx context.Context) ([]TaskTemplate, error)
	CreateTemplate(ctx context.Context, t *TaskTemplate) error
	UpdateTemplate(ctx context.Context, t *TaskTemplate) error
	DeleteTemplate(ctx context.Context, id string) error
}

// TaskUsecase 编排任务业务。
type TaskUsecase struct {
	repo TaskRepo
}

func NewTaskUsecase(repo TaskRepo) *TaskUsecase {
	return &TaskUsecase{repo: repo}
}

func (u *TaskUsecase) List(ctx context.Context, status, q string, page, pageSize int) ([]Task, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}
	return u.repo.ListTasks(ctx, status, strings.TrimSpace(q), pageSize, (page-1)*pageSize)
}

// Create 新建草稿任务。
func (u *TaskUsecase) Create(ctx context.Context, authorID, title, description, typ string, dueAt *time.Time) (*Task, error) {
	title = strings.TrimSpace(title)
	if title == "" || len([]rune(title)) > 128 {
		return nil, ErrInvalidTask
	}
	if len([]rune(description)) > 2000 {
		return nil, ErrInvalidTask
	}
	switch typ {
	case "homework", "correction", "trial", "checkin":
	default:
		return nil, ErrInvalidTask
	}
	if dueAt == nil {
		return nil, ErrInvalidTask
	}
	t := &Task{
		ID: NewID(), Title: title, Description: description, Type: typ,
		Status: TaskDraft, DueAt: dueAt, CreatedAt: time.Now().UTC(), CreatedBy: authorID,
	}
	if err := u.repo.CreateTask(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// Publish 按对象生成一人一条 assignment 并置为已发布。
func (u *TaskUsecase) Publish(ctx context.Context, id string, studentIDs []string) (int, error) {
	t, err := u.repo.GetTask(ctx, id)
	if err != nil {
		return 0, err
	}
	if t.Status != TaskDraft {
		return 0, ErrTaskState
	}
	if t.DueAt == nil || !t.DueAt.After(time.Now()) {
		return 0, ErrInvalidTask
	}
	seen := map[string]bool{}
	clean := make([]string, 0, len(studentIDs))
	for _, sid := range studentIDs {
		sid = strings.TrimSpace(sid)
		if sid != "" && !seen[sid] {
			seen[sid] = true
			clean = append(clean, sid)
		}
	}
	if len(clean) == 0 {
		return 0, ErrInvalidTask
	}
	if len(clean) > 500 {
		return 0, ErrInvalidTask
	}
	return u.repo.PublishTask(ctx, id, clean)
}

func (u *TaskUsecase) Close(ctx context.Context, id string) error {
	return u.repo.SetTaskStatus(ctx, id, TaskPublished, TaskClosed)
}

// Delete 允许删除草稿与已结束任务；已发布任务必须先结束。
// 已结束任务会连同 assignment 与提交历史一起删除，前端必须二次确认。
func (u *TaskUsecase) Delete(ctx context.Context, id string) error {
	t, err := u.repo.GetTask(ctx, id)
	if err != nil {
		return err
	}
	if t.Status != TaskDraft && t.Status != TaskClosed {
		return ErrTaskState
	}
	return u.repo.DeleteTask(ctx, id)
}

func (u *TaskUsecase) Detail(ctx context.Context, id string) (*TaskDetail, error) {
	t, err := u.repo.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	as, err := u.repo.ListAssignments(ctx, id)
	if err != nil {
		return nil, err
	}
	stats := TaskStats{Assigned: len(as)}
	for _, a := range as {
		switch a.Status {
		case AssignmentSubmitted:
			stats.Submitted++
			stats.PendingReview++
		case AssignmentReviewed:
			stats.Submitted++
			stats.Reviewed++
		case AssignmentReturned:
			stats.Submitted++
			stats.Returned++
		case AssignmentPending:
			stats.Pending++
			if a.DueAt != nil && a.DueAt.Before(time.Now()) {
				stats.Overdue++
			}
		}
	}
	if stats.Assigned > 0 {
		stats.Rate = float64(stats.Submitted) / float64(stats.Assigned)
	}
	return &TaskDetail{Task: *t, Assignments: as, Stats: stats}, nil
}

func (u *TaskUsecase) ReviewQueue(ctx context.Context) ([]TaskAssignment, error) {
	return u.repo.ReviewQueue(ctx, 100)
}

// Review 批阅；returnReason 非空即打回；reviewFileIDs 为批改图（最多 9 张）。
func (u *TaskUsecase) Review(ctx context.Context, assignmentID, reviewerID string, score *float64, comment, returnReason string, reviewFileIDs []string) error {
	if score != nil && (*score < 0 || *score > 100) {
		return ErrInvalidTask
	}
	comment = strings.TrimSpace(comment)
	returnReason = strings.TrimSpace(returnReason)
	if len([]rune(comment)) > 2000 || len([]rune(returnReason)) > 1000 || len(reviewFileIDs) > 9 {
		return ErrInvalidTask
	}
	for _, id := range reviewFileIDs {
		if strings.TrimSpace(id) == "" {
			return ErrInvalidTask
		}
	}
	return u.repo.ReviewAssignment(ctx, assignmentID, reviewerID, score, comment, returnReason, reviewFileIDs)
}

func (u *TaskUsecase) PortalList(ctx context.Context, guardianID string) ([]TaskAssignment, error) {
	return u.repo.PortalTasks(ctx, guardianID)
}

func (u *TaskUsecase) PortalDetail(ctx context.Context, guardianID, assignmentID string) (*TaskAssignment, error) {
	return u.repo.PortalTask(ctx, guardianID, assignmentID)
}

func (u *TaskUsecase) PortalArchive(ctx context.Context, guardianID string) ([]TaskAssignment, error) {
	return u.repo.PortalArchive(ctx, guardianID)
}

// Submit 家长提交/重交；文字与图片至少一项。
func (u *TaskUsecase) Submit(ctx context.Context, guardianID, assignmentID, text string, fileIDs []string) (*Submission, error) {
	text = strings.TrimSpace(text)
	if len([]rune(text)) > 5000 || len(fileIDs) > 9 {
		return nil, ErrInvalidTask
	}
	if text == "" && len(fileIDs) == 0 {
		return nil, ErrNothingToSubmit
	}
	for _, id := range fileIDs {
		if strings.TrimSpace(id) == "" {
			return nil, ErrInvalidTask
		}
	}
	return u.repo.Submit(ctx, guardianID, assignmentID, text, fileIDs)
}

// Templates 列出任务模板。
func (u *TaskUsecase) Templates(ctx context.Context) ([]TaskTemplate, error) {
	return u.repo.ListTemplates(ctx)
}

// CreateTemplate 新建模板；截止偏移 1~720 小时。
func (u *TaskUsecase) CreateTemplate(ctx context.Context, authorID, title, description, typ string, offsetHours int) (*TaskTemplate, error) {
	title = strings.TrimSpace(title)
	if title == "" || len([]rune(title)) > 128 || len([]rune(description)) > 2000 {
		return nil, ErrInvalidTask
	}
	switch typ {
	case "homework", "correction", "trial", "checkin":
	default:
		return nil, ErrInvalidTask
	}
	if offsetHours < 1 || offsetHours > 720 {
		return nil, ErrInvalidTask
	}
	t := &TaskTemplate{
		ID: NewID(), Title: title, Description: description, Type: typ,
		DueOffsetHours: offsetHours, CreatedBy: authorID, CreatedAt: time.Now().UTC(),
	}
	if err := u.repo.CreateTemplate(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// UpdateTemplate 更新模板。
func (u *TaskUsecase) UpdateTemplate(ctx context.Context, id, title, description, typ string, offsetHours int) (*TaskTemplate, error) {
	title = strings.TrimSpace(title)
	if title == "" || len([]rune(title)) > 128 || len([]rune(description)) > 2000 {
		return nil, ErrInvalidTask
	}
	switch typ {
	case "homework", "correction", "trial", "checkin":
	default:
		return nil, ErrInvalidTask
	}
	if offsetHours < 1 || offsetHours > 720 {
		return nil, ErrInvalidTask
	}
	t := &TaskTemplate{ID: id, Title: title, Description: description, Type: typ, DueOffsetHours: offsetHours}
	if err := u.repo.UpdateTemplate(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (u *TaskUsecase) DeleteTemplate(ctx context.Context, id string) error {
	return u.repo.DeleteTemplate(ctx, id)
}
