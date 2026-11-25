package biz

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubNoticeRepo struct {
	NoticeRepo
	created    *Notice
	sent       *Notice
	sendCount  int
	renotified []string
	status     string
}

func (s *stubNoticeRepo) ListNotices(context.Context, string, int) ([]Notice, error) {
	return nil, nil
}

func (s *stubNoticeRepo) GetNotice(_ context.Context, id string) (*Notice, error) {
	if id == "" {
		return nil, ErrNoticeNotFound
	}
	status := s.status
	if status == "" {
		status = NoticeDraft
	}
	return &Notice{ID: id, Title: "通知", Status: status}, nil
}

func (s *stubNoticeRepo) CreateNotice(_ context.Context, notice *Notice) error {
	s.created = notice
	return nil
}

func (s *stubNoticeRepo) UpdateNotice(context.Context, *Notice) error { return nil }

func (s *stubNoticeRepo) DeleteNotice(_ context.Context, id string) error {
	if id == "" {
		return ErrNoticeNotFound
	}
	return nil
}

func (s *stubNoticeRepo) SendNotice(_ context.Context, notice *Notice) (int, error) {
	s.sent = notice
	s.sendCount++
	return 3, nil
}

func (s *stubNoticeRepo) ListReceipts(context.Context, string, string) ([]NoticeReceipt, error) {
	return nil, nil
}

func (s *stubNoticeRepo) RemindPending(context.Context, string) ([]NoticeReceipt, error) {
	return []NoticeReceipt{{ID: "r1"}, {ID: "r2"}}, nil
}

func (s *stubNoticeRepo) ConfirmReceipt(context.Context, string, string) error { return nil }

func (s *stubNoticeRepo) PortalNotices(context.Context, string) ([]Notice, error) {
	return nil, nil
}

func (s *stubNoticeRepo) RenotifyDue(context.Context, time.Time) ([]NoticeReceipt, error) {
	return []NoticeReceipt{{ID: "r1"}, {ID: "r2"}}, nil
}

func (s *stubNoticeRepo) MarkRenotified(_ context.Context, ids []string) error {
	s.renotified = ids
	return nil
}

func TestCreateNoticeValidation(t *testing.T) {
	cases := []struct {
		name string
		in   NoticeInput
		ok   bool
	}{
		{"全员通知", NoticeInput{Title: "家长会", Body: "本周六 9:00", AudienceType: AudienceAll}, true},
		{"缺标题", NoticeInput{Body: "x", AudienceType: AudienceAll}, false},
		{"缺正文", NoticeInput{Title: "x", AudienceType: AudienceAll}, false},
		{"受众类型非法", NoticeInput{Title: "x", Body: "y", AudienceType: "everyone"}, false},
		{"分组缺值", NoticeInput{Title: "x", Body: "y", AudienceType: AudienceGroup}, false},
		{"指定学员缺值", NoticeInput{Title: "x", Body: "y", AudienceType: AudienceStudents}, false},
		{"布尔分组值", NoticeInput{Title: "x", Body: "y", AudienceType: AudienceGroup, AudienceValue: "class_active"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &stubNoticeRepo{}
			uc := NewNoticeUsecase(repo)
			_, err := uc.Create(context.Background(), "s1", tc.in)
			if tc.ok && err != nil {
				t.Fatalf("期望成功，得到 %v", err)
			}
			if !tc.ok && !errors.Is(err, ErrInvalidNotice) {
				t.Fatalf("期望 ErrInvalidNotice，得到 %v", err)
			}
			if tc.ok && (repo.created == nil || repo.created.Status != NoticeDraft || repo.created.CreatedBy != "s1") {
				t.Fatalf("草稿未落库: %+v", repo.created)
			}
		})
	}
}

func TestSendNoticeSkipsEmptyAudience(t *testing.T) {
	repo := &stubNoticeRepo{}
	uc := NewNoticeUsecase(repo)
	notice, count, err := uc.Send(context.Background(), "n1")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if count != 3 || notice.Status != NoticeSent || notice.SentAt == nil {
		t.Fatalf("发送结果不对: %+v count=%d", notice, count)
	}
}

func TestSendNoticeNotFound(t *testing.T) {
	uc := NewNoticeUsecase(&stubNoticeRepo{})
	if _, _, err := uc.Send(context.Background(), ""); !errors.Is(err, ErrNoticeNotFound) {
		t.Fatalf("期望 ErrNoticeNotFound，得到 %v", err)
	}
}

func TestRemindCounts(t *testing.T) {
	uc := NewNoticeUsecase(&stubNoticeRepo{status: NoticeSent})
	count, err := uc.Remind(context.Background(), "n1")
	if err != nil {
		t.Fatalf("Remind: %v", err)
	}
	if count != 2 {
		t.Fatalf("催办数量 = %d, want 2", count)
	}
}

func TestRenotifyDueMarksReceipts(t *testing.T) {
	repo := &stubNoticeRepo{}
	uc := NewNoticeUsecase(repo)
	receipts, err := uc.RenotifyDue(context.Background(), time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("RenotifyDue: %v", err)
	}
	if len(receipts) != 2 {
		t.Fatalf("再推数量 = %d, want 2", len(receipts))
	}
	if len(repo.renotified) != 2 {
		t.Fatalf("应标记 2 条已再推，得到 %v", repo.renotified)
	}
}

func TestNormalizeNoticeFileLimit(t *testing.T) {
	files := make([]string, 12)
	for i := range files {
		files[i] = "cloud://file"
	}
	repo := &stubNoticeRepo{}
	uc := NewNoticeUsecase(repo)
	if _, err := uc.Create(context.Background(), "s1", NoticeInput{
		Title:        "标题",
		Body:         "正文",
		FileIDs:      files,
		AudienceType: AudienceAll,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(repo.created.FileIDs) != 9 {
		t.Fatalf("附件应截断到 9 个，得到 %d", len(repo.created.FileIDs))
	}
}
