package service

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	khttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/xuri/excelize/v2"

	"github.com/bishop9910/hm-lesson/backend/internal/biz"
)

// ---- DTO ----

type noticeDTO struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Body          string   `json:"body"`
	FileIDs       []string `json:"file_ids"`
	AudienceType  string   `json:"audience_type"`
	AudienceValue string   `json:"audience_value"`
	NeedReceipt   bool     `json:"need_receipt"`
	Status        string   `json:"status"`
	SentAt        string   `json:"sent_at"`
	CreatorName   string   `json:"creator_name"`
	CreatedAt     string   `json:"created_at"`
	ReceiptTotal  int      `json:"receipt_total"`
	ReceiptDone   int      `json:"receipt_done"`
	MyReceipt     string   `json:"my_receipt_status,omitempty"`
	MyConfirmedAt string   `json:"my_confirmed_at,omitempty"`
}

type receiptDTO struct {
	ID           string `json:"id"`
	GuardianID   string `json:"guardian_id"`
	GuardianName string `json:"guardian_name"`
	Phone        string `json:"phone"`
	StudentNames string `json:"student_names"`
	Status       string `json:"status"`
	ConfirmedAt  string `json:"confirmed_at"`
	RemindedAt   string `json:"reminded_at"`
	CreatedAt    string `json:"created_at"`
}

func timeText(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

func toNoticeDTO(item *biz.Notice) noticeDTO {
	files := item.FileIDs
	if files == nil {
		files = []string{}
	}
	return noticeDTO{
		ID:            item.ID,
		Title:         item.Title,
		Body:          item.Body,
		FileIDs:       files,
		AudienceType:  item.AudienceType,
		AudienceValue: item.AudienceValue,
		NeedReceipt:   item.NeedReceipt,
		Status:        item.Status,
		SentAt:        timeText(item.SentAt),
		CreatorName:   item.CreatorName,
		CreatedAt:     materialTime(item.CreatedAt),
		ReceiptTotal:  item.ReceiptTotal,
		ReceiptDone:   item.ReceiptDone,
		MyReceipt:     item.MyReceiptStatus,
		MyConfirmedAt: timeText(item.MyConfirmedAt),
	}
}

func toReceiptDTO(item *biz.NoticeReceipt) receiptDTO {
	return receiptDTO{
		ID:           item.ID,
		GuardianID:   item.GuardianID,
		GuardianName: item.GuardianName,
		Phone:        item.Phone,
		StudentNames: item.StudentNames,
		Status:       item.Status,
		ConfirmedAt:  timeText(item.ConfirmedAt),
		RemindedAt:   timeText(item.RemindedAt),
		CreatedAt:    materialTime(item.CreatedAt),
	}
}

func (s *Server) requireNotices(ctx khttp.Context) error {
	if s.notices == nil {
		return fail(ctx, http.StatusServiceUnavailable, "NOTICE_DISABLED", "通知未启用")
	}
	return nil
}

// ---- 老师端 ----

// ListNotices 通知列表。
func (s *Server) ListNotices(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	if err := s.requireNotices(ctx); err != nil {
		return err
	}
	limit, _ := parsePositiveInt(ctx.Query().Get("limit"))
	items, err := s.notices.List(ctx, ctx.Query().Get("status"), limit)
	if err != nil {
		return mapNoticeError(ctx, err)
	}
	out := make([]noticeDTO, 0, len(items))
	for i := range items {
		out = append(out, toNoticeDTO(&items[i]))
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": out})
}

type noticeRequest struct {
	Title         string   `json:"title"`
	Body          string   `json:"body"`
	FileIDs       []string `json:"file_ids"`
	AudienceType  string   `json:"audience_type"`
	AudienceValue string   `json:"audience_value"`
	NeedReceipt   bool     `json:"need_receipt"`
}

func (b noticeRequest) input() biz.NoticeInput {
	return biz.NoticeInput{
		Title:         b.Title,
		Body:          b.Body,
		FileIDs:       b.FileIDs,
		AudienceType:  b.AudienceType,
		AudienceValue: b.AudienceValue,
		NeedReceipt:   b.NeedReceipt,
	}
}

// CreateNotice 新建通知草稿。
func (s *Server) CreateNotice(ctx khttp.Context) error {
	actor, err := s.requireStaff(ctx, false)
	if err != nil {
		return err
	}
	if err := s.requireNotices(ctx); err != nil {
		return err
	}
	var req noticeRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "VALIDATION_FAILED", "请求格式不正确")
	}
	notice, err := s.notices.Create(ctx, actor.ID, req.input())
	if err != nil {
		return mapNoticeError(ctx, err)
	}
	return ctx.JSON(http.StatusCreated, map[string]any{"notice": toNoticeDTO(notice)})
}

// GetNotice 通知详情。
func (s *Server) GetNotice(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	if err := s.requireNotices(ctx); err != nil {
		return err
	}
	notice, err := s.notices.Get(ctx, ctx.Vars().Get("id"))
	if err != nil {
		return mapNoticeError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"notice": toNoticeDTO(notice)})
}

// UpdateNotice 修改草稿。
func (s *Server) UpdateNotice(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	if err := s.requireNotices(ctx); err != nil {
		return err
	}
	var req noticeRequest
	if err := ctx.Bind(&req); err != nil {
		return fail(ctx, http.StatusBadRequest, "VALIDATION_FAILED", "请求格式不正确")
	}
	notice, err := s.notices.Update(ctx, ctx.Vars().Get("id"), req.input())
	if err != nil {
		return mapNoticeError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"notice": toNoticeDTO(notice)})
}

// DeleteNotice 删除草稿。
func (s *Server) DeleteNotice(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	if err := s.requireNotices(ctx); err != nil {
		return err
	}
	if err := s.notices.Delete(ctx, ctx.Vars().Get("id")); err != nil {
		return mapNoticeError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"ok": true})
}

// SendNotice 发送通知并快照受众。
func (s *Server) SendNotice(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	if err := s.requireNotices(ctx); err != nil {
		return err
	}
	notice, count, err := s.notices.Send(ctx, ctx.Vars().Get("id"))
	if err != nil {
		return mapNoticeError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"notice": toNoticeDTO(notice), "receipts": count})
}

// ListNoticeReceipts 回执列表。
func (s *Server) ListNoticeReceipts(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	if err := s.requireNotices(ctx); err != nil {
		return err
	}
	items, err := s.notices.Receipts(ctx, ctx.Vars().Get("id"), ctx.Query().Get("status"))
	if err != nil {
		return mapNoticeError(ctx, err)
	}
	out := make([]receiptDTO, 0, len(items))
	for i := range items {
		out = append(out, toReceiptDTO(&items[i]))
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": out})
}

// RemindNotice 一键催办未确认家长。
func (s *Server) RemindNotice(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, false); err != nil {
		return err
	}
	if err := s.requireNotices(ctx); err != nil {
		return err
	}
	count, err := s.notices.Remind(ctx, ctx.Vars().Get("id"))
	if err != nil {
		return mapNoticeError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"ok": true, "reminded": count})
}

// ExportNoticeReceipts 导出回执 xlsx（小批量同步，最多 2000 行）。
func (s *Server) ExportNoticeReceipts(ctx khttp.Context) error {
	if _, err := s.requireStaff(ctx, true); err != nil {
		return err
	}
	if err := s.requireNotices(ctx); err != nil {
		return err
	}
	noticeID := ctx.Vars().Get("id")
	notice, err := s.notices.Get(ctx, noticeID)
	if err != nil {
		return mapNoticeError(ctx, err)
	}
	items, err := s.notices.Receipts(ctx, noticeID, "")
	if err != nil {
		return mapNoticeError(ctx, err)
	}
	if len(items) > 2000 {
		items = items[:2000]
	}
	book := excelize.NewFile()
	defer func() { _ = book.Close() }()
	sheet := "回执"
	index, err := book.NewSheet(sheet)
	if err != nil {
		return fail(ctx, http.StatusInternalServerError, "EXPORT_FAILED", "生成表格失败")
	}
	book.SetActiveSheet(index)
	headers := []string{"家长姓名", "手机号", "学员", "状态", "确认时间", "催办时间"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := book.SetCellValue(sheet, cell, header); err != nil {
			return fail(ctx, http.StatusInternalServerError, "EXPORT_FAILED", "生成表格失败")
		}
	}
	for rowIndex, item := range items {
		status := "未确认"
		if item.Status == biz.ReceiptConfirmed {
			status = "已确认"
		}
		values := []string{
			item.GuardianName,
			item.Phone,
			item.StudentNames,
			status,
			timeText(item.ConfirmedAt),
			timeText(item.RemindedAt),
		}
		for colIndex, value := range values {
			cell, _ := excelize.CoordinatesToCellName(colIndex+1, rowIndex+2)
			if err := book.SetCellValue(sheet, cell, value); err != nil {
				return fail(ctx, http.StatusInternalServerError, "EXPORT_FAILED", "生成表格失败")
			}
		}
	}
	buffer, err := book.WriteToBuffer()
	if err != nil {
		return fail(ctx, http.StatusInternalServerError, "EXPORT_FAILED", "生成表格失败")
	}
	filename := fmt.Sprintf("回执-%s-%s.xlsx", notice.Title, time.Now().UTC().Add(8*time.Hour).Format("20060102"))
	ctx.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	ctx.Response().Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(filename))
	ctx.Response().WriteHeader(http.StatusOK)
	_, err = ctx.Response().Write(buffer.Bytes())
	return err
}

// ---- 家长端 ----

// PortalNotices 家长消息中心（含需回执的通知）。
func (s *Server) PortalNotices(ctx khttp.Context) error {
	guardianID, err := s.requireGuardian(ctx)
	if err != nil {
		return err
	}
	if err := s.requireNotices(ctx); err != nil {
		return err
	}
	items, err := s.notices.Portal(ctx, guardianID)
	if err != nil {
		return mapNoticeError(ctx, err)
	}
	out := make([]noticeDTO, 0, len(items))
	for i := range items {
		out = append(out, toNoticeDTO(&items[i]))
	}
	return ctx.JSON(http.StatusOK, map[string]any{"items": out})
}

// ConfirmNotice 家长确认回执（幂等）。
func (s *Server) ConfirmNotice(ctx khttp.Context) error {
	guardianID, err := s.requireGuardian(ctx)
	if err != nil {
		return err
	}
	if err := s.requireNotices(ctx); err != nil {
		return err
	}
	if err := s.notices.Confirm(ctx, guardianID, ctx.Vars().Get("id")); err != nil {
		return mapNoticeError(ctx, err)
	}
	return ctx.JSON(http.StatusOK, map[string]any{"ok": true})
}

func mapNoticeError(ctx khttp.Context, err error) error {
	switch {
	case errors.Is(err, biz.ErrNoticeNotFound):
		return fail(ctx, http.StatusNotFound, "NOTICE_NOT_FOUND", "通知不存在")
	case errors.Is(err, biz.ErrReceiptNotFound):
		return fail(ctx, http.StatusNotFound, "RECEIPT_NOT_FOUND", "回执不存在")
	case errors.Is(err, biz.ErrNoticeNoAudience):
		return fail(ctx, http.StatusBadRequest, "NOTICE_NO_AUDIENCE", "没有符合条件的家长需要通知")
	case errors.Is(err, biz.ErrNoticeState):
		return fail(ctx, http.StatusConflict, "STATE_CONFLICT", "通知状态不允许该操作")
	case errors.Is(err, biz.ErrInvalidNotice):
		return fail(ctx, http.StatusBadRequest, "VALIDATION_FAILED", "请求参数不合法")
	default:
		return fail(ctx, http.StatusInternalServerError, "INTERNAL", "服务器内部错误")
	}
}
