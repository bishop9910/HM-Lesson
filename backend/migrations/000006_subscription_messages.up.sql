-- 000006：微信订阅消息（一次性订阅授权 + 投递队列）
-- 一次性订阅：家长每 accept 一次，模板可发送 1 条；发送成功/尝试后扣减。
-- 站内通知（notifications）仍是保底通道，两张表互不替代。
-- 注意：MySQL 5.7 会解析但忽略 CHECK，template_key/status 的取值校验在 Go 侧（biz）再做一遍。

CREATE TABLE IF NOT EXISTS subscription_grants (
  id char(36) NOT NULL,
  guardian_id char(36) NOT NULL,
  template_key varchar(32) NOT NULL,
  remaining int NOT NULL DEFAULT 0,
  last_requested_at datetime(3) NULL,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_subscription_grants_guardian_template (guardian_id, template_key),
  CONSTRAINT fk_subscription_grants_guardian FOREIGN KEY (guardian_id) REFERENCES guardians (id),
  CONSTRAINT ck_subscription_grants_key CHECK (template_key IN ('task_published','task_reviewed','task_returned')),
  CONSTRAINT ck_subscription_grants_remaining CHECK (remaining >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS notification_deliveries (
  id char(36) NOT NULL,
  guardian_id char(36) NOT NULL,
  template_key varchar(32) NOT NULL,
  page varchar(128) NOT NULL DEFAULT '',
  payload json NULL,
  status varchar(16) NOT NULL DEFAULT 'pending',
  attempts int NOT NULL DEFAULT 0,
  next_attempt_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  last_error varchar(500) NOT NULL DEFAULT '',
  sent_at datetime(3) NULL,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_notification_deliveries_status_next (status, next_attempt_at),
  KEY idx_notification_deliveries_guardian (guardian_id, created_at),
  CONSTRAINT fk_notification_deliveries_guardian FOREIGN KEY (guardian_id) REFERENCES guardians (id),
  CONSTRAINT ck_notification_deliveries_key CHECK (template_key IN ('task_published','task_reviewed','task_returned')),
  CONSTRAINT ck_notification_deliveries_status CHECK (status IN ('pending','sending','sent','failed','skipped'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
