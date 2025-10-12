# 数据库学习资料目录

欢迎来到 DanceMirror 项目的数据库学习目录！这里包含了从零开始学习 MySQL 数据库的完整资料。

## 📚 学习文件说明

### 1. DATABASE-README.md（主学习文档）
**完整的数据库学习指南**，包含：
- 数据库基础概念（表、索引、外键、事务等）
- 本项目的数据库架构说明
- users 和 videos 表结构详解
- 常用 SQL 命令分类讲解
- 调试流程和 FAQ

**建议学习时长：** 2-3 小时通读，1-2 周实践

### 2. db-queries.sql（实战查询集）
**197 行可直接运行的 SQL 命令**，包含：
- 连接数据库的命令
- 查看表结构和索引
- 各种查询示例（SELECT、JOIN、GROUP BY）
- 性能分析（EXPLAIN、SHOW PROCESSLIST）
- 备份与恢复命令

**使用方法：**
```bash
# 连接到数据库
mysql -u dmuser -p -h 127.0.0.1 -P 3306 dancemirror

# 在 MySQL 命令行中，逐条复制粘贴 SQL 运行
# 或者直接执行整个文件
mysql -u dmuser -p dancemirror < db-queries.sql
```

## 🎯 学习路径（按优先级）

### 第一周：基础入门
1. **阅读** `DATABASE-README.md` 的前 5 章（基础概念 + 表结构）
2. **实践** 连接数据库并运行 `db-queries.sql` 中的查询命令
3. **对照** 项目代码 `service/user/store.go` 和 `service/video/store.go`
4. **练习** 自己写 3-5 个简单的 SELECT 查询

**学习目标：**
- 能独立连接数据库
- 理解 users 和 videos 表的关系
- 会写基本的 SELECT、WHERE、ORDER BY

### 第二周：进阶实践
1. **学习** JOIN 查询（INNER JOIN、LEFT JOIN）
2. **实践** 聚合函数（COUNT、SUM、GROUP BY）
3. **分析** 使用 EXPLAIN 查看查询计划
4. **理解** 索引的作用和使用场景

**学习目标：**
- 能写复杂的多表查询
- 理解索引如何加速查询
- 会使用 EXPLAIN 分析性能

### 第三周：深入理解
1. **学习** 事务和隔离级别
2. **实践** 在 Go 代码中使用事务
3. **掌握** 数据库备份与恢复
4. **优化** 找出并优化慢查询

**学习目标：**
- 理解事务的 ACID 特性
- 能在代码中正确使用事务
- 会备份和恢复数据库

## 🔗 相关项目文件

### 数据库相关代码
- `db/db.go` - 数据库连接配置
- `service/user/store.go` - 用户数据操作
- `service/video/store.go` - 视频数据操作
- `cmd/migrate/migrations/*.sql` - 数据库迁移文件

### 配置文件
- `.env` - 数据库连接信息（用户名、密码、地址）
- `config/env.go` - 环境配置读取

### 测试文件
- `api-test.http` - API 测试（包含数据库操作）
- `test_db_connection.go` - 数据库连接测试

## 💡 学习技巧

1. **边学边做**：不要只看文档，一定要自己动手运行 SQL
2. **对比学习**：对照 Go 代码和 SQL，理解如何在代码中操作数据库
3. **记录笔记**：把遇到的问题和解决方案记录下来
4. **多用 EXPLAIN**：养成分析查询性能的习惯
5. **定期备份**：练习时定期备份数据库，防止误操作

## 🎓 推荐资源

### 在线教程
- [SQLZoo](https://sqlzoo.net/) - 交互式 SQL 练习
- [LeetCode Database](https://leetcode.com/problemset/database/) - SQL 算法题
- [MySQL 官方文档](https://dev.mysql.com/doc/) - 权威参考

### 书籍推荐
- 《高性能 MySQL》- 性能优化必读
- 《MySQL 必知必会》- 快速入门
- 《SQL 基础教程》- 系统学习 SQL

### Go + MySQL
- [Go database/sql 官方文档](https://pkg.go.dev/database/sql)
- [sqlx 库](https://github.com/jmoiron/sqlx) - 更方便的查询
- [GORM](https://gorm.io/) - Go ORM 框架

## 📝 练习题

完成学习后，可以尝试这些练习：

1. **查询练习**：写 SQL 查询找出上传视频最多的前 10 个用户
2. **性能优化**：使用 EXPLAIN 分析一个慢查询并优化
3. **数据分析**：统计每个月新增的用户数和视频数
4. **Go 实现**：在 Go 代码中实现一个带事务的视频上传功能
5. **备份恢复**：备份数据库，删除一些数据，然后恢复

## ❓ 遇到问题？

1. 查看 `DATABASE-README.md` 第 8 章的 FAQ
2. 检查项目根目录的日志文件
3. 使用 `SHOW PROCESSLIST` 查看当前数据库状态
4. 参考项目代码中的实现

## 🚀 下一步

完成数据库学习后，建议继续学习：
- Redis 缓存（提升读取性能）
- 数据库连接池配置
- 读写分离架构
- 分库分表策略

祝学习顺利！💪
