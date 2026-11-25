ALTER TABLE notice_receipts
  DROP KEY idx_notice_receipts_pending,
  DROP COLUMN renotified_at;

ALTER TABLE notices
  DROP COLUMN file_ids,
  DROP COLUMN audience_type;
