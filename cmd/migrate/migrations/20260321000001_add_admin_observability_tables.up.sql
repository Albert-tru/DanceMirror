ALTER TABLE users
ADD COLUMN nickname VARCHAR(100) DEFAULT '' AFTER lastName,
ADD COLUMN lastLoginAt TIMESTAMP NULL DEFAULT NULL AFTER nickname,
ADD COLUMN accountStatus VARCHAR(20) NOT NULL DEFAULT 'active' AFTER lastLoginAt,
ADD COLUMN role VARCHAR(20) NOT NULL DEFAULT 'user' AFTER accountStatus,
ADD INDEX idx_users_role (role),
ADD INDEX idx_users_accountStatus (accountStatus),
ADD INDEX idx_users_lastLoginAt (lastLoginAt);

CREATE TABLE IF NOT EXISTS upload_events (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    userId BIGINT NOT NULL,
    videoId INT NULL,
    status VARCHAR(20) NOT NULL,
    fileSize BIGINT NOT NULL DEFAULT 0,
    errorCode VARCHAR(64) DEFAULT '',
    requestId VARCHAR(64) DEFAULT '',
    createdAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_upload_events_userId (userId),
    INDEX idx_upload_events_videoId (videoId),
    INDEX idx_upload_events_status (status),
    INDEX idx_upload_events_createdAt (createdAt),
    INDEX idx_upload_events_requestId (requestId)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS ai_tasks (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    userId BIGINT NOT NULL,
    videoId INT NOT NULL,
    status VARCHAR(32) NOT NULL,
    durationMs BIGINT NOT NULL DEFAULT 0,
    errorReason TEXT,
    requestId VARCHAR(64) DEFAULT '',
    createdAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updatedAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_ai_tasks_userId (userId),
    INDEX idx_ai_tasks_videoId (videoId),
    INDEX idx_ai_tasks_status (status),
    INDEX idx_ai_tasks_createdAt (createdAt),
    INDEX idx_ai_tasks_requestId (requestId)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS api_error_logs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    requestId VARCHAR(64) DEFAULT '',
    userId BIGINT NULL,
    videoId INT NULL,
    method VARCHAR(10) NOT NULL,
    path VARCHAR(255) NOT NULL,
    statusCode INT NOT NULL,
    errorCode VARCHAR(64) DEFAULT '',
    message TEXT,
    createdAt TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_api_error_logs_requestId (requestId),
    INDEX idx_api_error_logs_userId (userId),
    INDEX idx_api_error_logs_videoId (videoId),
    INDEX idx_api_error_logs_statusCode (statusCode),
    INDEX idx_api_error_logs_createdAt (createdAt)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
