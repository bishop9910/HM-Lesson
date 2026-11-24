DROP TABLE IF EXISTS notifications;

ALTER TABLE task_assignments
  DROP COLUMN review_file_ids;
