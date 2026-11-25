package biz

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeSubRepo struct {
	grants   map[string]int
	delivers []Delivery

	sentIDs    []string
	retriedIDs []string
	doneIDs    []string
	doneStatus []string
	addCalls   int
}

func newFakeSubRepo() *fakeSubRepo {
	return &fakeSubRepo{grants: map[string]int{}}
}

func (f *fakeSubRepo) ListGrants(_ context.Context, _ string) ([]SubscriptionGrant, error) {
	out := make([]SubscriptionGrant, 0, len(f.grants))
	for k, v := range f.grants {
		out = append(out, SubscriptionGrant{TemplateKey: k, Remaining: v})
	}
	return out, nil
}

func (f *fakeSubRepo) AddGrant(_ context.Context, _, key string, delta, max int) (int, error) {
	f.addCalls++
	next := f.grants[key] + delta
	if next > max {
		next = max
	}
	f.grants[key] = next
	return next, nil
}

func (f *fakeSubRepo) ClaimDeliveries(_ context.Context, limit int) ([]Delivery, error) {
	if len(f.delivers) > limit {
		return f.delivers[:limit], nil
	}
	return f.delivers, nil
}

func (f *fakeSubRepo) MarkDeliverySent(_ context.Context, id string) error {
	f.sentIDs = append(f.sentIDs, id)
	return nil
}

func (f *fakeSubRepo) MarkDeliveryRetry(_ context.Context, id, _ string, _ time.Time) error {
	f.retriedIDs = append(f.retriedIDs, id)
	return nil
}

func (f *fakeSubRepo) MarkDeliveryDone(_ context.Context, id, status, _ string) error {
	f.doneIDs = append(f.doneIDs, id)
	f.doneStatus = append(f.doneStatus, status)
	return nil
}

func (f *fakeSubRepo) ResetStaleDeliveries(_ context.Context, _ time.Time) error { return nil }

type fakeSender struct {
	calls []SubscribeMessage
	fn    func(SubscribeMessage) (bool, error)
}

func (f *fakeSender) SendSubscribe(_ context.Context, msg SubscribeMessage) (bool, error) {
	f.calls = append(f.calls, msg)
	if f.fn != nil {
		return f.fn(msg)
	}
	return false, nil
}

func testTemplates() map[string]SubscribeTemplate {
	return map[string]SubscribeTemplate{
		TemplateTaskPublished: {ID: "tmpl-published", Fields: map[string]string{"thing1": "title", "time2": "due_at"}},
		TemplateTaskReviewed:  {ID: "tmpl-reviewed", Fields: map[string]string{"thing1": "title", "number2": "score"}},
	}
}

func TestSubscriptionStatusListsAllKeys(t *testing.T) {
	repo := newFakeSubRepo()
	repo.grants[TemplateTaskPublished] = 2
	uc := NewSubscriptionUsecase(repo, &fakeSender{}, testTemplates(), "trial", nil)

	views, err := uc.Status(context.Background(), "g1")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(views) != 3 {
		t.Fatalf("want 3 views, got %d", len(views))
	}
	if views[0].TemplateKey != TemplateTaskPublished || views[0].Remaining != 2 || !views[0].Configured || views[0].TemplateID != "tmpl-published" {
		t.Fatalf("published view = %+v", views[0])
	}
	if views[2].TemplateKey != TemplateTaskReturned || views[2].Configured {
		t.Fatalf("未配置模板应标记 configured=false: %+v", views[2])
	}
}

func TestSubscriptionGrantOnlyAcceptCounts(t *testing.T) {
	repo := newFakeSubRepo()
	uc := NewSubscriptionUsecase(repo, &fakeSender{}, testTemplates(), "", nil)

	if _, err := uc.Grant(context.Background(), "g1", []GrantInput{
		{TemplateKey: TemplateTaskPublished, State: "accept"},
		{TemplateKey: TemplateTaskReviewed, State: "reject"},
	}); err != nil {
		t.Fatalf("Grant: %v", err)
	}
	if repo.grants[TemplateTaskPublished] != 1 {
		t.Fatalf("accept 应 +1，got %d", repo.grants[TemplateTaskPublished])
	}
	if repo.grants[TemplateTaskReviewed] != 0 {
		t.Fatalf("reject 不应增加额度，got %d", repo.grants[TemplateTaskReviewed])
	}
	if _, err := uc.Grant(context.Background(), "g1", []GrantInput{{TemplateKey: "task_unknown"}}); !errors.Is(err, ErrInvalidTemplateKey) {
		t.Fatalf("未知模板键应报错，got %v", err)
	}
}

func TestRenderTemplateDataTruncates(t *testing.T) {
	fields := map[string]string{
		"thing1":  "title",
		"phrase2": "student",
		"time3":   "missing",
		"thing4":  "empty",
	}
	payload := map[string]string{
		"title":   strings.Repeat("物", 30),
		"student": "张三丰丰丰",
		"empty":   "   ",
	}
	got := renderTemplateData(fields, payload)
	if len(got) != 2 {
		t.Fatalf("want 2 fields, got %+v", got)
	}
	if n := len([]rune(got["thing1"])); n != 20 {
		t.Fatalf("thing 字段应截断到 20 个字，got %d: %q", n, got["thing1"])
	}
	if n := len([]rune(got["phrase2"])); n != 5 {
		t.Fatalf("phrase 字段应截断到 5 个字，got %d: %q", n, got["phrase2"])
	}
	if out := renderTemplateData(fields, map[string]string{}); out != nil {
		t.Fatalf("空 payload 应返回 nil，got %+v", out)
	}
}

func TestDeliverOnceDisabledWithoutSender(t *testing.T) {
	repo := newFakeSubRepo()
	uc := NewSubscriptionUsecase(repo, nil, testTemplates(), "", nil)
	if uc.Enabled() {
		t.Fatal("无 sender 时不应 enabled")
	}
	if n, err := uc.DeliverOnce(context.Background()); err != nil || n != 0 {
		t.Fatalf("DeliverOnce = %d/%v，期望 0/nil", n, err)
	}
}

func TestDeliverOnceSuccessAndPermanentFailure(t *testing.T) {
	repo := newFakeSubRepo()
	repo.delivers = []Delivery{
		{ID: "d1", OpenID: "openid-1", TemplateKey: TemplateTaskPublished, Page: "p", Payload: map[string]string{"title": "物理作业", "due_at": "2026年11月30日 18:00"}},
		{ID: "d2", OpenID: "openid-2", TemplateKey: TemplateTaskReviewed, Payload: map[string]string{"title": "物理作业", "score": "88"}},
	}
	sender := &fakeSender{fn: func(msg SubscribeMessage) (bool, error) {
		if msg.OpenID == "openid-1" {
			return false, nil
		}
		return false, &permanentStub{}
	}}
	uc := NewSubscriptionUsecase(repo, sender, testTemplates(), "formal", nil)

	n, err := uc.DeliverOnce(context.Background())
	if err != nil || n != 2 {
		t.Fatalf("DeliverOnce = %d/%v", n, err)
	}
	// 第一条成功，第二条永久失败。
	if len(repo.sentIDs) != 1 || repo.sentIDs[0] != "d1" {
		t.Fatalf("sentIDs = %v", repo.sentIDs)
	}
	if len(repo.doneIDs) != 1 || repo.doneIDs[0] != "d2" || repo.doneStatus[0] != DeliveryFailed {
		t.Fatalf("doneIDs = %v/%v", repo.doneIDs, repo.doneStatus)
	}
	if len(sender.calls) != 2 || sender.calls[0].TemplateID != "tmpl-published" || sender.calls[0].MiniprogramState != "formal" {
		t.Fatalf("calls = %+v", sender.calls)
	}
	if sender.calls[0].Data["thing1"] != "物理作业" {
		t.Fatalf("data = %+v", sender.calls[0].Data)
	}
}

func TestDeliverOnceRetriesTransientFailure(t *testing.T) {
	repo := newFakeSubRepo()
	repo.delivers = []Delivery{
		{ID: "d1", OpenID: "openid-1", TemplateKey: TemplateTaskPublished, Payload: map[string]string{"title": "x"}, Attempts: 0},
	}
	uc := NewSubscriptionUsecase(repo, &fakeSender{fn: func(SubscribeMessage) (bool, error) {
		return true, errors.New("timeout")
	}}, testTemplates(), "", nil)
	if _, err := uc.DeliverOnce(context.Background()); err != nil {
		t.Fatalf("DeliverOnce: %v", err)
	}
	if len(repo.retriedIDs) != 1 || repo.retriedIDs[0] != "d1" {
		t.Fatalf("retriedIDs = %v", repo.retriedIDs)
	}

	// 达到最大尝试次数后置 failed。
	repo.retriedIDs = nil
	repo.delivers = []Delivery{
		{ID: "d2", OpenID: "openid-1", TemplateKey: TemplateTaskPublished, Payload: map[string]string{"title": "x"}, Attempts: 2},
	}
	if _, err := uc.DeliverOnce(context.Background()); err != nil {
		t.Fatalf("DeliverOnce: %v", err)
	}
	if len(repo.retriedIDs) != 0 || len(repo.doneIDs) != 1 || repo.doneStatus[0] != DeliveryFailed {
		t.Fatalf("retried=%v done=%v/%v", repo.retriedIDs, repo.doneIDs, repo.doneStatus)
	}
}

func TestDeliverSkipsMissingOpenID(t *testing.T) {
	repo := newFakeSubRepo()
	repo.delivers = []Delivery{{ID: "d1", TemplateKey: TemplateTaskPublished, Payload: map[string]string{"title": "x"}}}
	sender := &fakeSender{}
	uc := NewSubscriptionUsecase(repo, sender, testTemplates(), "", nil)
	if _, err := uc.DeliverOnce(context.Background()); err != nil {
		t.Fatalf("DeliverOnce: %v", err)
	}
	if len(sender.calls) != 0 {
		t.Fatalf("缺 openid 不应调用发送: %+v", sender.calls)
	}
	if len(repo.doneIDs) != 1 || repo.doneStatus[0] != DeliverySkipped {
		t.Fatalf("done = %v/%v", repo.doneIDs, repo.doneStatus)
	}
}

func TestBackoffCaps(t *testing.T) {
	if got := backoff(0); got != time.Minute {
		t.Fatalf("backoff(0) = %v", got)
	}
	if got := backoff(10); got != 10*time.Minute {
		t.Fatalf("backoff(10) = %v", got)
	}
	if got := backoff(-3); got != time.Minute {
		t.Fatalf("backoff(-3) = %v", got)
	}
}

func TestFieldLimit(t *testing.T) {
	cases := map[string]int{
		"thing1":            20,
		"phrase2":           5,
		"name3":             10,
		"character_string4": 32,
		"phone_number5":     17,
		"number6":           32,
		"amount7":           32,
		"date8":             32,
		"time9":             32,
		"something_else":    100,
	}
	for keyword, want := range cases {
		if got := fieldLimit(keyword); got != want {
			t.Fatalf("fieldLimit(%q) = %d, want %d", keyword, got, want)
		}
	}
}

func TestRunWorkerExitsWhenDisabled(t *testing.T) {
	uc := NewSubscriptionUsecase(newFakeSubRepo(), nil, testTemplates(), "", nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan struct{})
	go func() { uc.RunWorker(ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunWorker 在未启用时未立即退出")
	}
}

func TestRunWorkerDeliversThenStops(t *testing.T) {
	repo := newFakeSubRepo()
	repo.delivers = []Delivery{{
		ID: "d1", OpenID: "openid-1", TemplateKey: TemplateTaskPublished,
		Payload: map[string]string{"title": "物理作业", "student": "张三"},
	}}
	sender := &fakeSender{}
	uc := NewSubscriptionUsecase(repo, sender, testTemplates(), "", nil)
	uc.interval = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { uc.RunWorker(ctx); close(done) }()
	deadline := time.After(2 * time.Second)
	for len(repo.sentIDs) == 0 {
		select {
		case <-deadline:
			cancel()
			t.Fatal("worker 未发送投递")
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunWorker 取消后未退出")
	}
}

// permanentStub 让 fakeSender 的错误看起来像不可重试错误（biz 不依赖 pkg/wx）。
type permanentStub struct{}

func (e *permanentStub) Error() string { return "errcode=43101 user refuse" }
