-- DanceMirror 数据库查询测试文件
-- 连接: mysql -u dmuser -pDance@2025 dancemirror

-- ============================================
-- 查看所有表
-- ============================================
SHOW TABLES;

-- ============================================
-- 查看用户表结构
-- ============================================
DESCRIBE users;

-- ============================================
-- 查看视频表结构
-- ============================================
DESCRIBE videos;

-- ============================================
-- 查询所有用户
-- ============================================
SELECT 
    id,
    email,
    firstName,
    lastName,
    createdAt
FROM users
ORDER BY createdAt DESC;

-- ============================================
-- 查询所有视频
-- ============================================
SELECT 
    v.id,
    v.title,
    v.description,
    v.fileName,
    v.fileSize,
    v.duration,
    v.createdAt,
    u.email as userEmail,
    u.firstName as userFirstName
FROM videos v
JOIN users u ON v.userId = u.id
ORDER BY v.createdAt DESC;

-- ============================================
-- 查询特定用户的视频
-- ============================================
SELECT 
    v.*,
    u.email
FROM videos v
JOIN users u ON v.userId = u.id
WHERE u.email = 'test@dancemirror.com';

-- ============================================
-- 统计信息
-- ============================================
SELECT 
    'Total Users' as Metric,
    COUNT(*) as Count
FROM users
UNION ALL
SELECT 
    'Total Videos' as Metric,
    COUNT(*) as Count
FROM videos
UNION ALL
SELECT 
    'Users with Videos' as Metric,
    COUNT(DISTINCT userId) as Count
FROM videos;

-- ============================================
-- 查看用户的视频数量
-- ============================================
SELECT 
    u.id,
    u.email,
    u.firstName,
    u.lastName,
    COUNT(v.id) as videoCount,
    SUM(v.fileSize) as totalSize
FROM users u
LEFT JOIN videos v ON u.id = v.userId
GROUP BY u.id, u.email, u.firstName, u.lastName
ORDER BY videoCount DESC;

-- ============================================
-- 查看最近上传的视频
-- ============================================
SELECT 
    v.id,
    v.title,
    v.fileName,
    v.duration,
    v.createdAt,
    u.email
FROM videos v
JOIN users u ON v.userId = u.id
ORDER BY v.createdAt DESC
LIMIT 10;

-- ============================================
-- 索引信息
-- ============================================
SHOW INDEX FROM users;
SHOW INDEX FROM videos;

-- ============================================
-- 外键信息
-- ============================================
SELECT 
    TABLE_NAME,
    COLUMN_NAME,
    CONSTRAINT_NAME,
    REFERENCED_TABLE_NAME,
    REFERENCED_COLUMN_NAME
FROM information_schema.KEY_COLUMN_USAGE
WHERE TABLE_SCHEMA = 'dancemirror' 
    AND REFERENCED_TABLE_NAME IS NOT NULL;

-- ============================================
-- 迁移历史
-- ============================================
SELECT * FROM schema_migrations;

-- ============================================
-- 清理测试数据 (谨慎使用!)
-- ============================================
-- DELETE FROM videos WHERE title LIKE '%测试%';
-- DELETE FROM users WHERE email LIKE '%test%';

