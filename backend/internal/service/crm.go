package service

import (
	"errors"
	"fmt"
	"log/slog"
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

type notificationDTO struct {
	ID        string  `json:"id"`
	Kind      string  `json:"kind"`
	Title     string  `json:"title"`
	Body      string  `json:"body"`
	RefType   string  `json:"ref_type"`
	RefID     string  `json:"ref_id"`
	CreatedAt string  `json:"created_at"`
	ReadAt    *string `json:"read_at,omitempty"`
}

func toNotificationDTO(n biz.Notification) notificationDTO {
	d := notificationDTO{
		ID: n.ID, Kind: n.Kind, Title: n.Title, Body: n.Body,
		RefType: n.RefType, RefID: n.RefID,
		CreatedAt: n.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if n.ReadAt != nil {
		v := n.ReadAt.UTC().Format("2006-01-02T15:04:05Z")
		d.ReadAt = &v
	}
	return d
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

// guardianAccountDTO 是教师端家长账号列表的一行；一个家长可能对应多个学员。
type guardianAccountDTO struct {
	ID           string   `json:"id"`
	Phone        string   `json:"phone"`
	Name         string   `json:"name"`
	Status       string   `json:"status"`
	HasPassword  bool     `json:"has_password"`
	LastLoginAt  string   `json:"last_login_at,omitempty"`
	CreatedAt    string   `json:"created_at"`
	StudentIDs   []string `json:"student_ids"`
	StudentNames []string `json:"student_names"`
	// StudentCount 是仍绑定的在读学员数；为 0 且状态不是 pending 时前端提示“可清理”。
	StudentCount int `json:"student_count"`
}

func toGuardianAccountDTO(g *biz.GuardianAccount) guardianAccountDTO {
	d := guardianAccountDTO{
		ID: g.ID, Phone: g.Phone, Name: g.Name, Status: g.Status,
		HasPassword: g.HasPassword, CreatedAt: g.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		StudentIDs: g.StudentIDs, StudentNames: g.StudentNames, StudentCount: g.StudentCount(),
	}
	if g.LastLoginAt != nil {
		d.LastLoginAt = g.LastLoginAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	if d.StudentIDs == nil {
		d.StudentIDs = []string{}
	}
	if d.StudentNames == nil {
		d.StudentNames = []string{}
	}
	return d
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

// DeleteStudent 物理删除学员档案（仅 owner）。学员有课时流水时必须带 force=1 二次确认。
func (s *Server) DeleteStudent(ctx khttp.Context) error {
	actor, err := s.requireStaff(ctx, true)
	if err != nil {
		return err
	}
	id := ctx.Vars().Get("id")
	if id == "" {
		return fail(ctx, http.StatusBadRequest, "BAD_REQUEST", "缺少 id")
	}
	force := ctx.Query().Get("force") == "1" || strings.EqualFold(ctx.Query().Get("force"), "true")
	stats, err := s.crm.DeleteStudent(ctx, actor, id, force)
	if err != nil {
		if errors.Is(err, biz.ErrStudentHasLedger) {
			return fail(ctx, http.StatusConflict, "STUDENT_HAS_LESSON_LEDGER",
				fmt.Sprintf("该学员有 %d 条课时流水，删除会连同流水一起清除；确认请带 force=1 重试", stats.LessonLedger))
		}
		return mapCRMBizError(ctx, err)
	}
	s.ops.Record(ctx, actor, "student.delete", "student", id, fmt.Sprintf(
		`{"force":%t,"task_assignments":%d,"bookings":%d,"lesson_ledger":%d,"guardians":%d}`,
		force, stats.TaskAssignments, stats.Bookings, stats.LessonLedger, stats.Guardians,
	))
	return ctx.JSON(http.StatusOK, map[string]any{
		"ok": true,
		"deleted": map[string]any{
			"task_assignments": stats.TaskAssignments,
			"bookings":         stats.Bookings,
			"lesson_ledger":    stats.LessonLedger,
			"guardians":        stats.Guardians,
		},
	})
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
	// Password 是家长初始密码；留空时服务端生成一次性密码并只在本次响应里返回。
	Password string `json:"password"`
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
	password := strings.TrimSpace(req.Password)
	generated := false
	if password == "" {
		password = biz.NewTemporaryPassword()
		generated = true
	}
	if err := s.crm.ApproveGuardian(ctx, actor, ctx.Vars().Get("id"), req.StudentID, password); err != nil {
		return mapCRMBizError(ctx, err)
	}
	out := map[string]any{"status": "active"}
	if generated {
		out["initial_password"] = password
	}
	return ctx.JSON(http.StatusOK, out)
}

// ListGuardians 家长账号总览：owner 用它清理长期未绑定/无在读学员的账号。
func (s *Server) ListGuardians(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, true); err != nil {
		return err
	}
	accounts, err := s.crm.Guardians(ctx, ctx.Query().Get("status"))
	if err != nil {
		return mapCRMBizError(ctx, err)
	}
	out := make([]guardianAccountDTO, 0, len(accounts))
	for i := range accounts {
		out = append(out, toGuardianAccountDTO(&accounts[i]))
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": out})
}

type deleteGuardianRequest struct {
	// Confirm 仅用于前端二次确认，服务端不强制。
	Confirm bool `json:"confirm"`
}

// DeleteGuardian 删除家长账号；仍绑定在读学员时 409，提示先归档学员。
func (s *Server) DeleteGuardian(ctx khttp.Context) error {
	actor, err := s.requireStaff(ctx, true)
	if err != nil {
		return err
	}
	if err := s.crm.DeleteGuardian(ctx, actor, ctx.Vars().Get("id")); err != nil {
		return mapCRMBizError(ctx, err)
	}
	s.ops.Record(ctx, actor, "guardian.delete", "guardian", ctx.Vars().Get("id"), `{"ok":true}`)
	return ctx.JSON(http.StatusOK, map[string]any{"ok": true})
}

type setGuardianPasswordRequest struct {
	Password string `json:"password"`
}

// ResetGuardianPassword 由 owner 重置家长密码（家长忘记密码）。
func (s *Server) ResetGuardianPassword(ctx khttp.Context) error {
	actor, err := s.requireStaff(ctx, true)
	if err != nil {
		return err
	}
	var req setGuardianPasswordRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求格式错误")
	}
	if err := s.crm.ResetGuardianPassword(ctx, actor, ctx.Vars().Get("id"), req.Password); err != nil {
		return mapCRMBizError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"ok": true})
}

// ChangeGuardianPassword 家长自助改密；首次设置（未设过密码）允许不带旧密码。
func (s *Server) ChangeGuardianPassword(ctx khttp.Context) error {
	guardianID, err := s.requireGuardian(ctx)
	if err != nil {
		return err
	}
	var req changeGuardianPasswordRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求格式错误")
	}
	if err := s.crm.ChangeGuardianPassword(ctx, guardianID, req.OldPassword, req.NewPassword); err != nil {
		return mapCRMBizError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"ok": true})
}

type changeGuardianPasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
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
	Phone    string `json:"phone"`
	Code     string `json:"code"`
	Password string `json:"password"`
}

// GuardianLogin 家长登录：手机号 + 密码；新家长用手机号 + 6 位绑定码首次登录并设置密码。
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
		if err == nil && !guardian.HasPassword() {
			// 首次通过绑定码进入必须先设密码，否则下次又只能靠绑定码。
			if strings.TrimSpace(req.Password) == "" {
				return fail(ctx, http.StatusBadRequest, "PASSWORD_REQUIRED", "首次登录请设置密码（6~32 位）")
			}
			if perr := s.crm.SetInitialGuardianPassword(ctx, guardian.ID, req.Password); perr != nil {
				return mapCRMBizError(ctx, perr)
			}
			guardian.PasswordHash = "set"
		}
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
		if err == nil {
			if strings.TrimSpace(req.Password) == "" {
				return fail(ctx, http.StatusBadRequest, "PASSWORD_REQUIRED", "请输入密码")
			}
			if verr := s.crm.VerifyGuardianPassword(ctx, guardian, req.Password); verr != nil {
				if errors.Is(verr, biz.ErrPasswordNotSet) {
					return fail(ctx, http.StatusBadRequest, "PASSWORD_REQUIRED", "该账号还没有密码，请用绑定码登录并设置密码")
				}
				return fail(ctx, http.StatusUnauthorized, "INVALID_CREDENTIALS", "手机号或密码错误")
			}
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
	// 采集网关注入的 openid（X-WX-SOURCE 缺失时忽略），供订阅消息投递使用；失败不阻断登录。
	if openid := gatewayOpenID(ctx); openid != "" {
		if berr := s.crm.BindGuardianOpenID(ctx, guardian.ID, openid); berr != nil {
			slog.Warn("采集家长 openid 失败", "guardian_id", guardian.ID, "error", berr)
		}
	}
	tok, exp, err := s.tokens.Issue(guardian.ID, "guardian", guardian.Phone, biz.NewID())
	if err != nil {
		return fail(ctx, http.StatusInternalServerError, "INTERNAL", "登录失败")
	}
	out := make([]portalStudentDTO, 0, len(students))
	for _, st := range students {
		out = append(out, toPortalStudentDTO(st))
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"token": tok, "expires_at": exp.UTC().Format("2006-01-02T15:04:05Z"),
		"guardian": map[string]any{
			"id": guardian.ID, "phone": guardian.Phone, "status": guardian.Status,
			"has_password": guardian.HasPassword(),
		},
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

func (s *Server) PortalNotifications(ctx khttp.Context) error {
	gid, err := s.requireGuardian(ctx)
	if err != nil {
		return err
	}
	items, err := s.crm.UnreadNotifications(ctx, gid, 20)
	if err != nil {
		return mapCRMBizError(ctx, err)
	}
	out := make([]notificationDTO, 0, len(items))
	for _, n := range items {
		out = append(out, toNotificationDTO(n))
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": out, "total": len(out)})
}

func (s *Server) ReadNotification(ctx khttp.Context) error {
	gid, err := s.requireGuardian(ctx)
	if err != nil {
		return err
	}
	if err := s.crm.MarkNotificationRead(ctx, gid, ctx.Vars().Get("id")); err != nil {
		return mapCRMBizError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) ReadAllNotifications(ctx khttp.Context) error {
	gid, err := s.requireGuardian(ctx)
	if err != nil {
		return err
	}
	if err := s.crm.MarkAllNotificationsRead(ctx, gid); err != nil {
		return mapCRMBizError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"ok": true})
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
	case errors.Is(err, biz.ErrWrongPassword):
		return fail(ctx, http.StatusForbidden, "WRONG_PASSWORD", "旧密码不正确")
	case errors.Is(err, biz.ErrPasswordNotSet):
		return fail(ctx, http.StatusBadRequest, "PASSWORD_REQUIRED", "该账号还没有密码，请先在门户设置密码")
	case errors.Is(err, biz.ErrWeakPassword):
		return fail(ctx, http.StatusBadRequest, "WEAK_PASSWORD", "密码长度需要 6~32 位")
	case errors.Is(err, biz.ErrGuardianHasStudents):
		return fail(ctx, http.StatusConflict, "GUARDIAN_HAS_STUDENTS", "该家长仍绑定在读学员，请先归档学员")
	case errors.Is(err, biz.ErrStudentHasLedger):
		return fail(ctx, http.StatusConflict, "STUDENT_HAS_LESSON_LEDGER", "该学员有课时流水，请确认后带 force=1 重试")
	default:
		return fail(ctx, http.StatusInternalServerError, "INTERNAL", "操作失败")
	}
}
