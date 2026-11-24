-- 000002_teaching：跟进、任务作业、预约、课时、班课、资料、通知（MySQL 8.0）

CREATE TABLE follow_ups (
  id char(36) NOT NULL,
  student_id char(36) NOT NULL,
  method varchar(16) NOT NULL,
  content varchar(2000) NOT NULL,
  concern varchar(1000) NOT NULL DEFAULT '',
  next_action varchar(1000) NOT NULL DEFAULT '',
  next_follow_up_at datetime(3) NULL,
  created_by char(36) NOT NULL,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_follow_ups_student (student_id, created_at),
  KEY idx_follow_ups_next (next_follow_up_at),
  CONSTRAINT ck_follow_ups_method CHECK (method IN ('wechat','phone','offline','other')),
  CONSTRAINT fk_follow_ups_student FOREIGN KEY (student_id) REFERENCES students (id),
  CONSTRAINT fk_follow_ups_created_by FOREIGN KEY (created_by) REFERENCES staff (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE tasks (
  id char(36) NOT NULL,
  title varchar(128) NOT NULL,
  description varchar(2000) NOT NULL DEFAULT '',
  type varchar(32) NOT NULL DEFAULT 'homework',
  status varchar(16) NOT NULL DEFAULT 'draft',
  due_at datetime(3) NULL,
  published_at datetime(3) NULL,
  created_by char(36) NOT NULL,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_tasks_status_due (status, due_at),
  KEY idx_tasks_created_by (created_by, created_at),
  CONSTRAINT ck_tasks_type CHECK (type IN ('homework','correction','trial','checkin')),
  CONSTRAINT ck_tasks_status CHECK (status IN ('draft','published','closed')),
  CONSTRAINT fk_tasks_created_by FOREIGN KEY (created_by) REFERENCES staff (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE task_assignments (
  id char(36) NOT NULL,
  task_id char(36) NOT NULL,
  student_id char(36) NOT NULL,
  status varchar(16) NOT NULL DEFAULT 'pending',
  submitted_at datetime(3) NULL,
  reviewed_at datetime(3) NULL,
  score decimal(5,2) NULL,
  comment varchar(2000) NOT NULL DEFAULT '',
  return_reason varchar(1000) NOT NULL DEFAULT '',
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_task_assignments_task_student (task_id, student_id),
  KEY idx_task_assignments_student (student_id, status),
  CONSTRAINT ck_task_assignments_status CHECK (status IN ('pending','submitted','reviewed','returned')),
  CONSTRAINT fk_task_assignments_task FOREIGN KEY (task_id) REFERENCES tasks (id),
  CONSTRAINT fk_task_assignments_student FOREIGN KEY (student_id) REFERENCES students (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE submissions (
  id char(36) NOT NULL,
  assignment_id char(36) NOT NULL,
  version int NOT NULL DEFAULT 1,
  text_content varchar(5000) NOT NULL DEFAULT '',
  file_ids json NULL,
  submitted_by char(36) NOT NULL,
  submitted_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  is_late tinyint(1) NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  UNIQUE KEY uq_submissions_assignment_version (assignment_id, version),
  KEY idx_submissions_submitted_at (submitted_at),
  CONSTRAINT fk_submissions_assignment FOREIGN KEY (assignment_id) REFERENCES task_assignments (id),
  CONSTRAINT fk_submissions_submitted_by FOREIGN KEY (submitted_by) REFERENCES guardians (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE slots (
  id char(36) NOT NULL,
  start_at datetime(3) NOT NULL,
  end_at datetime(3) NOT NULL,
  status varchar(16) NOT NULL DEFAULT 'open',
  note varchar(255) NOT NULL DEFAULT '',
  created_by char(36) NOT NULL,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_slots_start_at (start_at),
  KEY idx_slots_status_start (status, start_at),
  CONSTRAINT ck_slots_status CHECK (status IN ('open','locked','completed','canceled')),
  CONSTRAINT fk_slots_created_by FOREIGN KEY (created_by) REFERENCES staff (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE bookings (
  id char(36) NOT NULL,
  slot_id char(36) NOT NULL,
  student_id char(36) NOT NULL,
  guardian_id char(36) NOT NULL,
  request_note varchar(1000) NOT NULL DEFAULT '',
  status varchar(16) NOT NULL DEFAULT 'pending',
  review_note varchar(1000) NOT NULL DEFAULT '',
  reviewed_by char(36) NULL,
  reviewed_at datetime(3) NULL,
  completed_at datetime(3) NULL,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  -- 同一时段只允许一个“占位”预约：pending/approved/completed 视为占位，canceled/rejected 释放
  active_flag tinyint GENERATED ALWAYS AS (
    CASE WHEN status IN ('pending','approved','completed') THEN 1 ELSE NULL END
  ) STORED,
  PRIMARY KEY (id),
  UNIQUE KEY uq_bookings_slot_active (slot_id, active_flag),
  KEY idx_bookings_student (student_id, status),
  KEY idx_bookings_guardian (guardian_id),
  CONSTRAINT ck_bookings_status CHECK (status IN ('pending','approved','rejected','completed','canceled')),
  CONSTRAINT fk_bookings_slot FOREIGN KEY (slot_id) REFERENCES slots (id),
  CONSTRAINT fk_bookings_student FOREIGN KEY (student_id) REFERENCES students (id),
  CONSTRAINT fk_bookings_guardian FOREIGN KEY (guardian_id) REFERENCES guardians (id),
  CONSTRAINT fk_bookings_reviewed_by FOREIGN KEY (reviewed_by) REFERENCES staff (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE lesson_accounts (
  student_id char(36) NOT NULL,
  purchased_hours decimal(6,2) NOT NULL DEFAULT 0,
  balance_hours decimal(6,2) NOT NULL DEFAULT 0,
  expires_at date NULL,
  note varchar(255) NOT NULL DEFAULT '',
  version bigint NOT NULL DEFAULT 0,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (student_id),
  CONSTRAINT fk_lesson_accounts_student FOREIGN KEY (student_id) REFERENCES students (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE lesson_ledger (
  id char(36) NOT NULL,
  student_id char(36) NOT NULL,
  kind varchar(16) NOT NULL,
  delta_hours decimal(6,2) NOT NULL,
  balance_after decimal(6,2) NOT NULL,
  booking_id char(36) NULL,
  note varchar(255) NOT NULL DEFAULT '',
  created_by char(36) NOT NULL,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_lesson_ledger_student (student_id, created_at),
  KEY idx_lesson_ledger_booking (booking_id),
  CONSTRAINT ck_lesson_ledger_kind CHECK (kind IN ('purchase','consume','adjust','refund')),
  CONSTRAINT fk_lesson_ledger_student FOREIGN KEY (student_id) REFERENCES students (id),
  CONSTRAINT fk_lesson_ledger_booking FOREIGN KEY (booking_id) REFERENCES bookings (id),
  CONSTRAINT fk_lesson_ledger_created_by FOREIGN KEY (created_by) REFERENCES staff (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE classes (
  id char(36) NOT NULL,
  name varchar(128) NOT NULL,
  subject varchar(32) NOT NULL DEFAULT 'physics',
  start_date date NULL,
  end_date date NULL,
  schedule_note varchar(255) NOT NULL DEFAULT '',
  status varchar(16) NOT NULL DEFAULT 'active',
  created_by char(36) NOT NULL,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_classes_status (status),
  CONSTRAINT ck_classes_status CHECK (status IN ('active','archived')),
  CONSTRAINT fk_classes_created_by FOREIGN KEY (created_by) REFERENCES staff (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE class_members (
  id char(36) NOT NULL,
  class_id char(36) NOT NULL,
  student_id char(36) NOT NULL,
  joined_at date NOT NULL,
  left_at date NULL,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_class_members_pair (class_id, student_id),
  KEY idx_class_members_student (student_id),
  CONSTRAINT fk_class_members_class FOREIGN KEY (class_id) REFERENCES classes (id),
  CONSTRAINT fk_class_members_student FOREIGN KEY (student_id) REFERENCES students (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE class_sessions (
  id char(36) NOT NULL,
  class_id char(36) NOT NULL,
  session_date date NOT NULL,
  attendance json NULL,
  content varchar(1000) NOT NULL DEFAULT '',
  created_by char(36) NOT NULL,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_class_sessions_class_date (class_id, session_date),
  CONSTRAINT fk_class_sessions_class FOREIGN KEY (class_id) REFERENCES classes (id),
  CONSTRAINT fk_class_sessions_created_by FOREIGN KEY (created_by) REFERENCES staff (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE materials (
  id char(36) NOT NULL,
  title varchar(255) NOT NULL,
  category varchar(32) NOT NULL,
  file_id char(36) NOT NULL,
  allowed_groups json NOT NULL,
  published_at datetime(3) NULL,
  created_by char(36) NOT NULL,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_materials_category (category, published_at),
  CONSTRAINT ck_materials_category CHECK (category IN ('class_handout','one_on_one','exam','trial','enrollment')),
  CONSTRAINT fk_materials_file FOREIGN KEY (file_id) REFERENCES files (id),
  CONSTRAINT fk_materials_created_by FOREIGN KEY (created_by) REFERENCES staff (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE notices (
  id char(36) NOT NULL,
  title varchar(128) NOT NULL,
  body varchar(5000) NOT NULL,
  audience json NOT NULL,
  need_receipt tinyint(1) NOT NULL DEFAULT 0,
  status varchar(16) NOT NULL DEFAULT 'draft',
  sent_at datetime(3) NULL,
  created_by char(36) NOT NULL,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_notices_status_sent (status, sent_at),
  CONSTRAINT ck_notices_status CHECK (status IN ('draft','sent')),
  CONSTRAINT fk_notices_created_by FOREIGN KEY (created_by) REFERENCES staff (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE notice_receipts (
  id char(36) NOT NULL,
  notice_id char(36) NOT NULL,
  guardian_id char(36) NOT NULL,
  status varchar(16) NOT NULL DEFAULT 'pending',
  confirmed_at datetime(3) NULL,
  reminded_at datetime(3) NULL,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_notice_receipts_pair (notice_id, guardian_id),
  KEY idx_notice_receipts_status (status),
  CONSTRAINT ck_notice_receipts_status CHECK (status IN ('pending','confirmed')),
  CONSTRAINT fk_notice_receipts_notice FOREIGN KEY (notice_id) REFERENCES notices (id),
  CONSTRAINT fk_notice_receipts_guardian FOREIGN KEY (guardian_id) REFERENCES guardians (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
