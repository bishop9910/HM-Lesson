-- 000001_init down：按外键逆序删除
DROP TABLE IF EXISTS settings;
DROP TABLE IF EXISTS token_denylist;
DROP TABLE IF EXISTS idempotency_keys;
DROP TABLE IF EXISTS outbox;
DROP TABLE IF EXISTS jobs;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS download_logs;
DROP TABLE IF EXISTS files;
DROP TABLE IF EXISTS import_batches;
DROP TABLE IF EXISTS binding_codes;
DROP TABLE IF EXISTS student_guardians;
DROP TABLE IF EXISTS guardians;
DROP TABLE IF EXISTS students;
DROP TABLE IF EXISTS staff;
