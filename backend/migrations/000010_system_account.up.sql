-- 000010: 系统账号（内部测试/调试用）与学员删除支持。
-- staff.hidden=1 的账号不出现在成员列表、不计入工作台统计，也不允许被成员管理修改；
-- 它的密码来自服务端环境变量 HM_SYSTEM_PASSWORD（首次创建时写入），之后可在小程序里自助改密。

ALTER TABLE staff
  ADD COLUMN hidden tinyint(1) NOT NULL DEFAULT 0 AFTER status,
  ADD KEY idx_staff_hidden (hidden);
