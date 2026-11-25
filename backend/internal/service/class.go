package service

import (
	"errors"
	"net/http"
	"strings"
	"time"

	khttp "github.com/go-kratos/kratos/v3/transport/http"

	"github.com/bishop9910/hm-lesson/backend/internal/biz"
)

// ---- DTO ----

type classDTO struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Subject      string `json:"subject"`
	StartDate    string `json:"start_date"`
	EndDate      string `json:"end_date"`
	ScheduleNote string `json:"schedule_note"`
	Status       string `json:"status"`
	MemberCount  int    `json:"member_count"`
	SessionCount int    `json:"session_count"`
	CreatedAt    string `json:"created_at"`
}

type classMemberDTO struct {
	StudentID   string `json:"student_id"`
	StudentName string `json:"student_name"`
	Grade       string `json:"grade"`
	GroupCode   string `json:"group_code"`
	JoinedAt    string `json:"joined_at"`
	LeftAt      string `json:"left_at"`
}

type attendanceDTO struct {
	StudentID   string `json:"student_id"`
	StudentName string `json:"student_name"`
	Status      string `json:"status"`
	Label       string `json:"label"`
}

type classSessionDTO struct {
	ID          string          `json:"id"`
	ClassID     string          `json:"class_id"`
	SessionDate string          `json:"session_date"`
	Content     string          `json:"content"`
	CreatedAt   string          `json:"created_at"`
	Attendance  []attendanceDTO `json:"attendance"`
}

func dayText(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}

func toClassDTO(item *biz.Class) classDTO {
	return classDTO{
		ID:           item.ID,
		Name:         item.Name,
		Subject:      item.Subject,
		StartDate:    dayText(item.StartDate),
		EndDate:      dayText(item.EndDate),
		ScheduleNote: item.ScheduleNote,
		Status:       item.Status,
		MemberCount:  item.MemberCount,
		SessionCount: item.SessionCount,
		CreatedAt:    materialTime(item.CreatedAt),
	}
}

func toClassMemberDTO(item *biz.ClassMember) classMemberDTO {
	return classMemberDTO{
		StudentID:   item.StudentID,
		StudentName: item.StudentName,
		Grade:       item.Grade,
		GroupCode:   item.GroupCode,
		JoinedAt:    item.JoinedAt.Format("2006-01-02"),
		LeftAt:      dayText(item.LeftAt),
	}
}

func toClassSessionDTO(item *biz.ClassSession) classSessionDTO {
	rows := make([]attendanceDTO, 0, len(item.Attendance))
	for _, row := range item.Attendance {
		rows = append(rows, attendanceDTO{
			StudentID:   row.StudentID,
			StudentName: row.StudentName,
			Status:      row.Status,
			Label:       biz.AttendanceLabel(row.Status),
		})
	}
	return classSessionDTO{
		ID:          item.ID,
		ClassID:     item.ClassID,
		SessionDate: item.SessionDate.Format("2006-01-02"),
		Content:     item.Content,
		CreatedAt:   materialTime(item.CreatedAt),
		Attendance:  rows,
	}
}

func (s *Server) requireClasses(ctx khttp.Context) error {
	if s.classes == nil {
		return fail(ctx, http.StatusServiceUnavailable, "CLASS_DISABLED", "班课未启用")
	}
	return nil
}

// ---- 老师端 ----

// ListClasses 班级列表。
func (s *Server) ListClasses(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	if err := s.requireClasses(ctx); err != nil {
		return err
	}
	items, err := s.classes.List(ctx, ctx.Query().Get("status"))
	if err != nil {
		return mapClassError(ctx, err)
	}
	out := make([]classDTO, 0, len(items))
	for i := range items {
		out = append(out, toClassDTO(&items[i]))
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": out})
}

type classRequest struct {
	Name         string `json:"name"`
	Subject      string `json:"subject"`
	StartDate    string `json:"start_date"`
	EndDate      string `json:"end_date"`
	ScheduleNote string `json:"schedule_note"`
}

func (b classRequest) input() (biz.ClassInput, error) {
	start, err := parseDay(b.StartDate)
	if err != nil {
		return biz.ClassInput{}, err
	}
	end, err := parseDay(b.EndDate)
	if err != nil {
		return biz.ClassInput{}, err
	}
	return biz.ClassInput{
		Name:         b.Name,
		Subject:      b.Subject,
		StartDate:    start,
		EndDate:      end,
		ScheduleNote: b.ScheduleNote,
	}, nil
}

// CreateClass 新建班级。
func (s *Server) CreateClass(ctx khttp.Context) error {
	actor, err := s.requireStaff(ctx, false)
	if err != nil {
		return err
	}
	if err := s.requireClasses(ctx); err != nil {
		return err
	}
	var req classRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "VALIDATION_FAILED", "请求格式不正确")
	}
	in, err := req.input()
	if err != nil {
		return fail(ctx, http.StatusBadRequest, "VALIDATION_FAILED", "日期格式应为 YYYY-MM-DD")
	}
	class, err := s.classes.Create(ctx, actor.ID, in)
	if err != nil {
		return mapClassError(ctx, err)
	}
	return ctx.JSON(http.StatusCreated, map[string]any{"class": toClassDTO(class)})
}

// GetClass 班级详情（含成员与课次）。
func (s *Server) GetClass(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	if err := s.requireClasses(ctx); err != nil {
		return err
	}
	classID := ctx.Vars().Get("id")
	class, err := s.classes.Get(ctx, classID)
	if err != nil {
		return mapClassError(ctx, err)
	}
	members, err := s.classes.Members(ctx, classID)
	if err != nil {
		return mapClassError(ctx, err)
	}
	sessions, err := s.classes.Sessions(ctx, classID)
	if err != nil {
		return mapClassError(ctx, err)
	}
	memberOut := make([]classMemberDTO, 0, len(members))
	for i := range members {
		memberOut = append(memberOut, toClassMemberDTO(&members[i]))
	}
	sessionOut := make([]classSessionDTO, 0, len(sessions))
	for i := range sessions {
		sessionOut = append(sessionOut, toClassSessionDTO(&sessions[i]))
	}
	return ctx.JSON(http.StatusOK, map[string]any{
		"class":    toClassDTO(class),
		"members":  memberOut,
		"sessions": sessionOut,
	})
}

// UpdateClass 更新班级。
func (s *Server) UpdateClass(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	if err := s.requireClasses(ctx); err != nil {
		return err
	}
	var req classRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "VALIDATION_FAILED", "请求格式不正确")
	}
	in, err := req.input()
	if err != nil {
		return fail(ctx, http.StatusBadRequest, "VALIDATION_FAILED", "日期格式应为 YYYY-MM-DD")
	}
	class, err := s.classes.Update(ctx, ctx.Vars().Get("id"), in)
	if err != nil {
		return mapClassError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"class": toClassDTO(class)})
}

// ArchiveClass 归档班级。
func (s *Server) ArchiveClass(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, true); err != nil {
		return err
	}
	if err := s.requireClasses(ctx); err != nil {
		return err
	}
	if err := s.classes.Archive(ctx, ctx.Vars().Get("id")); err != nil {
		return mapClassError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"ok": true})
}

type classMembersRequest struct {
	StudentIDs []string `json:"student_ids"`
}

// AddClassMembers 批量加入学员。
func (s *Server) AddClassMembers(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	if err := s.requireClasses(ctx); err != nil {
		return err
	}
	var req classMembersRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "VALIDATION_FAILED", "请求格式不正确")
	}
	added, err := s.classes.AddMembers(ctx, ctx.Vars().Get("id"), req.StudentIDs)
	if err != nil {
		return mapClassError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"ok": true, "added": added})
}

// RemoveClassMember 移除成员。
func (s *Server) RemoveClassMember(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	if err := s.requireClasses(ctx); err != nil {
		return err
	}
	if err := s.classes.RemoveMember(ctx, ctx.Vars().Get("id"), ctx.Vars().Get("studentID")); err != nil {
		return mapClassError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"ok": true})
}

type sessionRequest struct {
	SessionDate string            `json:"session_date"`
	Content     string            `json:"content"`
	Attendance  map[string]string `json:"attendance"`
}

// CreateClassSession 记录一次上课。
func (s *Server) CreateClassSession(ctx khttp.Context) error {
	actor, err := s.requireStaff(ctx, false)
	if err != nil {
		return err
	}
	if err := s.requireClasses(ctx); err != nil {
		return err
	}
	var req sessionRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "VALIDATION_FAILED", "请求格式不正确")
	}
	day, err := parseDay(req.SessionDate)
	if err != nil || day == nil {
		return fail(ctx, http.StatusBadRequest, "VALIDATION_FAILED", "上课日期格式应为 YYYY-MM-DD")
	}
	session, err := s.classes.CreateSession(ctx, actor.ID, ctx.Vars().Get("id"), biz.SessionInput{
		SessionDate: *day,
		Content:     req.Content,
		Attendance:  req.Attendance,
	})
	if err != nil {
		return mapClassError(ctx, err)
	}
	return ctx.JSON(http.StatusCreated, map[string]any{"session": toClassSessionDTO(session)})
}

// DeleteClassSession 删除课次。
func (s *Server) DeleteClassSession(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	if err := s.requireClasses(ctx); err != nil {
		return err
	}
	if err := s.classes.DeleteSession(ctx, ctx.Vars().Get("sessionID")); err != nil {
		return mapClassError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"ok": true})
}

// parseDay 解析 YYYY-MM-DD；空串返回 nil。
func parseDay(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, err
	}
	utc := parsed.UTC()
	return &utc, nil
}

func mapClassError(ctx khttp.Context, err error) error {
	switch {
	case errors.Is(err, biz.ErrClassNotFound):
		return fail(ctx, http.StatusNotFound, "CLASS_NOT_FOUND", "班级不存在")
	case errors.Is(err, biz.ErrClassMemberNotFound):
		return fail(ctx, http.StatusNotFound, "CLASS_MEMBER_NOT_FOUND", "该学员不在班级里")
	case errors.Is(err, biz.ErrSessionNotFound):
		return fail(ctx, http.StatusNotFound, "SESSION_NOT_FOUND", "课次不存在")
	case errors.Is(err, biz.ErrStudentNotActive):
		return fail(ctx, http.StatusBadRequest, "STUDENT_NOT_ACTIVE", "只有在读学员可以加入班级")
	case errors.Is(err, biz.ErrClassState):
		return fail(ctx, http.StatusConflict, "STATE_CONFLICT", "班级状态不允许该操作")
	case errors.Is(err, biz.ErrInvalidClass), errors.Is(err, biz.ErrInvalidSession),
		errors.Is(err, biz.ErrStudentNotFound):
		return fail(ctx, http.StatusBadRequest, "VALIDATION_FAILED", "请求参数不合法")
	case errors.Is(err, biz.ErrForbidden):
		return fail(ctx, http.StatusNotFound, "NOT_FOUND", "资源不存在")
	default:
		return fail(ctx, http.StatusInternalServerError, "INTERNAL", "服务器内部错误")
	}
}
