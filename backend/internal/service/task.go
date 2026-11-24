package service

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	khttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/bishop9910/hm-lesson/backend/internal/biz"
)

type taskDTO struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	Description    string  `json:"description"`
	Type           string  `json:"type"`
	Status         string  `json:"status"`
	DueAt          *string `json:"due_at,omitempty"`
	PublishedAt    *string `json:"published_at,omitempty"`
	CreatedAt      string  `json:"created_at"`
	AssignedCount  int     `json:"assigned_count"`
	SubmittedCount int     `json:"submitted_count"`
	ReviewedCount  int     `json:"reviewed_count"`
	PendingReview  int     `json:"pending_review"`
	OverdueCount   int     `json:"overdue_count"`
	SubmitRate     float64 `json:"submit_rate"`
}

type submissionDTO struct {
	ID          string   `json:"id"`
	Version     int      `json:"version"`
	Text        string   `json:"text"`
	FileIDs     []string `json:"file_ids"`
	SubmittedAt string   `json:"submitted_at"`
	IsLate      bool     `json:"is_late"`
}

type assignmentDTO struct {
	ID           string          `json:"id"`
	TaskID       string          `json:"task_id"`
	TaskTitle    string          `json:"task_title"`
	TaskType     string          `json:"task_type"`
	TaskDesc     string          `json:"task_description,omitempty"`
	StudentID    string          `json:"student_id,omitempty"`
	StudentName  string          `json:"student_name,omitempty"`
	Status       string          `json:"status"`
	DueAt        *string         `json:"due_at,omitempty"`
	SubmittedAt  *string         `json:"submitted_at,omitempty"`
	ReviewedAt   *string         `json:"reviewed_at,omitempty"`
	Score        *float64        `json:"score,omitempty"`
	Comment      string          `json:"comment"`
	ReturnReason string          `json:"return_reason"`
	Submission   *submissionDTO  `json:"submission,omitempty"`
	History      []submissionDTO `json:"history,omitempty"`
}

func fmtTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	v := t.UTC().Format("2006-01-02T15:04:05Z")
	return &v
}

func toTaskDTO(t biz.Task) taskDTO {
	rate := 0.0
	if t.AssignedCount > 0 {
		rate = float64(t.SubmittedCount) / float64(t.AssignedCount)
	}
	return taskDTO{
		ID: t.ID, Title: t.Title, Description: t.Description, Type: t.Type, Status: t.Status,
		DueAt: fmtTime(t.DueAt), PublishedAt: fmtTime(t.PublishedAt),
		CreatedAt:     t.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		AssignedCount: t.AssignedCount, SubmittedCount: t.SubmittedCount, ReviewedCount: t.ReviewedCount,
		PendingReview: t.PendingReview, OverdueCount: t.OverdueCount, SubmitRate: rate,
	}
}

func toSubmissionDTO(s biz.Submission) submissionDTO {
	ids := s.FileIDs
	if ids == nil {
		ids = []string{}
	}
	return submissionDTO{ID: s.ID, Version: s.Version, Text: s.Text, FileIDs: ids,
		SubmittedAt: s.SubmittedAt.UTC().Format("2006-01-02T15:04:05Z"), IsLate: s.IsLate}
}

func toAssignmentDTO(a biz.TaskAssignment) assignmentDTO {
	d := assignmentDTO{
		ID: a.ID, TaskID: a.TaskID, TaskTitle: a.TaskTitle, TaskType: a.TaskType, TaskDesc: a.TaskDesc,
		StudentID: a.StudentID, StudentName: a.StudentName, Status: a.Status,
		DueAt: fmtTime(a.DueAt), SubmittedAt: fmtTime(a.SubmittedAt), ReviewedAt: fmtTime(a.ReviewedAt),
		Score: a.Score, Comment: a.Comment, ReturnReason: a.ReturnReason,
	}
	if a.Submission != nil {
		s := toSubmissionDTO(*a.Submission)
		d.Submission = &s
	}
	if len(a.History) > 0 {
		d.History = make([]submissionDTO, 0, len(a.History))
		for _, s := range a.History {
			d.History = append(d.History, toSubmissionDTO(s))
		}
	}
	return d
}

func (s *Server) ListTasks(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	page, _ := strconv.Atoi(ctx.Query().Get("page"))
	items, total, err := s.tasks.List(ctx, ctx.Query().Get("status"), ctx.Query().Get("q"), page, 20)
	if err != nil {
		return mapTaskError(ctx, err)
	}
	out := make([]taskDTO, 0, len(items))
	for _, t := range items {
		out = append(out, toTaskDTO(t))
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": out, "total": total})
}

type createTaskRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Type        string `json:"type"`
	DueAt       string `json:"due_at"`
}

func (s *Server) CreateTask(ctx khttp.Context) error {
	staff, err := s.requireStaff(ctx, false)
	if err != nil {
		return err
	}
	var req createTaskRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求体格式错误")
	}
	due, perr := time.Parse(time.RFC3339, strings.TrimSpace(req.DueAt))
	if perr != nil {
		return fail(ctx, http.StatusBadRequest, "INVALID_DUE_AT", "截止时间格式错误")
	}
	t, err := s.tasks.Create(ctx, staff.ID, req.Title, req.Description, req.Type, &due)
	if err != nil {
		return mapTaskError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"task": toTaskDTO(*t)})
}

type publishTaskRequest struct {
	StudentIDs []string `json:"student_ids"`
}

func (s *Server) PublishTask(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	var req publishTaskRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求体格式错误")
	}
	n, err := s.tasks.Publish(ctx, ctx.Vars().Get("id"), req.StudentIDs)
	if err != nil {
		return mapTaskError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"assigned": n})
}

func (s *Server) GetTask(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	d, err := s.tasks.Detail(ctx, ctx.Vars().Get("id"))
	if err != nil {
		return mapTaskError(ctx, err)
	}
	as := make([]assignmentDTO, 0, len(d.Assignments))
	for _, a := range d.Assignments {
		as = append(as, toAssignmentDTO(a))
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"task": toTaskDTO(d.Task), "assignments": as, "stats": d.Stats,
	})
}

func (s *Server) CloseTask(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	if err := s.tasks.Close(ctx, ctx.Vars().Get("id")); err != nil {
		return mapTaskError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) ReviewQueue(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	items, err := s.tasks.ReviewQueue(ctx)
	if err != nil {
		return mapTaskError(ctx, err)
	}
	out := make([]assignmentDTO, 0, len(items))
	for _, a := range items {
		out = append(out, toAssignmentDTO(a))
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": out})
}

type reviewRequest struct {
	Score        *float64 `json:"score"`
	Comment      string   `json:"comment"`
	ReturnReason string   `json:"return_reason"`
}

func (s *Server) ReviewAssignment(ctx khttp.Context) error {
	staff, err := s.requireStaff(ctx, false)
	if err != nil {
		return err
	}
	var req reviewRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求体格式错误")
	}
	if err := s.tasks.Review(ctx, ctx.Vars().Get("id"), staff.ID, req.Score, req.Comment, req.ReturnReason); err != nil {
		return mapTaskError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) PortalTasks(ctx khttp.Context) error {
	gid, err := s.requireGuardian(ctx)
	if err != nil {
		return err
	}
	items, err := s.tasks.PortalList(ctx, gid)
	if err != nil {
		return mapTaskError(ctx, err)
	}
	out := make([]assignmentDTO, 0, len(items))
	for _, a := range items {
		out = append(out, toAssignmentDTO(a))
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": out})
}

func (s *Server) PortalTask(ctx khttp.Context) error {
	gid, err := s.requireGuardian(ctx)
	if err != nil {
		return err
	}
	a, err := s.tasks.PortalDetail(ctx, gid, ctx.Vars().Get("id"))
	if err != nil {
		return mapTaskError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"assignment": toAssignmentDTO(*a)})
}

func (s *Server) PortalArchive(ctx khttp.Context) error {
	gid, err := s.requireGuardian(ctx)
	if err != nil {
		return err
	}
	items, err := s.tasks.PortalArchive(ctx, gid)
	if err != nil {
		return mapTaskError(ctx, err)
	}
	out := make([]assignmentDTO, 0, len(items))
	for _, a := range items {
		out = append(out, toAssignmentDTO(a))
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": out})
}

type submitRequest struct {
	Text    string   `json:"text"`
	FileIDs []string `json:"file_ids"`
}

func (s *Server) PortalSubmit(ctx khttp.Context) error {
	gid, err := s.requireGuardian(ctx)
	if err != nil {
		return err
	}
	var req submitRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求体格式错误")
	}
	sub, err := s.tasks.Submit(ctx, gid, ctx.Vars().Get("id"), req.Text, req.FileIDs)
	if err != nil {
		return mapTaskError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"submission": toSubmissionDTO(*sub)})
}

type templateRequest struct {
	Title          string `json:"title"`
	Description    string `json:"description"`
	Type           string `json:"type"`
	DueOffsetHours int    `json:"due_offset_hours"`
}

// ListTemplates 返回任务模板。
func (s *Server) ListTemplates(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	items, err := s.tasks.Templates(ctx)
	if err != nil {
		return mapTaskError(ctx, err)
	}
	if items == nil {
		items = []biz.TaskTemplate{}
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": items})
}

// CreateTemplate 新建模板，仅 owner。
func (s *Server) CreateTemplate(ctx khttp.Context) error {
	actor, err := s.requireStaff(ctx, true)
	if err != nil {
		return err
	}
	var req templateRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求体格式错误")
	}
	t, err := s.tasks.CreateTemplate(ctx, actor.ID, req.Title, req.Description, req.Type, req.DueOffsetHours)
	if err != nil {
		return mapTaskError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"template": t})
}

// UpdateTemplate 修改模板，仅 owner。
func (s *Server) UpdateTemplate(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, true); err != nil {
		return err
	}
	var req templateRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求体格式错误")
	}
	t, err := s.tasks.UpdateTemplate(ctx, ctx.Vars().Get("id"), req.Title, req.Description, req.Type, req.DueOffsetHours)
	if err != nil {
		return mapTaskError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"template": t})
}

// DeleteTemplate 删除模板，仅 owner。
func (s *Server) DeleteTemplate(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, true); err != nil {
		return err
	}
	if err := s.tasks.DeleteTemplate(ctx, ctx.Vars().Get("id")); err != nil {
		return mapTaskError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func mapTaskError(ctx khttp.Context, err error) error {
	switch {
	case errors.Is(err, biz.ErrTaskNotFound):
		return fail(ctx, http.StatusNotFound, "TASK_NOT_FOUND", "任务不存在")
	case errors.Is(err, biz.ErrAssignmentNotFound):
		return fail(ctx, http.StatusNotFound, "ASSIGNMENT_NOT_FOUND", "作业不存在或不属于当前账号")
	case errors.Is(err, biz.ErrTaskState):
		return fail(ctx, http.StatusConflict, "TASK_STATE", "当前状态不允许该操作")
	case errors.Is(err, biz.ErrAlreadyReviewed):
		return fail(ctx, http.StatusConflict, "TASK_ALREADY_REVIEWED", "该作业已批阅")
	case errors.Is(err, biz.ErrNothingToSubmit):
		return fail(ctx, http.StatusBadRequest, "NOTHING_TO_SUBMIT", "请填写内容或上传图片")
	case errors.Is(err, biz.ErrInvalidTask):
		return fail(ctx, http.StatusBadRequest, "INVALID_TASK", "任务参数不合法")
	default:
		return fail(ctx, http.StatusInternalServerError, "INTERNAL", "任务操作失败")
	}
}
