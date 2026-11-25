-- 000011 回滚：删掉 M5 资料库四张表，并把 000002 形态的 materials 还原回去。
-- 注意：回滚会丢失资料数据（此时 M5 接口已下线）。

DROP TABLE IF EXISTS material_download_logs;
DROP TABLE IF EXISTS materials;
DROP TABLE IF EXISTS material_visibility;
DROP TABLE IF EXISTS material_categories;

CREATE TABLE IF NOT EXISTS materials (
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
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
