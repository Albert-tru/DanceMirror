# DanceMirror 项目数据库面试问题详解

针对 DanceMirror 项目（Go + MySQL + 视频上传），整理了面试官最可能问的数据库相关问题，按照从基础到深入的顺序。
\home\dmin\go\DanceMirror\learning\database\DanceMirror-Interview-Questions.md
---

## 一、项目架构与设计（必问）

### 1. 你的数据库有哪些表？它们之间的关系是什么？

**要点答案：**
- `users` 表：存储用户信息（id, email, phone, password, firstName, lastName, createdAt）
- `videos` 表：存储视频元数据（id, userId, title, description, filePath, fileName, fileSize, duration, thumbnail, createdAt, updatedAt）
- 关系：`videos.userId` 外键关联 `users.id`，一对多关系（一个用户可以上传多个视频）
- 外键设置了 `ON DELETE CASCADE`，用户删除时自动删除其所有视频

### 2. 为什么视频文件不直接存数据库，而是存文件路径？

**要点答案：**
- **性能**：大文件（几十MB）存 DB 会严重拖慢查询、备份、复制
- **成本**：DB 存储比对象存储/文件系统贵得多
- **扩展性**：文件可以用 CDN 加速分发，DB 做不到
- **实践**：DB 只存元数据（路径、大小、时长），文件存 `uploads/` 目录或对象存储（S3/MinIO）
- **你的实现**：`filePath` 字段存 `/uploads/xxx.mp4`，前端通过 HTTP 直接访问

### 3. 用户密码如何存储？为什么这样做？

**要点答案：**
- 使用 bcrypt 哈希算法（Go 的 `golang.org/x/crypto/bcrypt`）
- 不存明文，防止数据库泄露导致账号被盗
- bcrypt 自带 salt，每次哈希结果不同，防止彩虹表攻击
- 登录时用 `bcrypt.CompareHashAndPassword` 对比

---

## 二、索引与性能优化（高频）

### 4. 你给哪些字段建了索引？为什么？

**要点答案：**
- `users.email`：唯一索引（UNIQUE），加速登录查询（WHERE email = ?）
- `users.phone`：项目用手机号登录，应该建索引
- `videos.userId`：外键索引（idx_userId），加速查询某用户的所有视频（WHERE userId = ?）
- 主键 `id` 自动是聚簇索引（InnoDB）

### 5. 如果视频列表查询很慢，你会怎么优化？

**要点答案：**
1. 用 EXPLAIN 分析查询计划，看是否用到索引
2. 检查 `ORDER BY createdAt DESC` 是否需要 filesort，考虑给 `createdAt` 建索引
3. 如果是联表查询（JOIN users），确保 JOIN 字段有索引
4. 限制返回列（不要 SELECT *），只取需要的字段
5. 分页优化：避免大 OFFSET（用 keyset pagination：WHERE id < ? ORDER BY id DESC LIMIT 20）
6. 考虑缓存热点数据（Redis）

### 6. EXPLAIN 结果中哪些字段最重要？

**要点答案：**
- `type`：访问类型，从好到坏：system > const > eq_ref > ref > range > index > ALL（ALL 是全表扫描，要避免）
- `key`：实际使用的索引，NULL 表示没用索引
- `rows`：预计扫描行数，越少越好
- `Extra`：
  - `Using index`：覆盖索引，很好
  - `Using filesort`：需要排序，可能慢
  - `Using temporary`：用临时表，需要优化

---

## 三、事务与并发（必考）

### 7. 你的项目哪里用到了事务？如何在 Go 中实现？

**要点答案：**

场景：视频上传时，需要同时写 `videos` 表和移动文件，要保证原子性

Go 实现：
```go
tx, err := db.BeginTx(ctx, nil)
if err != nil { return err }
defer func() {
    if p := recover(); p != nil {
        tx.Rollback()
        panic(p)
    } else if err != nil {
        tx.Rollback()
    } else {
        err = tx.Commit()
    }
}()
// 执行 SQL：tx.ExecContext(ctx, "INSERT INTO videos ...", args...)
```

关键点：用 context 控制超时，defer 中处理 Rollback/Commit

### 8. MySQL 的隔离级别是什么？你的项目用的哪个？

**要点答案：**
- MySQL 默认：REPEATABLE READ（可重复读）
- 四种隔离级别及问题：
  - READ UNCOMMITTED：脏读
  - READ COMMITTED：不可重复读
  - REPEATABLE READ：幻读（InnoDB 用 gap lock 解决）
  - SERIALIZABLE：性能最差，完全串行
- 你的项目用默认即可，除非有特殊需求

### 9. 如果遇到死锁，你会怎么排查和解决？

**要点答案：**

**排查：**
```sql
SHOW ENGINE INNODB STATUS\G
```
查看死锁日志

**原因：** 两个事务相互等待对方释放锁

**解决：**
- 统一加锁顺序（比如都按 id 升序加锁）
- 缩短事务时间，减少锁持有
- 降低隔离级别（如果业务允许）
- 代码中捕获死锁错误并重试（指数退避）

---

## 四、安全性（必问）

### 10. 如何防止 SQL 注入？

**要点答案：**
- 永远使用参数化查询（Prepared Statement）
- Go 中用 `?` 占位符：
  ```go
  db.Query("SELECT * FROM users WHERE email = ?", email)
  ```
- 不要拼接字符串：❌ `"SELECT * FROM users WHERE email = '" + email + "'"`
- ORM（如 GORM）会自动处理，但要注意 Raw SQL

### 11. 数据库密码如何管理？

**要点答案：**
- 不要硬编码在代码中
- 你的项目：放在 `.env` 文件中，通过 `config/env.go` 读取
- 生产环境：用环境变量、密钥管理服务（AWS Secrets Manager、HashiCorp Vault）
- `.env` 加入 `.gitignore`，不提交到仓库

---

## 五、连接池与性能（加分项）

### 12. 你如何配置数据库连接池？

**要点答案：**
```go
db.SetMaxOpenConns(25)      // 最大打开连接数（不超过 MySQL max_connections）
db.SetMaxIdleConns(25)      // 最大空闲连接数（减少频繁建立连接）
db.SetConnMaxLifetime(5 * time.Minute) // 连接最大生命周期（避免长连接被 MySQL 关闭）
```

根据并发量和 MySQL 配置调整（一般 10-100）

### 13. 如果数据库连接数耗尽，会发生什么？如何排查？

**要点答案：**

**现象：** 新请求会阻塞或超时（context deadline exceeded）

**排查：**
- 查看 MySQL：`SHOW PROCESSLIST;` 看有多少连接
- 检查代码：是否有忘记关闭的连接（`defer rows.Close()`）
- 监控：连接池使用率（db.Stats()）

**解决：** 增大连接池、优化慢查询、检查连接泄漏

---

## 六、迁移与运维（实际工作）

### 14. 你如何管理数据库 schema 变更（迁移）？

**要点答案：**
- 你的项目：用 SQL 文件版本化管理（`cmd/migrate/migrations/`）
- 命名规范：`20250105000001_create_users_table.up.sql`（时间戳+描述）
- 每个迁移有 `.up.sql`（升级）和 `.down.sql`（回滚）
- 工具推荐：`golang-migrate`, `goose`
- 原则：向前兼容、先加后删（避免停机）

### 15. 如何备份和恢复数据库？

**要点答案：**

**逻辑备份：**
```bash
mysqldump -u user -p dbname > backup.sql
```

**恢复：**
```bash
mysql -u user -p dbname < backup.sql
```

**物理备份：** Percona XtraBackup（生产环境，热备份）

**原则：** 定期备份 + 测试恢复流程

---

## 七、扩展性（架构思维）

### 16. 如果用户量增长到百万级，你会怎么优化数据库？

**要点答案：**

**读优化：**
- 主从复制（读写分离），读请求分散到从库
- Redis 缓存热点数据（用户信息、视频列表）

**写优化：**
- 异步写入（消息队列）
- 批量插入

**存储优化：**
- 分库分表（按 userId 哈希分片）
- 冷热数据分离（历史视频归档）

**文件存储：**
- 对象存储（S3/OSS）+ CDN

### 17. 视频表数据量很大时，如何优化分页查询？

**要点答案：**

**问题：** `LIMIT 1000000, 20` 需要跳过 100 万行，很慢

**方案 1：游标分页（keyset pagination）**
```sql
-- 第一页
SELECT * FROM videos ORDER BY id DESC LIMIT 20;

-- 第二页（假设上一页最小 id 是 980）
SELECT * FROM videos WHERE id < 980 ORDER BY id DESC LIMIT 20;
```

**方案 2：** 缓存翻页记录的 id 范围

**方案 3：** 业务上禁止跳转到很后面的页（只允许下一页）

---

## 八、实战调试（展示你的能力）

### 18. 如果线上突然报"too many connections"，你会怎么办？

**要点答案：**

**紧急处理：** 重启应用释放连接，或临时增大 MySQL max_connections

**排查：**
- `SHOW PROCESSLIST;` 看哪些连接在干什么
- 检查慢查询（是否有长时间占用连接的查询）
- 查看应用日志（是否有连接泄漏）

**根治：** 优化慢查询、增大连接池、检查代码中的连接关闭

---

## 面试准备建议

### 准备策略

1. **手写答案**：把上面的问题手写一遍答案（加深记忆）
2. **实践操作**：在本地 MySQL 中复现关键 SQL（EXPLAIN、事务、索引）
3. **准备故事**：准备 1-2 个"你遇到的数据库问题和如何解决"的案例
4. **画图准备**：能画出你的项目 ER 图（表关系图）

### 面试时展现

- ✅ **具体化**：不只是说"用了索引"，而是说"在 userId 上建了索引，因为查询视频列表时需要 WHERE userId = ?"
- ✅ **结合项目**："我们用 bcrypt 存密码，因为..."
- ✅ **体现思考**："如果用户量大，我会考虑读写分离/分库分表..."
- ✅ **承认不足**：遇到不会的，可以说"这块我还没深入实践，但我知道可以通过...方向去学习"

---

## 快速复习清单

面试前 30 分钟快速过一遍：

- [ ] 两张表的字段和关系（users、videos）
- [ ] 为什么不把视频文件存 DB
- [ ] bcrypt 密码加密原理
- [ ] 建了哪些索引、为什么
- [ ] EXPLAIN 的 type、key、rows、Extra 含义
- [ ] 事务的 Go 代码实现（BeginTx、defer、Rollback/Commit）
- [ ] MySQL 四种隔离级别
- [ ] 如何防止 SQL 注入（参数化查询）
- [ ] 连接池三个参数（MaxOpenConns、MaxIdleConns、ConnMaxLifetime）
- [ ] 数据库迁移如何管理（SQL 文件版本化）
- [ ] 大数据量分页优化（keyset pagination）
- [ ] 读写分离、分库分表的思路

---

**祝你面试成功！🚀**

如需针对某个问题深入讲解或模拟面试，请随时提问！
