-- 000005：家长站内通知 + 教师批阅图片
-- 通知只做站内弹窗/列表，不依赖微信订阅消息模板；后续接订阅消息可用同一批事件。

ALTER TABLE task_assignments
  ADD COLUMN review_file_ids json NULL AFTER return_reason;

CREATE TABLE IF NOT EXISTS notifications (
  id char(36) NOT NULL,
  guardian_id char(36) NOT NULL,
  student_id char(36) NULL,
  kind varchar(32) NOT NULL,
  title varchar(128) NOT NULL,
  body varchar(500) NOT NULL DEFAULT '',
  ref_type varchar(32) NOT NULL,
  ref_id char(36) NOT NULL,
  read_at datetime(3) NULL,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_notifications_guardian_read (guardian_id, read_at, created_at),
  KEY idx_notifications_ref (ref_type, ref_id),
  CONSTRAINT fk_notifications_guardian FOREIGN KEY (guardian_id) REFERENCES guardians (id),
  CONSTRAINT fk_notifications_student FOREIGN KEY (student_id) REFERENCES students (id),
  CONSTRAINT ck_notifications_kind CHECK (kind IN ('task_published','task_reviewed','task_returned'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
