package biz

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"
)

// 三个订阅消息模板键，与 notifications.kind 保持一致，便于站内信与微信消息同源。
const (
	TemplateTaskPublished = "task_published"
	TemplateTaskReviewed  = "task_reviewed"
	TemplateTaskReturned  = "task_returned"
)

// MaxGrantRemaining 是单模板可累计的一次性订阅额度上限，防止客户端反复上报 accept 无限堆积。
const MaxGrantRemaining = 10

// DeliveryStatus* 与 notification_deliveries.status 的取值一致（MySQL 5.7 忽略 CHECK，Go 侧校验）。
const (
	DeliveryPending = "pending"
	DeliverySending = "sending"
	DeliverySent    = "sent"
	DeliveryFailed  = "failed"
	DeliverySkipped = "skipped"
)

var (
	ErrInvalidTemplateKey = errors.New("invalid template key")
	ErrSubscriptionOff    = errors.New("subscription messages disabled")
)

// TemplateKeys 是受支持的模板键；顺序用于对外展示。
var TemplateKeys = []string{TemplateTaskPublished, TemplateTaskReviewed, TemplateTaskReturned}

// IsTemplateKey 判断模板键是否受支持。
func IsTemplateKey(key string) bool {
	switch key {
	case TemplateTaskPublished, TemplateTaskReviewed, TemplateTaskReturned:
		return true
	default:
		return false
	}
}

// SubscribeTemplate 是一个已配置的订阅消息模板：微信模板 ID + 关键词到内容 token 的映射。
type SubscribeTemplate struct {
	ID     string
	Fields map[string]string
}

// SubscriptionGrant 是家长对某模板的一次性订阅授权余额。
type SubscriptionGrant struct {
	TemplateKey string
	Remaining   int
	UpdatedAt   time.Time
}

// Delivery 是一条待发送的微信订阅消息；Payload 是事件发生时抓取的文案 token。
type Delivery struct {
	ID          string
	GuardianID  string
	OpenID      string
	TemplateKey string
	Page        string
	Payload     map[string]string
	Attempts    int
}

// SubscriptionRepo 由 data 层实现。
type SubscriptionRepo interface {
	ListGrants(ctx context.Context, guardianID string) ([]SubscriptionGrant, error)
	// AddGrant 在授权额度上增加 delta（上限 max），返回新的剩余额度。
	AddGrant(ctx context.Context, guardianID, templateKey string, delta, max int) (int, error)
	// ClaimDeliveries 取一批到期待发送的投递并标记为 sending。
	ClaimDeliveries(ctx context.Context, limit int) ([]Delivery, error)
	MarkDeliverySent(ctx context.Context, id string) error
	MarkDeliveryRetry(ctx context.Context, id, reason string, next time.Time) error
	MarkDeliveryDone(ctx context.Context, id, status, reason string) error
	// ResetStaleDeliveries 把卡在 sending 超过阈值的投递放回 pending（实例重启/发送中断）。
	ResetStaleDeliveries(ctx context.Context, before time.Time) error
}

// SubscribeSender 是微信订阅消息发送端口；retryable=false 表示重试无意义。
// nil sender 表示未配置，整个功能 no-op。
type SubscribeSender interface {
	SendSubscribe(ctx context.Context, msg SubscribeMessage) (retryable bool, err error)
}

// SubscribeMessage 是待发送消息的最小内容。
type SubscribeMessage struct {
	OpenID           string
	TemplateID       string
	Page             string
	MiniprogramState string
	Data             map[string]string
}

// SubscriptionView 是返回给家长端的模板状态。
type SubscriptionView struct {
	TemplateKey string `json:"template_key"`
	TemplateID  string `json:"template_id"`
	Remaining   int    `json:"remaining"`
	Configured  bool   `json:"configured"`
}

// GrantInput 是家长端回传的单次订阅结果；只有 accept 会增加额度。
type GrantInput struct {
	TemplateKey string
	State       string
}

// SubscriptionUsecase 负责一次性订阅额度与投递队列。
type SubscriptionUsecase struct {
	repo      SubscriptionRepo
	sender    SubscribeSender
	templates map[string]SubscribeTemplate
	state     string
	logger    *slog.Logger
	now       func() time.Time

	interval    time.Duration
	batchSize   int
	maxAttempts int
}

// NewSubscriptionUsecase 组装用例；state 是 miniprogram_state（formal/trial/developer）。
func NewSubscriptionUsecase(repo SubscriptionRepo, sender SubscribeSender, templates map[string]SubscribeTemplate, state string, logger *slog.Logger) *SubscriptionUsecase {
	if logger == nil {
		logger = slog.Default()
	}
	if state == "" {
		state = "formal"
	}
	return &SubscriptionUsecase{
		repo:        repo,
		sender:      sender,
		templates:   templates,
		state:       state,
		logger:      logger,
		now:         time.Now,
		interval:    30 * time.Second,
		batchSize:   20,
		maxAttempts: 3,
	}
}

// Enabled 表示当前部署是否真的能发订阅消息（sender 与至少一个模板都配置了）。
func (uc *SubscriptionUsecase) Enabled() bool {
	return uc != nil && uc.sender != nil && len(uc.templates) > 0
}

// Status 返回三个模板键的配置与剩余额度；未配置的模板也会出现，便于前端提示“老师未开启提醒”。
func (uc *SubscriptionUsecase) Status(ctx context.Context, guardianID string) ([]SubscriptionView, error) {
	if uc == nil || uc.repo == nil {
		return nil, ErrSubscriptionOff
	}
	grants, err := uc.repo.ListGrants(ctx, guardianID)
	if err != nil {
		return nil, err
	}
	remaining := make(map[string]int, len(grants))
	for _, g := range grants {
		remaining[g.TemplateKey] = g.Remaining
	}
	out := make([]SubscriptionView, 0, len(TemplateKeys))
	for _, key := range TemplateKeys {
		tpl, ok := uc.templates[key]
		out = append(out, SubscriptionView{
			TemplateKey: key,
			TemplateID:  tpl.ID,
			Remaining:   remaining[key],
			Configured:  ok && tpl.ID != "",
		})
	}
	return out, nil
}

// Grant 记录家长端的订阅结果：accept 增加一次额度，reject/ban/filter 只记时间不增额度。
// 微信一次性订阅是“accept 一次发一条”，所以每次点击授权都可以累加，但上限 MaxGrantRemaining。
func (uc *SubscriptionUsecase) Grant(ctx context.Context, guardianID string, inputs []GrantInput) ([]SubscriptionView, error) {
	if uc == nil || uc.repo == nil {
		return nil, ErrSubscriptionOff
	}
	for _, in := range inputs {
		if !IsTemplateKey(in.TemplateKey) {
			return nil, ErrInvalidTemplateKey
		}
		delta := 0
		if strings.EqualFold(strings.TrimSpace(in.State), "accept") {
			delta = 1
		}
		if _, err := uc.repo.AddGrant(ctx, guardianID, in.TemplateKey, delta, MaxGrantRemaining); err != nil {
			return nil, err
		}
	}
	return uc.Status(ctx, guardianID)
}

// RunWorker 是进程内发送循环：启动后立即跑一次，之后每 interval 一次，ctx 取消即退出。
func (uc *SubscriptionUsecase) RunWorker(ctx context.Context) {
	if !uc.Enabled() {
		uc.logger.Info("微信订阅消息未配置，投递 worker 不启动")
		return
	}
	uc.logger.Info("微信订阅消息投递 worker 已启动", "interval", uc.interval.String(), "templates", len(uc.templates))
	if n, err := uc.DeliverOnce(ctx); err != nil {
		uc.logger.Warn("订阅消息投递失败", "error", err)
	} else if n > 0 {
		uc.logger.Info("订阅消息投递完成", "count", n)
	}
	ticker := time.NewTicker(uc.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			uc.logger.Info("微信订阅消息投递 worker 已停止")
			return
		case <-ticker.C:
			n, err := uc.DeliverOnce(ctx)
			if err != nil {
				uc.logger.Warn("订阅消息投递失败", "error", err)
				continue
			}
			if n > 0 {
				uc.logger.Info("订阅消息投递完成", "count", n)
			}
		}
	}
}

// DeliverOnce 处理一批到期投递，返回实际处理的条数。
func (uc *SubscriptionUsecase) DeliverOnce(ctx context.Context) (int, error) {
	if !uc.Enabled() || uc.repo == nil {
		return 0, nil
	}
	if err := uc.repo.ResetStaleDeliveries(ctx, uc.now().Add(-10*time.Minute)); err != nil {
		return 0, err
	}
	items, err := uc.repo.ClaimDeliveries(ctx, uc.batchSize)
	if err != nil {
		return 0, err
	}
	for _, d := range items {
		uc.deliver(ctx, d)
	}
	return len(items), nil
}

func (uc *SubscriptionUsecase) deliver(ctx context.Context, d Delivery) {
	if strings.TrimSpace(d.OpenID) == "" {
		uc.finish(ctx, d, DeliverySkipped, "家长未采集到 openid")
		return
	}
	tpl, ok := uc.templates[d.TemplateKey]
	if !ok || tpl.ID == "" {
		uc.finish(ctx, d, DeliverySkipped, "模板未配置")
		return
	}
	data := renderTemplateData(tpl.Fields, d.Payload)
	if len(data) == 0 {
		uc.finish(ctx, d, DeliverySkipped, "模板字段无可用内容")
		return
	}
	retryable, err := uc.sender.SendSubscribe(ctx, SubscribeMessage{
		OpenID:           d.OpenID,
		TemplateID:       tpl.ID,
		Page:             d.Page,
		MiniprogramState: uc.state,
		Data:             data,
	})
	if err == nil {
		if err := uc.repo.MarkDeliverySent(ctx, d.ID); err != nil {
			uc.logger.Warn("标记订阅消息已发送失败", "delivery_id", d.ID, "error", err)
		}
		return
	}
	reason := truncateRunes(err.Error(), 500)
	if !retryable || d.Attempts+1 >= uc.maxAttempts {
		uc.finish(ctx, d, DeliveryFailed, reason)
		return
	}
	next := uc.now().Add(backoff(d.Attempts))
	if err := uc.repo.MarkDeliveryRetry(ctx, d.ID, reason, next); err != nil {
		uc.logger.Warn("标记订阅消息重试失败", "delivery_id", d.ID, "error", err)
	}
}

func (uc *SubscriptionUsecase) finish(ctx context.Context, d Delivery, status, reason string) {
	if err := uc.repo.MarkDeliveryDone(ctx, d.ID, status, truncateRunes(reason, 500)); err != nil {
		uc.logger.Warn("关闭订阅消息投递失败", "delivery_id", d.ID, "status", status, "error", err)
	}
	if status == DeliveryFailed {
		uc.logger.Warn("订阅消息发送失败", "delivery_id", d.ID, "template", d.TemplateKey, "reason", reason)
	}
}

// backoff 按尝试次数指数退避：1 分钟、2 分钟、4 分钟…最多 10 分钟。
func backoff(attempts int) time.Duration {
	if attempts < 0 {
		attempts = 0
	}
	d := time.Minute << uint(attempts)
	if d > 10*time.Minute {
		d = 10 * time.Minute
	}
	return d
}

// renderTemplateData 把模板关键词映射成微信要求的数据；空内容字段直接丢弃，
// 全部为空时返回空 map（调用方跳过发送，避免 47003 参数错误）。
func renderTemplateData(fields map[string]string, payload map[string]string) map[string]string {
	if len(fields) == 0 || len(payload) == 0 {
		return nil
	}
	out := make(map[string]string, len(fields))
	for keyword, token := range fields {
		value := strings.TrimSpace(payload[token])
		if value == "" {
			continue
		}
		out[keyword] = truncateRunes(value, fieldLimit(keyword))
	}
	return out
}

// fieldLimit 按微信模板关键词的类型前缀限制长度，避免因超长被拒（47003）。
func fieldLimit(keyword string) int {
	switch {
	case strings.HasPrefix(keyword, "thing"):
		return 20
	case strings.HasPrefix(keyword, "phrase"):
		return 5
	case strings.HasPrefix(keyword, "name"):
		return 10
	case strings.HasPrefix(keyword, "character_string"):
		return 32
	case strings.HasPrefix(keyword, "phone_number"):
		return 17
	case strings.HasPrefix(keyword, "number"), strings.HasPrefix(keyword, "amount"):
		return 32
	case strings.HasPrefix(keyword, "date"), strings.HasPrefix(keyword, "time"):
		return 32
	default:
		return 100
	}
}

func truncateRunes(s string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(s) <= limit {
		return s
	}
	return string([]rune(s)[:limit])
}
