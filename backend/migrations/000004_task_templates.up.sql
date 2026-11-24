-- 任务模板：复用标题、说明、类型与默认截止偏移。
CREATE TABLE IF NOT EXISTS task_templates (
  id CHAR(36) NOT NULL,
  title VARCHAR(128) NOT NULL,
  description TEXT NOT NULL,
  type VARCHAR(20) NOT NULL,
  due_offset_hours INT NOT NULL DEFAULT 24,
  created_by CHAR(36) NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_task_templates_created_by (created_by)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
