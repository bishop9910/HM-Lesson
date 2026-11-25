DROP INDEX idx_guardians_status_created ON guardians;

ALTER TABLE guardians
  DROP COLUMN last_login_at;

ALTER TABLE guardians
  DROP COLUMN password_hash;
