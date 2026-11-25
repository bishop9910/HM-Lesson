package biz

import (
	"context"
	"errors"
	"strings"
	"time"
)

// 通知状态（docs/backend-requirements.md §3.7）。
const (
	NoticeDraft = "draft"
	NoticeSent  = "sent"
)

// 通知受众类型。
const (
	AudienceAll      = "all"
	AudienceGroup    = "group"
	AudienceStudents = "students"
)

// 回执状态。
const (
	ReceiptPending   = "pending"
	ReceiptConfirmed = "confirmed"
)

// NoticeRenotifyAfter 是「48 小时未确认自动再推一次」的窗口（AF-15）。
const NoticeRenotifyAfter = 48 * time.Hour

var (
	// ErrNoticeNotFound 通知不存在。
	ErrNoticeNotFound = errors.New("通知不存在")
	// ErrReceiptNotFound 回执不存在。
	ErrReceiptNotFound = errors.New("回执不存在")
	// ErrInvalidNotice 通知参数不合法。
	ErrInvalidNotice = errors.New("通知内容不合法")
	// ErrNoticeState 通知状态不允许该操作。
	ErrNoticeState = errors.New("通知状态不允许该操作")
	// ErrNoticeNoAudience 受众为空。
	ErrNoticeNoAudience = errors.New("没有符合条件的家长需要通知")
)

// Notice 通知。
type Notice struct {
	ID            string
	Title         string
	Body          string
	FileIDs       []string
	AudienceType  string
	AudienceValue string
	NeedReceipt   bool
	Status        string
	SentAt        *time.Time
	CreatedBy     string
	CreatorName   string
	CreatedAt     time.Time
	ReceiptTotal  int
	ReceiptDone   int
	// 家长端专用：当前家长的回执状态。
	MyReceiptStatus string
	MyConfirmedAt   *time.Time
}

// NoticeReceipt 回执。
type NoticeReceipt struct {
	ID           string
	NoticeID     string
	GuardianID   string
	GuardianName string
	Phone        string
	StudentNames string
	Status       string
	ConfirmedAt  *time.Time
	RemindedAt   *time.Time
	CreatedAt    time.Time
}

// NoticeInput 创建/更新通知入参。
type NoticeInput struct {
	Title         string
	Body          string
	FileIDs       []string
	AudienceType  string
	AudienceValue string
	NeedReceipt   bool
}

// NoticeRepo 通知仓储。
type NoticeRepo interface {
	ListNotices(ctx context.Context, status string, limit int) ([]Notice, error)
	GetNotice(ctx context.Context, noticeID string) (*Notice, error)
	CreateNotice(ctx context.Context, notice *Notice) error
	UpdateNotice(ctx context.Context, notice *Notice) error
	DeleteNotice(ctx context.Context, noticeID string) error
	// SendNotice 在同一事务里快照受众、生成回执、写入家长消息中心。
	SendNotice(ctx context.Context, notice *Notice) (int, error)
	ListReceipts(ctx context.Context, noticeID, status string) ([]NoticeReceipt, error)
	// RemindPending 记录催办时间并返回仍需通知的家长数量。
	RemindPending(ctx context.Context, noticeID string) ([]NoticeReceipt, error)
	ConfirmReceipt(ctx context.Context, noticeID, guardianID string) error
	PortalNotices(ctx context.Context, guardianID string) ([]Notice, error)
	// RenotifyDue 找出 48h 未确认且未再推过的回执，返回需要再推的通知与家长。
	RenotifyDue(ctx context.Context, before time.Time) ([]NoticeReceipt, error)
	MarkRenotified(ctx context.Context, receiptIDs []string) error
}

// NoticeUsecase 通知用例。
type NoticeUsecase struct {
	repo NoticeRepo
}

// NewNoticeUsecase 构造通知用例。
func NewNoticeUsecase(repo NoticeRepo) *NoticeUsecase {
	return &NoticeUsecase{repo: repo}
}

// List 通知列表。
func (uc *NoticeUsecase) List(ctx context.Context, status string, limit int) ([]Notice, error) {
	status = strings.TrimSpace(status)
	if status != "" && status != NoticeDraft && status != NoticeSent {
		return nil, ErrInvalidNotice
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	return uc.repo.ListNotices(ctx, status, limit)
}

// Get 通知详情。
func (uc *NoticeUsecase) Get(ctx context.Context, noticeID string) (*Notice, error) {
	return uc.repo.GetNotice(ctx, strings.TrimSpace(noticeID))
}

// Create 创建草稿。
func (uc *NoticeUsecase) Create(ctx context.Context, actorID string, in NoticeInput) (*Notice, error) {
	normalized, files, err := normalizeNotice(in)
	if err != nil {
		return nil, err
	}
	notice := &Notice{
		ID:            NewID(),
		Title:         normalized.Title,
		Body:          normalized.Body,
		FileIDs:       files,
		AudienceType:  normalized.AudienceType,
		AudienceValue: normalized.AudienceValue,
		NeedReceipt:   normalized.NeedReceipt,
		Status:        NoticeDraft,
		CreatedBy:     actorID,
		CreatedAt:     time.Now().UTC(),
	}
	if err := uc.repo.CreateNotice(ctx, notice); err != nil {
		return nil, err
	}
	return notice, nil
}

// Update 修改草稿；已发送的通知不可改。
func (uc *NoticeUsecase) Update(ctx context.Context, noticeID string, in NoticeInput) (*Notice, error) {
	notice, err := uc.repo.GetNotice(ctx, strings.TrimSpace(noticeID))
	if err != nil {
		return nil, err
	}
	if notice.Status != NoticeDraft {
		return nil, ErrNoticeState
	}
	normalized, files, err := normalizeNotice(in)
	if err != nil {
		return nil, err
	}
	notice.Title = normalized.Title
	notice.Body = normalized.Body
	notice.FileIDs = files
	notice.AudienceType = normalized.AudienceType
	notice.AudienceValue = normalized.AudienceValue
	notice.NeedReceipt = normalized.NeedReceipt
	if err := uc.repo.UpdateNotice(ctx, notice); err != nil {
		return nil, err
	}
	return notice, nil
}

// Delete 删除草稿。
func (uc *NoticeUsecase) Delete(ctx context.Context, noticeID string) error {
	notice, err := uc.repo.GetNotice(ctx, strings.TrimSpace(noticeID))
	if err != nil {
		return err
	}
	if notice.Status != NoticeDraft {
		return ErrNoticeState
	}
	return uc.repo.DeleteNotice(ctx, notice.ID)
}

// Send 发送：快照受众、按家长生成回执、写入消息中心。
func (uc *NoticeUsecase) Send(ctx context.Context, noticeID string) (*Notice, int, error) {
	notice, err := uc.repo.GetNotice(ctx, strings.TrimSpace(noticeID))
	if err != nil {
		return nil, 0, err
	}
	if notice.Status != NoticeDraft {
		return nil, 0, ErrNoticeState
	}
	count, err := uc.repo.SendNotice(ctx, notice)
	if err != nil {
		return nil, 0, err
	}
	now := time.Now().UTC()
	notice.Status = NoticeSent
	notice.SentAt = &now
	notice.ReceiptTotal = count
	return notice, count, nil
}

// Receipts 回执列表，status 为空表示全部。
func (uc *NoticeUsecase) Receipts(ctx context.Context, noticeID, status string) ([]NoticeReceipt, error) {
	status = strings.TrimSpace(status)
	if status != "" && status != ReceiptPending && status != ReceiptConfirmed {
		return nil, ErrInvalidNotice
	}
	return uc.repo.ListReceipts(ctx, strings.TrimSpace(noticeID), status)
}

// Remind 催办未确认的家长，返回催办条数。
func (uc *NoticeUsecase) Remind(ctx context.Context, noticeID string) (int, error) {
	notice, err := uc.repo.GetNotice(ctx, strings.TrimSpace(noticeID))
	if err != nil {
		return 0, err
	}
	if notice.Status != NoticeSent {
		return 0, ErrNoticeState
	}
	receipts, err := uc.repo.RemindPending(ctx, notice.ID)
	if err != nil {
		return 0, err
	}
	return len(receipts), nil
}

// Confirm 家长确认回执（幂等）。
func (uc *NoticeUsecase) Confirm(ctx context.Context, guardianID, noticeID string) error {
	return uc.repo.ConfirmReceipt(ctx, strings.TrimSpace(noticeID), guardianID)
}

// Portal 家长消息中心。
func (uc *NoticeUsecase) Portal(ctx context.Context, guardianID string) ([]Notice, error) {
	return uc.repo.PortalNotices(ctx, guardianID)
}

// RenotifyDue 返回超过 48 小时仍未确认的回执，供后台再推一次（AF-15）。
func (uc *NoticeUsecase) RenotifyDue(ctx context.Context, now time.Time) ([]NoticeReceipt, error) {
	receipts, err := uc.repo.RenotifyDue(ctx, now.Add(-NoticeRenotifyAfter))
	if err != nil || len(receipts) == 0 {
		return nil, err
	}
	ids := make([]string, 0, len(receipts))
	for _, item := range receipts {
		ids = append(ids, item.ID)
	}
	if err := uc.repo.MarkRenotified(ctx, ids); err != nil {
		return nil, err
	}
	return receipts, nil
}

func normalizeNotice(in NoticeInput) (NoticeInput, []string, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" || len([]rune(title)) > 128 {
		return in, nil, ErrInvalidNotice
	}
	body := strings.TrimSpace(in.Body)
	if body == "" || len([]rune(body)) > 5000 {
		return in, nil, ErrInvalidNotice
	}
	audienceType := strings.TrimSpace(in.AudienceType)
	if audienceType == "" {
		audienceType = AudienceAll
	}
	if audienceType != AudienceAll && audienceType != AudienceGroup && audienceType != AudienceStudents {
		return in, nil, ErrInvalidNotice
	}
	audienceValue := strings.TrimSpace(in.AudienceValue)
	if audienceType == AudienceGroup && audienceValue == "" {
		return in, nil, ErrInvalidNotice
	}
	if audienceType == AudienceStudents && audienceValue == "" {
		return in, nil, ErrInvalidNotice
	}
	files := make([]string, 0, len(in.FileIDs))
	for _, id := range in.FileIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		files = append(files, trimmed)
		if len(files) == 9 {
			break
		}
	}
	out := NoticeInput{
		Title:         title,
		Body:          body,
		FileIDs:       files,
		AudienceType:  audienceType,
		AudienceValue: audienceValue,
		NeedReceipt:   in.NeedReceipt,
	}
	return out, files, nil
}
