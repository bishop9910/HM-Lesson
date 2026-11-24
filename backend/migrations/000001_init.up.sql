-- 000001_init：账号、学员、家长绑定、文件与运维基础表（MySQL 8.0）
-- 约定：id=char(36) UUIDv7；时间 UTC datetime(3)；枚举 text + CHECK；表名全小写。

CREATE TABLE staff (
  id char(36) NOT NULL,
  name varchar(64) NOT NULL,
  phone varchar(32) NOT NULL,
  password_hash varchar(255) NOT NULL,
  role varchar(16) NOT NULL DEFAULT 'owner',
  status varchar(16) NOT NULL DEFAULT 'active',
  created_by char(36) NULL,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  deleted_at datetime(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_staff_phone (phone),
  KEY idx_staff_role (role),
  CONSTRAINT ck_staff_role CHECK (role IN ('owner','assistant')),
  CONSTRAINT ck_staff_status CHECK (status IN ('active','disabled')),
  CONSTRAINT fk_staff_created_by FOREIGN KEY (created_by) REFERENCES staff (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE students (
  id char(36) NOT NULL,
  name varchar(64) NOT NULL,
  grade varchar(16) NOT NULL,
  school varchar(128) NOT NULL DEFAULT '',
  group_code varchar(32) NOT NULL,
  status varchar(16) NOT NULL DEFAULT 'active',
  tags json NULL,
  next_follow_up_at datetime(3) NULL,
  note varchar(1000) NOT NULL DEFAULT '',
  created_by char(36) NOT NULL,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  archived_at datetime(3) NULL,
  PRIMARY KEY (id),
  KEY idx_students_group_status (group_code, status),
  KEY idx_students_next_follow_up (next_follow_up_at),
  KEY idx_students_name (name),
  CONSTRAINT ck_students_group CHECK (group_code IN ('class_active','one_on_one','intent','visitor')),
  CONSTRAINT ck_students_status CHECK (status IN ('active','graduated','lost')),
  CONSTRAINT fk_students_created_by FOREIGN KEY (created_by) REFERENCES staff (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE guardians (
  id char(36) NOT NULL,
  openid varchar(64) NULL,
  unionid varchar(64) NULL,
  name varchar(64) NOT NULL DEFAULT '',
  phone varchar(32) NOT NULL,
  status varchar(16) NOT NULL DEFAULT 'pending',
  rejected_reason varchar(255) NULL,
  bound_at datetime(3) NULL,
  approved_by char(36) NULL,
  approved_at datetime(3) NULL,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_guardians_openid (openid),
  KEY idx_guardians_phone (phone),
  KEY idx_guardians_status (status),
  CONSTRAINT ck_guardians_status CHECK (status IN ('pending','active','rejected','disabled')),
  CONSTRAINT fk_guardians_approved_by FOREIGN KEY (approved_by) REFERENCES staff (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE student_guardians (
  id char(36) NOT NULL,
  student_id char(36) NOT NULL,
  guardian_id char(36) NOT NULL,
  relation varchar(16) NOT NULL DEFAULT 'parent',
  is_primary tinyint(1) NOT NULL DEFAULT 0,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_student_guardians_pair (student_id, guardian_id),
  KEY idx_student_guardians_guardian (guardian_id),
  CONSTRAINT fk_student_guardians_student FOREIGN KEY (student_id) REFERENCES students (id),
  CONSTRAINT fk_student_guardians_guardian FOREIGN KEY (guardian_id) REFERENCES guardians (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE binding_codes (
  id char(36) NOT NULL,
  code char(6) NOT NULL,
  student_id char(36) NOT NULL,
  created_by char(36) NOT NULL,
  expires_at datetime(3) NOT NULL,
  used_at datetime(3) NULL,
  used_by_guardian_id char(36) NULL,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_binding_codes_code (code),
  KEY idx_binding_codes_student (student_id),
  KEY idx_binding_codes_expires (expires_at),
  CONSTRAINT fk_binding_codes_student FOREIGN KEY (student_id) REFERENCES students (id),
  CONSTRAINT fk_binding_codes_created_by FOREIGN KEY (created_by) REFERENCES staff (id),
  CONSTRAINT fk_binding_codes_used_by FOREIGN KEY (used_by_guardian_id) REFERENCES guardians (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE import_batches (
  id char(36) NOT NULL,
  kind varchar(32) NOT NULL DEFAULT 'students',
  status varchar(16) NOT NULL DEFAULT 'preview',
  file_id char(36) NULL,
  total_rows int NOT NULL DEFAULT 0,
  created_rows int NOT NULL DEFAULT 0,
  skipped_rows int NOT NULL DEFAULT 0,
  error_count int NOT NULL DEFAULT 0,
  error_report json NULL,
  created_by char(36) NOT NULL,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  committed_at datetime(3) NULL,
  rolled_back_at datetime(3) NULL,
  PRIMARY KEY (id),
  KEY idx_import_batches_created_by (created_by, created_at),
  CONSTRAINT ck_import_batches_status CHECK (status IN ('preview','committed','failed','rolled_back')),
  CONSTRAINT fk_import_batches_created_by FOREIGN KEY (created_by) REFERENCES staff (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE files (
  id char(36) NOT NULL,
  provider varchar(16) NOT NULL DEFAULT 'cos',
  object_key varchar(512) NOT NULL,
  file_id varchar(512) NULL,
  original_name varchar(255) NOT NULL,
  content_type varchar(128) NOT NULL,
  size_bytes bigint NOT NULL,
  sha256 char(64) NULL,
  category varchar(32) NOT NULL DEFAULT 'attachment',
  uploaded_by char(36) NOT NULL,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  deleted_at datetime(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_files_object_key (object_key),
  KEY idx_files_uploaded_by (uploaded_by, created_at),
  KEY idx_files_category (category),
  CONSTRAINT ck_files_provider CHECK (provider IN ('cos','minio','local')),
  CONSTRAINT fk_files_uploaded_by FOREIGN KEY (uploaded_by) REFERENCES staff (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE download_logs (
  id char(36) NOT NULL,
  file_id char(36) NOT NULL,
  actor_type varchar(16) NOT NULL,
  actor_id char(36) NOT NULL,
  ip varchar(64) NOT NULL DEFAULT '',
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_download_logs_file (file_id, created_at),
  KEY idx_download_logs_actor (actor_type, actor_id),
  CONSTRAINT ck_download_logs_actor_type CHECK (actor_type IN ('staff','guardian'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE audit_logs (
  id char(36) NOT NULL,
  actor_type varchar(16) NOT NULL,
  actor_id char(36) NOT NULL,
  action varchar(64) NOT NULL,
  target_type varchar(32) NOT NULL,
  target_id char(36) NOT NULL,
  before_summary json NULL,
  after_summary json NULL,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_audit_logs_target (target_type, target_id),
  KEY idx_audit_logs_actor (actor_type, actor_id, created_at),
  CONSTRAINT ck_audit_logs_actor_type CHECK (actor_type IN ('staff','guardian','system'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE jobs (
  id char(36) NOT NULL,
  kind varchar(64) NOT NULL,
  payload json NULL,
  status varchar(16) NOT NULL DEFAULT 'pending',
  run_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  locked_by varchar(64) NULL,
  locked_until datetime(3) NULL,
  attempts int NOT NULL DEFAULT 0,
  max_attempts int NOT NULL DEFAULT 5,
  last_error varchar(1000) NULL,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_jobs_status_run_at (status, run_at),
  KEY idx_jobs_locked_until (locked_until),
  CONSTRAINT ck_jobs_status CHECK (status IN ('pending','running','done','failed','dead'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE outbox (
  id char(36) NOT NULL,
  aggregate_type varchar(32) NOT NULL,
  aggregate_id char(36) NOT NULL,
  event_type varchar(64) NOT NULL,
  payload json NOT NULL,
  status varchar(16) NOT NULL DEFAULT 'pending',
  run_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  attempts int NOT NULL DEFAULT 0,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_outbox_status_run_at (status, run_at),
  KEY idx_outbox_aggregate (aggregate_type, aggregate_id),
  CONSTRAINT ck_outbox_status CHECK (status IN ('pending','done','failed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE idempotency_keys (
  id char(36) NOT NULL,
  actor_id char(36) NOT NULL,
  idem_key varchar(128) NOT NULL,
  route varchar(128) NOT NULL,
  request_hash char(64) NOT NULL,
  response json NULL,
  expires_at datetime(3) NOT NULL,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_idempotency_actor_key (actor_id, idem_key),
  KEY idx_idempotency_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE token_denylist (
  id char(36) NOT NULL,
  token_hash char(64) NOT NULL,
  expires_at datetime(3) NOT NULL,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_token_denylist_hash (token_hash),
  KEY idx_token_denylist_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE settings (
  key_name varchar(64) NOT NULL,
  value_json json NOT NULL,
  updated_by char(36) NULL,
  updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (key_name),
  CONSTRAINT fk_settings_updated_by FOREIGN KEY (updated_by) REFERENCES staff (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
