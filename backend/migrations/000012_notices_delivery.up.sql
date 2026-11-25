-- 000012：M6 通知与回执补充字段
-- notices / notice_receipts 由 000002 建过（旧列集，从未有写入路径），这里只做增量补列，不重建。

ALTER TABLE notices
  ADD COLUMN audience_type varchar(16) NOT NULL DEFAULT 'all' AFTER audience,
  ADD COLUMN file_ids json NULL AFTER body;

ALTER TABLE notice_receipts
  ADD COLUMN renotified_at datetime(3) NULL AFTER reminded_at,
  ADD KEY idx_notice_receipts_pending (status, created_at);
