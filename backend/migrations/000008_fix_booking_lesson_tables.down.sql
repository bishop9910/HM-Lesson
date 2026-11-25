-- 回滚 000008：恢复 000002_teaching 中的旧列集定义。
-- 注意：回滚后新接口会重新报 1054，仅用于把库恢复到 000007 之后的状态。

DROP TABLE IF EXISTS lesson_ledger;
DROP TABLE IF EXISTS lesson_accounts;
DROP TABLE IF EXISTS bookings;
DROP TABLE IF EXISTS slots;

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
  CONSTRAINT ck_slots_status CHECK (status IN ('open', 'locked', 'completed', 'canceled')),
  CONSTRAINT fk_slots_created_by FOREIGN KEY (created_by) REFERENCES staff (id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

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
  active_flag tinyint GENERATED ALWAYS AS (
    CASE WHEN status IN ('pending', 'approved', 'completed') THEN 1 ELSE NULL END
  ) STORED,
  PRIMARY KEY (id),
  UNIQUE KEY uq_bookings_slot_active (slot_id, active_flag),
  KEY idx_bookings_student (student_id, status),
  KEY idx_bookings_guardian (guardian_id),
  CONSTRAINT ck_bookings_status CHECK (status IN ('pending', 'approved', 'rejected', 'completed', 'canceled')),
  CONSTRAINT fk_bookings_slot FOREIGN KEY (slot_id) REFERENCES slots (id),
  CONSTRAINT fk_bookings_student FOREIGN KEY (student_id) REFERENCES students (id),
  CONSTRAINT fk_bookings_guardian FOREIGN KEY (guardian_id) REFERENCES guardians (id),
  CONSTRAINT fk_bookings_reviewed_by FOREIGN KEY (reviewed_by) REFERENCES staff (id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

CREATE TABLE lesson_accounts (
  student_id char(36) NOT NULL,
  purchased_hours decimal(6, 2) NOT NULL DEFAULT 0,
  balance_hours decimal(6, 2) NOT NULL DEFAULT 0,
  expires_at date NULL,
  note varchar(255) NOT NULL DEFAULT '',
  version bigint NOT NULL DEFAULT 0,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (student_id),
  CONSTRAINT fk_lesson_accounts_student FOREIGN KEY (student_id) REFERENCES students (id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

CREATE TABLE lesson_ledger (
  id char(36) NOT NULL,
  student_id char(36) NOT NULL,
  kind varchar(16) NOT NULL,
  delta_hours decimal(6, 2) NOT NULL,
  balance_after decimal(6, 2) NOT NULL,
  booking_id char(36) NULL,
  note varchar(255) NOT NULL DEFAULT '',
  created_by char(36) NOT NULL,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_lesson_ledger_student (student_id, created_at),
  KEY idx_lesson_ledger_booking (booking_id),
  CONSTRAINT ck_lesson_ledger_kind CHECK (kind IN ('purchase', 'consume', 'adjust', 'refund')),
  CONSTRAINT fk_lesson_ledger_student FOREIGN KEY (student_id) REFERENCES students (id),
  CONSTRAINT fk_lesson_ledger_booking FOREIGN KEY (booking_id) REFERENCES bookings (id),
  CONSTRAINT fk_lesson_ledger_created_by FOREIGN KEY (created_by) REFERENCES staff (id)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;
