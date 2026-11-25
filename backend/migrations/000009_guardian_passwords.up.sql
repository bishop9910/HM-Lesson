-- 家长账号密码：家长不再只靠手机号进入门户。
-- 与 staff 一致使用 Argon2id 存哈希；空字符串表示“尚未设置密码”。
ALTER TABLE guardians
  ADD COLUMN password_hash varchar(255) NOT NULL DEFAULT '' AFTER phone;

ALTER TABLE guardians
  ADD COLUMN last_login_at datetime(3) NULL AFTER status;

-- 家长账号管理页按状态筛选、按创建时间排序（清理长期未绑定的申请）。
CREATE INDEX idx_guardians_status_created ON guardians (status, created_at);
