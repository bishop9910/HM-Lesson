-- 回滚顺序与 up 相反。
ALTER TABLE guardians
  DROP INDEX idx_guardians_import_batch,
  DROP COLUMN import_batch_id;

ALTER TABLE students
  DROP INDEX idx_students_import_batch,
  DROP COLUMN import_batch_id;

ALTER TABLE import_batches
  DROP COLUMN mode,
  DROP COLUMN rows_json;
