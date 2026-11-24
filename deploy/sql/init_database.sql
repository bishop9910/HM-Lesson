-- ============================================================
-- HM-Lesson：复用现有 MySQL 实例，删除模板库并新建业务库
-- 目标：MySQL 5.7.8+ / 8.0（云托管 MySQL / 本地 Docker 均适用）
-- 注意：utf8mb4_0900_ai_ci 是 MySQL 8.0 专属排序规则；
--       若实例是 5.7，会报 ERROR 1273，务必使用 utf8mb4_unicode_ci。
--
-- !!! 破坏性操作 !!!
-- 1. 执行前先确认 golang-073k 模板服务已停止，避免它继续连 golang_demo。
-- 2. 如 golang_demo 里有需要保留的数据，先备份：
--    mysqldump -h <host> -P <port> -u <user> -p \
--      --databases golang_demo > golang_demo_backup.sql
-- 3. 本脚本只删除 golang_demo；不会删除 hm_lesson（若已存在则保留）。
-- 4. 授权段落默认注释；按控制台里的 MYSQL_USERNAME 取消注释后执行。
-- ============================================================

SET NAMES utf8mb4;

-- 删除模板库（仅在确认无业务数据后执行）
DROP DATABASE IF EXISTS `golang_demo`;

-- 新建业务库；字符集必须 utf8mb4，排序规则用 5.7/8.0 都支持的 utf8mb4_unicode_ci
CREATE DATABASE IF NOT EXISTS `hm_lesson`
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_unicode_ci;

-- 复用实例上的现有账号，不新建用户。
-- 把 'hm'@'%' 换成云托管环境变量 MYSQL_USERNAME 对应的账号；
-- host 部分按控制台实际值填写（常见为 '%'，也可能是具体网段）。
-- 如果该账号已经能访问 hm_lesson，这一段可以跳过。
-- GRANT ALL PRIVILEGES ON `hm_lesson`.* TO 'hm'@'%';
-- FLUSH PRIVILEGES;

USE `hm_lesson`;

SELECT
  DATABASE() AS current_database,
  @@character_set_database AS charset_name,
  @@collation_database AS collation_name;

-- 表结构不在这里创建：后端启动时 data.Migrate 会自动执行
-- backend/migrations/*.up.sql，并写入 schema_migrations。
