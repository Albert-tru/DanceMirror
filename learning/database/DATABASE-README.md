# DanceMirror — 数据库学习指南

这个文档面向还没学过数据库的你，汇总了在本项目中使用到的数据库相关知识，并把常用的 SQL 调试命令整理成可直接运行或参考的示例。

**目标：**
- 让你理解本项目如何使用 MySQL
- 给出常见的 SQL 调试命令（带解释和示例）
- 标注项目中相关的代码文件，方便你实操学习

**目录**
- 基础概念回顾
- 项目里使用到的数据库组件与文件映射
- 数据库连接与 DSN（代码级别）
- 数据库迁移（migrations）
- 表结构（users、videos）详解
- 常用 SQL 调试命令（带示例）
- 日常开发/调试工作流小结
- 安全与备份要点


## 1. 基础概念回顾（简明）
- **数据库（Database）**: 存储结构化数据的地方，本项目使用 MySQL。
- **表（Table）**: 类似于电子表格的一张表，比如 `users`, `videos`。
- **行（Row）/列（Column）**: 行是记录，列是字段（name, email, createdAt）。
- **主键（PRIMARY KEY）**: 唯一标识一行，通常是 `id` 自增。
- **索引（INDEX）**: 用来加速查询（WHERE / JOIN），例如按 `userId` 建索引。
- **外键（FOREIGN KEY）**: 表与表之间的引用关系（如 videos.userId -> users.id）。
- **迁移（Migration）**: 用 SQL 文件或工具来创建/修改表结构，保持 schema 可重放。
- **事务（Transaction）**: 把多个写操作组合成一个原子操作（全部成功或全部回滚）。


## 2. 项目里使用到的数据库组件与文件映射
**代码层（Go）：**
- `db/db.go` — 负责创建 MySQL 连接（见下节 DSN）
- `service/user/store.go` — 与 `users` 表相关的 CRUD 操作
- `service/video/store.go` — 与 `videos` 表相关的 CRUD 操作
- `cmd/migrate/migrations/*.sql` — 存放迁移脚本（建表/回滚 SQL）
- `cmd/migrate/main.go` — 运行迁移的程序

**SQL 辅助文件：**
- `db-queries.sql` — 常用查询示例（已整理）


## 3. 数据库连接与 DSN（代码级别）
在 `db/db.go` 中看到的连接逻辑：
- 使用标准库 `database/sql` + MySQL 驱动 `github.com/go-sql-driver/mysql`。

**示例 DSN 格式（项目中使用）：**
```
user:password@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True&loc=Local
```

**作用：**
- `charset=utf8mb4`：支持 emoji 等 4 字节字符
- `parseTime=True`：把 MySQL 的 DATETIME/TIMESTAMP 自动解析为 Go 的 time.Time
- `loc=Local`：使用本地时区解析时间

在本地调试通常会把这些配置放在 `config` 文件中（本项目的 `config/env.go`），包含 `DBUser`, `DBPassword`, `DBAddress`, `DBName` 等。


## 4. 数据库迁移（migrations）
项目中已经包含迁移脚本：
- `20250105000001_create_users_table.up.sql` — 创建 users 表
- `20250105000002_create_videos_table.up.sql` — 创建 videos 表
- 以及对应的 `.down.sql` 用于回滚

**迁移原则：**
- 所有结构变更都写成 SQL 文件并版本化（按时间戳命名）
- 在部署/开发前先运行迁移，确保数据库 schema 一致

**如何运行迁移（示例）**
```bash
# 以命令行方式执行迁移
mysql -u dmuser -pDance@2025 dancemirror < cmd/migrate/migrations/20250105000001_create_users_table.up.sql
mysql -u dmuser -pDance@2025 dancemirror < cmd/migrate/migrations/20250105000002_create_videos_table.up.sql
```


## 5. 表结构（users、videos）详解

### users 表（关键字段）
- `id` INT AUTO_INCREMENT PRIMARY KEY
- `email` VARCHAR(255) UNIQUE
- `phone` VARCHAR(20) (项目中使用 phone 登录)
- `password` VARCHAR(255) — 存储哈希后的密码
- `firstName`, `lastName`
- `createdAt` TIMESTAMP DEFAULT CURRENT_TIMESTAMP
- 索引：`idx_email`（加速按 email 查询）

### videos 表（关键字段）
- `id` INT AUTO_INCREMENT PRIMARY KEY
- `userId` INT NOT NULL — 外键，引用 users(id)
- `title` VARCHAR(255)
- `description` TEXT
- `filePath` VARCHAR(500) — 服务端存储路径（如 `/uploads/...`）
- `fileName`, `fileSize`
- `duration` FLOAT — 视频时长（可空）
- `thumbnail` VARCHAR(500) — 预览图路径
- `createdAt` / `updatedAt`
- 索引：`idx_userId`，外键 ON DELETE CASCADE（用户删除则删除其视频）


## 6. 常用 SQL 调试命令（按用途分类，带示例）

### 连接到数据库（MySQL 命令行客户端）
```bash
mysql -u dmuser -p -h 127.0.0.1 -P 3306 dancemirror
# 系统会提示输入密码
root密码：MySQL666
```

### 查看数据库和表结构
```sql
--列出可用数据库
SHOW DATABASES;

--选择要使用的数据库：
USE your_database;

-- 查看当前数据库的表
SHOW TABLES;

-- 查看具体表的列定义
DESCRIBE users;
DESCRIBE videos;

-- 查看创建表的 SQL（更详细）
SHOW CREATE TABLE videos;

--退出 mysql> 命令提示窗口
EXIT;
```

### 查看/查询数据（常用 SELECT）
```sql
-- 查询所有用户（分页用 LIMIT）
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

-- 查询最近 10 个视频（带作者信息）
SELECT v.id, v.title, v.fileName, v.duration, v.createdAt, u.email 
FROM videos v 
JOIN users u ON v.userId = u.id 
ORDER BY v.createdAt DESC 
LIMIT 10;
```

### 聚合与统计
```sql
-- 统计用户数和视频数
SELECT 'Total Users' AS Metric, COUNT(*) AS Count FROM users;
SELECT 'Total Videos' AS Metric, COUNT(*) AS Count FROM videos;

-- 每个用户的视频数量
SELECT u.id, u.email, COUNT(v.id) AS videoCount, SUM(v.fileSize) AS totalSize 
FROM users u 
LEFT JOIN videos v ON u.id = v.userId 
GROUP BY u.id, u.email 
ORDER BY videoCount DESC;
```

### 索引/外键/状态检查
```sql
-- 查看某个表的索引
SHOW INDEX FROM videos;

-- 查看外键约束（从 information_schema）
SELECT TABLE_NAME, COLUMN_NAME, CONSTRAINT_NAME, 
       REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME
FROM information_schema.KEY_COLUMN_USAGE
WHERE TABLE_SCHEMA = 'dancemirror' 
  AND REFERENCED_TABLE_NAME IS NOT NULL;

-- 查看正在运行的查询（分析慢查询）
SHOW PROCESSLIST;
```

### 写/改/删（示例，谨慎使用）
```sql
-- 新增一条用户（示例，仅测试用）
INSERT INTO users (email, phone, password, firstName, lastName) 
VALUES ('test@dancemirror.com', '13900139000', '$2y$...', 'Test', 'User');

-- 新增视频记录（如果文件已上传至 uploads/）
INSERT INTO videos (userId, title, description, filePath, fileName, fileSize) 
VALUES (1, '测试视频', 'desc', '/uploads/1_test.mp4', '1_test.mp4', 3141592);

-- 更新视频信息
UPDATE videos SET title = '新标题' WHERE id = 1;

-- 删除视频
DELETE FROM videos WHERE id = 123;
```

### 导出与导入（备份/恢复）
```bash
# 导出整个数据库为 SQL 文件（备份）
mysqldump -u dmuser -p dancemirror > dancemirror-backup.sql

# 导入（恢复）
mysql -u dmuser -p dancemirror < dancemirror-backup.sql
```


## 7. 调试数据库时的实用步骤（按操作顺序）
1. 确认服务能连上数据库：看 `db/db.go` 的日志（`db.Ping()` 成功）
2. 用 `SHOW TABLES` / `DESCRIBE` 验证表结构
3. 用 `SELECT` 验证数据是否存在（定位问题是否来自 DB）
4. 如果 API 报错（例如 500），在 server 日志里查看 SQL 错误消息
5. 使用 `SHOW PROCESSLIST` 或 `SHOW ENGINE INNODB STATUS` 检查锁/事务阻塞
6. 对 schema 变更，先在本地备份，再运行迁移脚本


## 8. 常见问题与解法（FAQ）

**Q: 插入数据后查询不到，为什么？**
- A: 确认使用了正确的数据库（注意 host/port/dbname），以及代码是否在事务中但未提交。

**Q: 外键约束导致删除失败？**
- A: 检查是否存在依赖的子记录（外键）。videos 使用 `ON DELETE CASCADE`，用户删除会级联删除视频。

**Q: 查询慢 / 页面加载慢？**
- A: 检查是否缺少索引（WHERE、JOIN、ORDER BY 常用字段应建索引）；使用 `EXPLAIN SELECT ...` 来分析查询计划。


## 9. 推荐的学习顺序与资源
1. 学会基本的 SELECT/INSERT/UPDATE/DELETE
2. 理解索引、JOIN、GROUP BY、聚合函数
3. 理解事务（BEGIN/COMMIT/ROLLBACK）和隔离级别
4. 学习迁移工具（例如 goose、golang-migrate）来管理 schema
5. 推荐资源：
   - MySQL 官方文档
   - 《高性能 MySQL》
   - SQLZoo / LeetCode Database 题库
   - Tutorialspoint 等在线教程


---

**下一步学习建议：**
- 打开 `db-queries.sql` 文件，在 MySQL 客户端逐条运行这些查询
- 尝试修改查询条件，观察结果变化
- 使用 `EXPLAIN` 分析查询性能
- 在项目代码中找到对应的 Go 实现（`service/user/store.go` 和 `service/video/store.go`）
