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

type studentDTO struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Grade          string   `json:"grade"`
	School         string   `json:"school"`
	GroupCode      string   `json:"group_code"`
	Status         string   `json:"status"`
	Tags           []string `json:"tags"`
	NextFollowUpAt *string  `json:"next_follow_up_at,omitempty"`
	Note           string   `json:"note"`
	CreatedAt      string   `json:"created_at"`
}

func toStudentDTO(s biz.Student) studentDTO {
	d := studentDTO{
		ID: s.ID, Name: s.Name, Grade: s.Grade, School: s.School,
		GroupCode: s.GroupCode, Status: s.Status, Tags: s.Tags, Note: s.Note,
		CreatedAt: s.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if d.Tags == nil {
		d.Tags = []string{}
	}
	if s.NextFollowUpAt != nil {
		v := s.NextFollowUpAt.UTC().Format("2006-01-02T15:04:05Z")
		d.NextFollowUpAt = &v
	}
	return d
}

type portalStudentDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Grade     string `json:"grade"`
	School    string `json:"school"`
	GroupCode string `json:"group_code"`
	Status    string `json:"status"`
}

// toPortalStudentDTO 只暴露家长可见字段，刻意不含老师备注、标签与回访时间。
func toPortalStudentDTO(s biz.Student) portalStudentDTO {
	return portalStudentDTO{
		ID: s.ID, Name: s.Name, Grade: s.Grade, School: s.School,
		GroupCode: s.GroupCode, Status: s.Status,
	}
}

type followUpDTO struct {
	ID           string  `json:"id"`
	Method       string  `json:"method"`
	Content      string  `json:"content"`
	Concern      string  `json:"concern"`
	NextAction   string  `json:"next_action"`
	NextFollowUp *string `json:"next_follow_up_at,omitempty"`
	CreatedAt    string  `json:"created_at"`
}

func toFollowUpDTO(f biz.FollowUp) followUpDTO {
	d := followUpDTO{
		ID: f.ID, Method: f.Method, Content: f.Content, Concern: f.Concern,
		NextAction: f.NextAction, CreatedAt: f.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if f.NextFollowUp != nil {
		v := f.NextFollowUp.UTC().Format("2006-01-02T15:04:05Z")
		d.NextFollowUp = &v
	}
	return d
}

func parseTime(v string) *time.Time {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return nil
	}
	return &t
}

func (s *Server) ListStudents(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	if s.crm == nil {
		return fail(ctx, http.StatusServiceUnavailable, "CRM_DISABLED", "数据库未配置")
	}
	page, _ := strconv.Atoi(ctx.Query().Get("page"))
	size, _ := strconv.Atoi(ctx.Query().Get("page_size"))
	items, total, err := s.crm.ListStudents(ctx, biz.StudentFilter{
		Group: ctx.Query().Get("group"), Status: ctx.Query().Get("status"),
		Query: ctx.Query().Get("q"), Page: page, PageSize: size,
	})
	if err != nil {
		return mapCRMBizError(ctx, err)
	}
	out := make([]studentDTO, 0, len(items))
	for _, item := range items {
		out = append(out, toStudentDTO(item))
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": out, "total": total})
}

type createStudentRequest struct {
	Name           string   `json:"name"`
	Grade          string   `json:"grade"`
	School         string   `json:"school"`
	GroupCode      string   `json:"group_code"`
	Status         string   `json:"status"`
	Tags           []string `json:"tags"`
	NextFollowUpAt string   `json:"next_follow_up_at"`
	Note           string   `json:"note"`
}

func (s *Server) CreateStudent(ctx khttp.Context) error {
	actor, err := s.requireStaff(ctx, true)
	if err != nil {
		return err
	}
	var req createStudentRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求格式错误")
	}
	created, err := s.crm.CreateStudent(ctx, actor, biz.CreateStudentInput{
		Name: req.Name, Grade: req.Grade, School: req.School, GroupCode: req.GroupCode,
		Status: req.Status, Tags: req.Tags, NextFollowUpAt: parseTime(req.NextFollowUpAt), Note: req.Note,
	})
	if err != nil {
		return mapCRMBizError(ctx, err)
	}
	return ctx.JSON(http.StatusCreated, map[string]any{"student": toStudentDTO(*created)})
}

func (s *Server) GetStudent(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	id := ctx.Vars().Get("id")
	student, err := s.crm.GetStudent(ctx, id)
	if err != nil {
		return mapCRMBizError(ctx, err)
	}
	follows, _ := s.crm.ListFollowUps(ctx, id, 50)
	out := make([]followUpDTO, 0, len(follows))
	for _, f := range follows {
		out = append(out, toFollowUpDTO(f))
	}
	return ctx.JSON(http.StatusOK, map[string]any{"student": toStudentDTO(*student), "follow_ups": out})
}

type updateStudentRequest struct {
	Name           *string   `json:"name"`
	Grade          *string   `json:"grade"`
	School         *string   `json:"school"`
	GroupCode      *string   `json:"group_code"`
	Status         *string   `json:"status"`
	Tags           *[]string `json:"tags"`
	NextFollowUpAt *string   `json:"next_follow_up_at"`
	Note           *string   `json:"note"`
}

func (s *Server) UpdateStudent(ctx khttp.Context) error {
	actor, err := s.requireStaff(ctx, true)
	if err != nil {
		return err
	}
	_ = actor
	id := ctx.Vars().Get("id")
	var req updateStudentRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求格式错误")
	}
	updated, err := s.crm.UpdateStudent(ctx, id, biz.UpdateStudentInput{
		Name: req.Name, Grade: req.Grade, School: req.School, GroupCode: req.GroupCode,
		Status: req.Status, Tags: req.Tags, NextFollowUpAt: parseTime(deref(req.NextFollowUpAt)),
		ClearNextFollowUp: req.NextFollowUpAt != nil && strings.TrimSpace(*req.NextFollowUpAt) == "",
		Note:              req.Note,
	})
	if err != nil {
		return mapCRMBizError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"student": toStudentDTO(*updated)})
}

func deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func (s *Server) ListFollowUps(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	items, err := s.crm.ListFollowUps(ctx, ctx.Vars().Get("id"), 50)
	if err != nil {
		return mapCRMBizError(ctx, err)
	}
	out := make([]followUpDTO, 0, len(items))
	for _, f := range items {
		out = append(out, toFollowUpDTO(f))
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": out})
}

type createFollowUpRequest struct {
	Method       string `json:"method"`
	Content      string `json:"content"`
	Concern      string `json:"concern"`
	NextAction   string `json:"next_action"`
	NextFollowUp string `json:"next_follow_up_at"`
}

func (s *Server) CreateFollowUp(ctx khttp.Context) error {
	actor, err := s.requireStaff(ctx, true)
	if err != nil {
		return err
	}
	var req createFollowUpRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求格式错误")
	}
	f, err := s.crm.AddFollowUp(ctx, actor, ctx.Vars().Get("id"), req.Method, req.Content, req.Concern, req.NextAction, parseTime(req.NextFollowUp))
	if err != nil {
		return mapCRMBizError(ctx, err)
	}
	return ctx.JSON(http.StatusCreated, map[string]any{"follow_up": toFollowUpDTO(*f)})
}

func (s *Server) PendingGuardians(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, true); err != nil {
		return err
	}
	items, err := s.crm.PendingGuardians(ctx)
	if err != nil {
		return mapCRMBizError(ctx, err)
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"id": item.ID, "phone": item.Phone, "name": item.Name,
			"status": item.Status, "created_at": item.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": out})
}

type approveGuardianRequest struct {
	StudentID string `json:"student_id"`
}

func (s *Server) ApproveGuardian(ctx khttp.Context) error {
	actor, err := s.requireStaff(ctx, true)
	if err != nil {
		return err
	}
	var req approveGuardianRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求格式错误")
	}
	if err := s.crm.ApproveGuardian(ctx, actor, ctx.Vars().Get("id"), req.StudentID); err != nil {
		return mapCRMBizError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"status": "active"})
}

type rejectGuardianRequest struct {
	Reason string `json:"reason"`
}

func (s *Server) RejectGuardian(ctx khttp.Context) error {
	actor, err := s.requireStaff(ctx, true)
	if err != nil {
		return err
	}
	var req rejectGuardianRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求格式错误")
	}
	if err := s.crm.RejectGuardian(ctx, actor, ctx.Vars().Get("id"), req.Reason); err != nil {
		return mapCRMBizError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"status": "rejected"})
}

func (s *Server) CreateBindingCode(ctx khttp.Context) error {
	actor, err := s.requireStaff(ctx, true)
	if err != nil {
		return err
	}
	code, err := s.crm.CreateBindingCode(ctx, ctx.Vars().Get("id"), actor.ID)
	if err != nil {
		return mapCRMBizError(ctx, err)
	}
	return ctx.JSON(http.StatusCreated, map[string]any{
		"code": code.Code, "expires_at": code.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z"),
	})
}

type guardianLoginRequest struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

// GuardianLogin 家长登录：已审批的家长可用手机号；新家长用手机号 + 6 位绑定码直接绑定。
func (s *Server) GuardianLogin(ctx khttp.Context) error {
	if s.crm == nil || s.tokens == nil {
		return fail(ctx, http.StatusServiceUnavailable, "AUTH_DISABLED", "数据库未配置")
	}
	var req guardianLoginRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求格式错误")
	}
	var guardian *biz.GuardianRequest
	var students []biz.Student
	var err error
	if strings.TrimSpace(req.Code) != "" {
		guardian, students, err = s.crm.GuardianLoginByCode(ctx, req.Phone, req.Code)
	} else {
		guardian, err = s.crm.GuardianByPhone(ctx, req.Phone)
		if err == nil && guardian.Status == "pending" {
			return fail(ctx, http.StatusForbidden, "BIND_PENDING", "绑定申请待老师审批")
		}
		if err == nil && guardian.Status == "rejected" {
			return fail(ctx, http.StatusForbidden, "BIND_REJECTED", "绑定申请已被驳回，请联系老师")
		}
		if err == nil && guardian.Status != "active" {
			return fail(ctx, http.StatusForbidden, "GUARDIAN_DISABLED", "账号不可用")
		}
	}
	if err != nil {
		return mapCRMBizError(ctx, err)
	}
	if guardian == nil || guardian.Status != "active" {
		return fail(ctx, http.StatusForbidden, "BIND_PENDING", "绑定尚未通过")
	}
	if len(students) == 0 {
		if home, herr := s.crm.PortalHome(ctx, guardian.ID); herr == nil {
			students = home.Students
		}
	}
	tok, exp, err := s.tokens.Issue(guardian.ID, "guardian", guardian.Phone, biz.NewID())
	if err != nil {
		return fail(ctx, http.StatusInternalServerError, "INTERNAL", "登录失败")
	}
	out := make([]studentDTO, 0, len(students))
	for _, st := range students {
		out = append(out, toStudentDTO(st))
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"token": tok, "expires_at": exp.UTC().Format("2006-01-02T15:04:05Z"),
		"guardian": map[string]any{"id": guardian.ID, "phone": guardian.Phone, "status": guardian.Status},
		"students": out,
	})
}

func (s *Server) requireGuardian(ctx khttp.Context) (string, error) {
	if s.tokens == nil {
		return "", fail(ctx, http.StatusServiceUnavailable, "AUTH_DISABLED", "数据库未配置")
	}
	raw := strings.TrimSpace(ctx.Header().Get("Authorization"))
	if raw == "" {
		if v := strings.TrimSpace(ctx.Header().Get("X-HM-Token")); v != "" {
			raw = "Bearer " + v
		}
	}
	if !strings.HasPrefix(raw, "Bearer ") {
		return "", fail(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "未登录")
	}
	claims, err := s.tokens.Parse(strings.TrimSpace(strings.TrimPrefix(raw, "Bearer ")))
	if err != nil || claims.Role != "guardian" {
		return "", fail(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "登录已过期")
	}
	return claims.Subject, nil
}

func (s *Server) PortalStudent(ctx khttp.Context) error {
	gid, err := s.requireGuardian(ctx)
	if err != nil {
		return err
	}
	detail, err := s.crm.PortalStudent(ctx, gid, ctx.Vars().Get("id"))
	if err != nil {
		return mapCRMBizError(ctx, err)
	}
	tasks := make([]map[string]any, 0, len(detail.Tasks))
	for _, t := range detail.Tasks {
		item := map[string]any{"id": t.ID, "title": t.Title, "status": t.Status}
		if t.DueAt != nil {
			item["due_at"] = t.DueAt.UTC().Format("2006-01-02T15:04:05Z")
		}
		tasks = append(tasks, item)
	}
	lessons := make([]map[string]any, 0, len(detail.Lessons))
	for _, l := range detail.Lessons {
		lessons = append(lessons, map[string]any{
			"id": l.ID, "kind": l.Kind, "delta_hours": l.DeltaHours,
			"balance_after": l.BalanceAfter, "note": l.Note,
			"created_at": l.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	var expires any
	if detail.ExpiresAt != nil {
		expires = detail.ExpiresAt.Format("2006-01-02")
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"student":       toPortalStudentDTO(detail.Student),
		"tasks":         tasks,
		"balance_hours": detail.BalanceHours,
		"expires_at":    expires,
		"lessons":       lessons,
	})
}

func (s *Server) PortalHome(ctx khttp.Context) error {
	gid, err := s.requireGuardian(ctx)
	if err != nil {
		return err
	}
	home, err := s.crm.PortalHome(ctx, gid)
	if err != nil {
		return mapCRMBizError(ctx, err)
	}
	students := make([]portalStudentDTO, 0, len(home.Students))
	for _, st := range home.Students {
		students = append(students, toPortalStudentDTO(st))
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"guardian":      map[string]any{"name": home.GuardianName, "phone": home.GuardianPhone},
		"students":      students,
		"pending_tasks": home.PendingTasks,
		"balance_hours": home.BalanceHours,
	})
}

func mapCRMBizError(ctx khttp.Context, err error) error {
	switch {
	case errors.Is(err, biz.ErrStudentNotFound):
		return fail(ctx, http.StatusNotFound, "STUDENT_NOT_FOUND", "学员不存在")
	case errors.Is(err, biz.ErrGuardianNotFound):
		return fail(ctx, http.StatusNotFound, "GUARDIAN_NOT_FOUND", "家长不存在")
	case errors.Is(err, biz.ErrInvalidStudent), errors.Is(err, biz.ErrInvalidFollowUp), errors.Is(err, biz.ErrInvalidCode):
		return fail(ctx, http.StatusBadRequest, "INVALID_INPUT", "参数不合法")
	case errors.Is(err, biz.ErrCodeExpired):
		return fail(ctx, http.StatusBadRequest, "CODE_EXPIRED", "绑定码已过期")
	case errors.Is(err, biz.ErrCodeUsed):
		return fail(ctx, http.StatusBadRequest, "CODE_USED", "绑定码已被使用")
	case errors.Is(err, biz.ErrGuardianDisabled):
		return fail(ctx, http.StatusForbidden, "GUARDIAN_DISABLED", "账号不可用")
	case errors.Is(err, biz.ErrForbidden):
		return fail(ctx, http.StatusForbidden, "FORBIDDEN", "没有权限")
	default:
		return fail(ctx, http.StatusInternalServerError, "INTERNAL", "操作失败")
	}
}
