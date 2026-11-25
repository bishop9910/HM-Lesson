-- 修复 000007 未生效的问题（2026-11-25 线上 1054 Unknown column）
--
-- 背景：000002_teaching 已经用「旧列集」建过 slots / bookings / lesson_accounts / lesson_ledger
-- （slots.created_by、bookings 无 cancel_reason/attendance/lesson_ledger_id、lesson_accounts 无
-- used_hours/status/valid_until、lesson_ledger 用 kind/booking_id）。000007 用的是
-- CREATE TABLE IF NOT EXISTS，于是整段被静默跳过，迁移版本却照样记为已执行，
-- 结果线上新接口全部报 Error 1054 Unknown column。
--
-- 安全性：这四张表在 000007 之前从没有任何写入路径（预约/课时的接口是本次才写的，
-- 且写操作同样因列不存在而失败），因此可以确认是空表，重建不丢数据。
-- 应用顺序：先删有外键指向别人的子表，再删父表。

DROP TABLE IF EXISTS lesson_ledger;
DROP TABLE IF EXISTS lesson_accounts;
DROP TABLE IF EXISTS bookings;
DROP TABLE IF EXISTS slots;

CREATE TABLE slots (
    id char(36) NOT NULL,
    staff_id char(36) NOT NULL,
    start_at datetime(3) NOT NULL,
    end_at datetime(3) NOT NULL,
    status varchar(16) NOT NULL DEFAULT 'open',
    note varchar(500) NOT NULL DEFAULT '',
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_slots_status_start (status, start_at),
    KEY idx_slots_staff_start (staff_id, start_at),
    CONSTRAINT fk_slots_staff FOREIGN KEY (staff_id) REFERENCES staff (id),
    CONSTRAINT ck_slots_status CHECK (status IN ('open', 'locked', 'completed', 'cancelled')),
    CONSTRAINT ck_slots_range CHECK (end_at > start_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

CREATE TABLE bookings (
    id char(36) NOT NULL,
    slot_id char(36) NOT NULL,
    student_id char(36) NOT NULL,
    guardian_id char(36) NOT NULL,
    status varchar(16) NOT NULL DEFAULT 'pending',
    request_note varchar(1000) NOT NULL DEFAULT '',
    review_note varchar(1000) NOT NULL DEFAULT '',
    cancel_reason varchar(500) NOT NULL DEFAULT '',
    attendance varchar(16) NULL,
    lesson_ledger_id char(36) NULL,
    submitted_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    reviewed_at datetime(3) NULL,
    reviewed_by char(36) NULL,
    completed_at datetime(3) NULL,
    active_flag tinyint GENERATED ALWAYS AS (CASE WHEN status IN ('pending', 'approved') THEN 1 ELSE NULL END) VIRTUAL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uq_bookings_active_slot (slot_id, active_flag),
    KEY idx_bookings_student (student_id, submitted_at),
    KEY idx_bookings_status (status, submitted_at),
    KEY idx_bookings_guardian (guardian_id, submitted_at),
    CONSTRAINT fk_bookings_slot FOREIGN KEY (slot_id) REFERENCES slots (id),
    CONSTRAINT fk_bookings_student FOREIGN KEY (student_id) REFERENCES students (id),
    CONSTRAINT fk_bookings_guardian FOREIGN KEY (guardian_id) REFERENCES guardians (id),
    CONSTRAINT fk_bookings_reviewer FOREIGN KEY (reviewed_by) REFERENCES staff (id),
    CONSTRAINT ck_bookings_status CHECK (status IN ('pending', 'approved', 'rejected', 'completed', 'cancelled', 'no_show')),
    CONSTRAINT ck_bookings_attendance CHECK (attendance IS NULL OR attendance IN ('attended', 'no_show'))
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

CREATE TABLE lesson_accounts (
    student_id char(36) NOT NULL,
    purchased_hours decimal(6, 2) NOT NULL DEFAULT 0.00,
    used_hours decimal(6, 2) NOT NULL DEFAULT 0.00,
    balance_hours decimal(6, 2) NOT NULL DEFAULT 0.00,
    status varchar(16) NOT NULL DEFAULT 'active',
    valid_until date NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (student_id),
    CONSTRAINT fk_lesson_accounts_student FOREIGN KEY (student_id) REFERENCES students (id),
    CONSTRAINT ck_lesson_accounts_status CHECK (status IN ('active', 'exhausted', 'expired')),
    CONSTRAINT ck_lesson_accounts_balance CHECK (balance_hours >= 0)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

CREATE TABLE lesson_ledger (
    id char(36) NOT NULL,
    student_id char(36) NOT NULL,
    type varchar(16) NOT NULL,
    delta_hours decimal(6, 2) NOT NULL,
    balance_after decimal(6, 2) NOT NULL,
    ref_type varchar(16) NULL,
    ref_id char(36) NULL,
    reason varchar(500) NOT NULL DEFAULT '',
    operator_id char(36) NOT NULL,
    occurred_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    consume_ref_id char(36) GENERATED ALWAYS AS (
        CASE WHEN type = 'consume' AND ref_type = 'booking' THEN ref_id ELSE NULL END
    ) VIRTUAL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_lesson_ledger_consume_booking (consume_ref_id),
    KEY idx_ledger_student_time (student_id, occurred_at),
    CONSTRAINT fk_lesson_ledger_student FOREIGN KEY (student_id) REFERENCES students (id),
    CONSTRAINT fk_lesson_ledger_operator FOREIGN KEY (operator_id) REFERENCES staff (id),
    CONSTRAINT ck_lesson_ledger_type CHECK (type IN ('purchase', 'consume', 'adjust', 'refund')),
    CONSTRAINT ck_lesson_ledger_ref CHECK (ref_type IS NULL OR ref_type IN ('booking', 'manual', 'order')),
    CONSTRAINT ck_lesson_ledger_delta CHECK (delta_hours <> 0),
    CONSTRAINT ck_lesson_ledger_balance CHECK (balance_after >= 0)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;
