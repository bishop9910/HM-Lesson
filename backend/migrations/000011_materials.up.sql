-- M5 资料库：分类 + 分组可见矩阵 + 资料 + 下载日志。
--
-- 背景：000002_teaching.up.sql 里已经有一张早期形态的 `materials`
-- （单文件、category 枚举、allowed_groups json、无状态机），该表从未有写入路径
-- （没有任何接口能建资料），因此可以安全重建。这里显式 DROP，避免
-- `CREATE TABLE IF NOT EXISTS` 被静默跳过导致列不匹配（000007 的事故，见 HANDOVER 第 12 条坑）。

DROP TABLE IF EXISTS materials;

CREATE TABLE IF NOT EXISTS material_categories (
  id char(36) NOT NULL,
  code varchar(32) NOT NULL,
  name varchar(64) NOT NULL,
  sort int NOT NULL DEFAULT 0,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  UNIQUE KEY uq_material_categories_code (code),
  CONSTRAINT ck_material_categories_code CHECK (code IN ('class_handout','one_on_one','exam','trial','enrollment','recording'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS material_visibility (
  category_id char(36) NOT NULL,
  group_code varchar(16) NOT NULL,
  visible tinyint(1) NOT NULL DEFAULT 0,
  PRIMARY KEY (category_id, group_code),
  CONSTRAINT ck_material_visibility_group CHECK (group_code IN ('class_active','one_on_one','intent','visitor')),
  CONSTRAINT fk_material_visibility_category FOREIGN KEY (category_id) REFERENCES material_categories (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS materials (
  id char(36) NOT NULL,
  category_id char(36) NOT NULL,
  title varchar(200) NOT NULL,
  description varchar(1000) NOT NULL DEFAULT '',
  grade varchar(16) NULL,
  status varchar(16) NOT NULL DEFAULT 'draft',
  file_ids json NULL,
  published_at datetime(3) NULL,
  created_by char(36) NOT NULL,
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_materials_category_status (category_id, status, published_at),
  CONSTRAINT ck_materials_status CHECK (status IN ('draft','published','archived')),
  CONSTRAINT ck_materials_grade CHECK (grade IS NULL OR grade IN ('g1','g2','g3','other')),
  CONSTRAINT fk_materials_category FOREIGN KEY (category_id) REFERENCES material_categories (id),
  CONSTRAINT fk_materials_created_by FOREIGN KEY (created_by) REFERENCES staff (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS material_download_logs (
  id char(36) NOT NULL,
  material_id char(36) NOT NULL,
  file_id varchar(512) NOT NULL DEFAULT '',
  guardian_id char(36) NOT NULL,
  student_id char(36) NULL,
  ip varchar(64) NOT NULL DEFAULT '',
  created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (id),
  KEY idx_material_downloads_material (material_id, created_at),
  KEY idx_material_downloads_guardian (guardian_id, created_at),
  CONSTRAINT fk_material_downloads_material FOREIGN KEY (material_id) REFERENCES materials (id),
  CONSTRAINT fk_material_downloads_guardian FOREIGN KEY (guardian_id) REFERENCES guardians (id),
  CONSTRAINT fk_material_downloads_student FOREIGN KEY (student_id) REFERENCES students (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
