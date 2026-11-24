-- 导入批次：保存预览行，回滚时按批次定位新建学员/家长。
ALTER TABLE import_batches
  ADD COLUMN rows_json MEDIUMTEXT NULL COMMENT '预览解析后的行 JSON',
  ADD COLUMN mode VARCHAR(16) NOT NULL DEFAULT 'create' COMMENT 'create | update';

ALTER TABLE students
  ADD COLUMN import_batch_id CHAR(36) NULL COMMENT '由哪个导入批次新建',
  ADD KEY idx_students_import_batch (import_batch_id);

ALTER TABLE guardians
  ADD COLUMN import_batch_id CHAR(36) NULL COMMENT '由哪个导入批次新建',
  ADD KEY idx_guardians_import_batch (import_batch_id);
