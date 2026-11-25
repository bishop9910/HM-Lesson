package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"github.com/bishop9910/hm-lesson/backend/internal/biz"
)

// SubscriptionRepo 实现一次性订阅授权与微信消息投递队列。
type SubscriptionRepo struct {
	db *gorm.DB
}

func NewSubscriptionRepo(db *gorm.DB) *SubscriptionRepo { return &SubscriptionRepo{db: db} }

func (r *SubscriptionRepo) ListGrants(ctx context.Context, guardianID string) ([]biz.SubscriptionGrant, error) {
	var rows []struct {
		TemplateKey string
		Remaining   int
		UpdatedAt   time.Time
	}
	if err := r.db.WithContext(ctx).Raw(
		`SELECT template_key, remaining, updated_at FROM subscription_grants WHERE guardian_id = ?`,
		guardianID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]biz.SubscriptionGrant, 0, len(rows))
	for _, row := range rows {
		out = append(out, biz.SubscriptionGrant{
			TemplateKey: row.TemplateKey,
			Remaining:   row.Remaining,
			UpdatedAt:   row.UpdatedAt,
		})
	}
	return out, nil
}

// AddGrant 用 ON DUPLICATE KEY 累加额度；delta=0 只刷新 last_requested_at（家长拒绝时也留痕）。
func (r *SubscriptionRepo) AddGrant(ctx context.Context, guardianID, templateKey string, delta, max int) (int, error) {
	if delta < 0 {
		delta = 0
	}
	if max <= 0 {
		max = biz.MaxGrantRemaining
	}
	if delta > max {
		delta = max
	}
	var remaining int
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`
INSERT INTO subscription_grants (id, guardian_id, template_key, remaining, last_requested_at, created_at, updated_at)
VALUES (?, ?, ?, ?, NOW(3), NOW(3), NOW(3))
ON DUPLICATE KEY UPDATE
  remaining = LEAST(remaining + VALUES(remaining), ?),
  last_requested_at = NOW(3),
  updated_at = NOW(3)`, biz.NewID(), guardianID, templateKey, delta, max).Error; err != nil {
			return err
		}
		var grantRow struct{ Remaining int }
		if err := tx.Raw(
			`SELECT remaining FROM subscription_grants WHERE guardian_id = ? AND template_key = ?`,
			guardianID, templateKey).Scan(&grantRow).Error; err != nil {
			return err
		}
		remaining = grantRow.Remaining
		return nil
	})
	return remaining, err
}

// ClaimDeliveries 在事务内先锁待发送行再标记 sending，避免同一条被并发重复发送（MySQL 5.7 无 SKIP LOCKED）。
func (r *SubscriptionRepo) ClaimDeliveries(ctx context.Context, limit int) ([]biz.Delivery, error) {
	if limit <= 0 {
		limit = 20
	}
	var out []biz.Delivery
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var pending []struct{ ID string }
		if err := tx.Raw(`
SELECT id FROM notification_deliveries
WHERE status = 'pending' AND next_attempt_at <= NOW(3)
ORDER BY next_attempt_at ASC, created_at ASC
LIMIT ? FOR UPDATE`, limit).Scan(&pending).Error; err != nil {
			return err
		}
		if len(pending) == 0 {
			return nil
		}
		ids := make([]string, 0, len(pending))
		for _, row := range pending {
			ids = append(ids, row.ID)
		}
		if err := tx.Exec(
			`UPDATE notification_deliveries SET status='sending', attempts=attempts+1, updated_at=NOW(3) WHERE id IN (?)`,
			ids).Error; err != nil {
			return err
		}
		var rows []struct {
			ID          string
			GuardianID  string
			TemplateKey string
			Page        string
			Payload     []byte
			Attempts    int
			OpenID      sql.NullString
		}
		if err := tx.Raw(`
SELECT d.id, d.guardian_id, d.template_key, d.page, d.payload, d.attempts, g.openid
FROM notification_deliveries d
JOIN guardians g ON g.id = d.guardian_id
WHERE d.id IN (?)`, ids).Scan(&rows).Error; err != nil {
			return err
		}
		out = make([]biz.Delivery, 0, len(rows))
		for _, row := range rows {
			payload := map[string]string{}
			if len(row.Payload) > 0 {
				_ = json.Unmarshal(row.Payload, &payload)
			}
			out = append(out, biz.Delivery{
				ID:          row.ID,
				GuardianID:  row.GuardianID,
				OpenID:      row.OpenID.String,
				TemplateKey: row.TemplateKey,
				Page:        row.Page,
				Payload:     payload,
				Attempts:    row.Attempts - 1, // 本批次已自增，回退到本次尝试前的次数
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *SubscriptionRepo) MarkDeliverySent(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Exec(`
UPDATE notification_deliveries
SET status='sent', sent_at=NOW(3), last_error='', updated_at=NOW(3)
WHERE id = ?`, id).Error
}

func (r *SubscriptionRepo) MarkDeliveryRetry(ctx context.Context, id, reason string, next time.Time) error {
	return r.db.WithContext(ctx).Exec(`
UPDATE notification_deliveries
SET status='pending', last_error=?, next_attempt_at=?, updated_at=NOW(3)
WHERE id = ?`, reason, next, id).Error
}

func (r *SubscriptionRepo) MarkDeliveryDone(ctx context.Context, id, status, reason string) error {
	return r.db.WithContext(ctx).Exec(`
UPDATE notification_deliveries
SET status=?, last_error=?, updated_at=NOW(3)
WHERE id = ?`, status, reason, id).Error
}

func (r *SubscriptionRepo) ResetStaleDeliveries(ctx context.Context, before time.Time) error {
	return r.db.WithContext(ctx).Exec(`
UPDATE notification_deliveries
SET status='pending', updated_at=NOW(3)
WHERE status='sending' AND updated_at < ?`, before).Error
}

// enqueueDeliveries 在业务事务内为某学员的活跃家长入队微信订阅消息。
// 只有还存在剩余授权（remaining>0）才入队并立即扣减；没有授权时只保留站内通知，不报错。
// payload 是事件发生时的文案 token，worker 发送时再按模板关键词渲染。
func enqueueDeliveries(tx *gorm.DB, studentID, templateKey, page string, payload map[string]string) error {
	var guardianIDs []string
	if err := tx.Raw(`
SELECT sg.guardian_id
FROM student_guardians sg
JOIN guardians g ON g.id = sg.guardian_id AND g.status = 'active'
JOIN subscription_grants sgr
  ON sgr.guardian_id = sg.guardian_id AND sgr.template_key = ? AND sgr.remaining > 0
WHERE sg.student_id = ?`, templateKey, studentID).Scan(&guardianIDs).Error; err != nil {
		return err
	}
	if len(guardianIDs) == 0 {
		return nil
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	// MySQL 5.7 的 JSON 列不接受 binary 字符集的字节串，必须传 string。
	payloadText := string(payloadJSON)
	for _, guardianID := range guardianIDs {
		res := tx.Exec(`
UPDATE subscription_grants SET remaining = remaining - 1, updated_at = NOW(3)
WHERE guardian_id = ? AND template_key = ? AND remaining > 0`, guardianID, templateKey)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			continue
		}
		if err := tx.Exec(`
INSERT INTO notification_deliveries
  (id, guardian_id, template_key, page, payload, status, next_attempt_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, 'pending', NOW(3), NOW(3), NOW(3))`,
			biz.NewID(), guardianID, templateKey, page, payloadText).Error; err != nil {
			return err
		}
	}
	return nil
}
