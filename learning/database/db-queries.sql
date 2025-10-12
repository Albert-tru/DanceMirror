-- DanceMirror 数据库学习用 SQL 查询示例
-- 作者：AI Assistant
-- 用途：学习和调试数据库

-- ============================================
-- 连接数据库（在终端运行）
-- ============================================
-- mysql -u dmuser -p -h 127.0.0.1 -P 3306 dancemirror
-- 输入密码：Dance@2025

-- ============================================
-- 查看数据库结构
-- ============================================

-- 查看所有表
SHOW TABLES;

-- 查看表结构
DESCRIBE users;
DESCRIBE videos;

-- 查看创建表的完整 SQL
SHOW CREATE TABLE videos;

-- ============================================
-- 基本查询（SELECT）
-- ============================================

-- 查询所有用户（限制50条）
SELECT id, email, firstName, lastName, createdAt 
FROM users 
ORDER BY createdAt DESC 
LIMIT 50;

-- 查询某个用户的所有视频
SELECT v.* 
FROM videos v 
JOIN users u ON v.userId = u.id 
WHERE u.email = 'test@dancemirror.com' 
ORDER BY v.createdAt DESC;

-- 查询最近10个视频（带作者信息）
SELECT v.id, v.title, v.fileName, v.duration, v.createdAt, u.email 
FROM videos v 
JOIN users u ON v.userId = u.id 
ORDER BY v.createdAt DESC 
LIMIT 10;

-- 根据手机号查询用户
SELECT id, email, phone, firstName, lastName 
FROM users 
WHERE phone = '13900139000';

-- ============================================
-- 聚合与统计
-- ============================================

-- 统计用户总数
SELECT 'Total Users' AS Metric, COUNT(*) AS Count FROM users;

-- 统计视频总数
SELECT 'Total Videos' AS Metric, COUNT(*) AS Count FROM videos;

-- 统计每个用户的视频数量和总大小
SELECT 
    u.id, 
    u.email, 
    u.firstName, 
    u.lastName, 
    COUNT(v.id) AS videoCount, 
    SUM(v.fileSize) AS totalSize
FROM users u 
LEFT JOIN videos v ON u.id = v.userId 
GROUP BY u.id, u.email, u.firstName, u.lastName
ORDER BY videoCount DESC;

-- 查找上传视频最多的用户
SELECT 
    u.email, 
    COUNT(v.id) AS videoCount
FROM users u 
JOIN videos v ON u.id = v.userId 
GROUP BY u.id, u.email
ORDER BY videoCount DESC
LIMIT 10;

-- ============================================
-- 索引和性能分析
-- ============================================

-- 查看 users 表的索引
SHOW INDEX FROM users;

-- 查看 videos 表的索引
SHOW INDEX FROM videos;

-- 查看外键约束
SELECT 
    TABLE_NAME, 
    COLUMN_NAME, 
    CONSTRAINT_NAME, 
    REFERENCED_TABLE_NAME, 
    REFERENCED_COLUMN_NAME
FROM information_schema.KEY_COLUMN_USAGE
WHERE TABLE_SCHEMA = 'dancemirror' 
  AND REFERENCED_TABLE_NAME IS NOT NULL;

-- 分析查询性能（使用 EXPLAIN）
EXPLAIN SELECT v.* 
FROM videos v 
JOIN users u ON v.userId = u.id 
WHERE u.phone = '13900139000' 
ORDER BY v.createdAt DESC;

-- 查看正在运行的查询
SHOW PROCESSLIST;

-- 查看 InnoDB 引擎状态
SHOW ENGINE INNODB STATUS\G

-- ============================================
-- 数据修改（谨慎使用！）
-- ============================================

-- 插入测试用户
-- INSERT INTO users (email, phone, password, firstName, lastName) 
-- VALUES ('test@dancemirror.com', '13900139000', '$2y$10$...', 'Test', 'User');

-- 插入测试视频
-- INSERT INTO videos (userId, title, description, filePath, fileName, fileSize) 
-- VALUES (1, '测试视频', '这是一个测试视频', '/uploads/test.mp4', 'test.mp4', 1024000);

-- 更新视频标题
-- UPDATE videos SET title = '新标题' WHERE id = 1;

-- 删除视频（会级联删除相关记录）
-- DELETE FROM videos WHERE id = 1;

-- ============================================
-- 调试常用查询
-- ============================================

-- 查看最近上传的视频
SELECT id, userId, title, fileName, fileSize, createdAt 
FROM videos 
ORDER BY createdAt DESC 
LIMIT 20;

-- 检查用户是否存在
SELECT id, email, phone FROM users WHERE phone = '13900139000';

-- 查看用户上传的所有视频文件路径
SELECT userId, filePath, fileName, fileSize 
FROM videos 
WHERE userId = 1
ORDER BY createdAt DESC;

-- 查找大文件（超过10MB）
SELECT id, title, fileName, fileSize, fileSize/1024/1024 AS sizeMB
FROM videos 
WHERE fileSize > 10485760
ORDER BY fileSize DESC;

-- ============================================
-- 备份与恢复（在终端运行）
-- ============================================

-- 导出整个数据库
-- mysqldump -u dmuser -p dancemirror > dancemirror-backup-$(date +%Y%m%d).sql

-- 导入数据库
-- mysql -u dmuser -p dancemirror < dancemirror-backup.sql

-- 只导出表结构（不含数据）
-- mysqldump -u dmuser -p --no-data dancemirror > dancemirror-schema.sql

-- ============================================
-- 清理测试数据（危险操作！）
-- ============================================

-- 删除测试视频
-- DELETE FROM videos WHERE title LIKE '%测试%';

-- 删除测试用户
-- DELETE FROM users WHERE email LIKE '%test%';

-- 清空表（保留结构）
-- TRUNCATE TABLE videos;

-- ============================================
-- 学习建议
-- ============================================
-- 1. 先运行查询类命令（SELECT、SHOW、DESCRIBE）熟悉数据
-- 2. 使用 EXPLAIN 分析查询性能
-- 3. 尝试修改 WHERE 条件观察结果变化
-- 4. 对照项目代码（service/user/store.go）理解 Go 如何调用 SQL
-- 5. 谨慎使用修改类命令（INSERT、UPDATE、DELETE）
