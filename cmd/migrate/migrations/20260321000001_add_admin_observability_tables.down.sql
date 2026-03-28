DROP TABLE IF EXISTS api_error_logs;
DROP TABLE IF EXISTS ai_tasks;
DROP TABLE IF EXISTS upload_events;

ALTER TABLE users
DROP INDEX idx_users_lastLoginAt,
DROP INDEX idx_users_accountStatus,
DROP INDEX idx_users_role,
DROP COLUMN role,
DROP COLUMN accountStatus,
DROP COLUMN lastLoginAt,
DROP COLUMN nickname;
