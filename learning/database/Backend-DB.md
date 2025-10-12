
# Go 后端开发者数据库知识与面试高频问题

本文为目标进入 Go 后端岗位的你，整理了必备的数据库知识体系，以及常见的面试高频问题和要点答案，帮助你有针对性地复习和自测。
\home\dmin\go\DanceMirror\learning\database\Go-Backend-DB-Interview.md
---

## 一、必须掌握的数据库知识点

### 1. 基础概念
- 关系型数据库基本术语：**数据库、表、行、列、主键、外键、索引**。  
- 常见数据类型：INT、VARCHAR、TEXT、DATETIME/TIMESTAMP、BLOB/JSON。  
- 基本操作：SELECT、INSERT、UPDATE、DELETE、事务（BEGIN/COMMIT/ROLLBACK）。

### 2. 查询与优化
- **JOIN**：INNER、LEFT、RIGHT、CROSS JOIN 的区别和使用场景。  
- **聚合与分组**：GROUP BY、HAVING、COUNT/SUM/AVG/MAX/MIN。  
- **索引原理**：B-tree 索引、唯一索引、覆盖索引、全文索引。  
- **EXPLAIN** 分析：重点关注 `type`、`key`、`rows`、`Extra` 等字段。  
- 避免全表扫描、减少 SELECT *、分页性能优化（offset vs keyset）。

### 3. 事务与并发
- **ACID** 原则及各隔离级别：READ UNCOMMITTED、READ COMMITTED、REPEATABLE READ、SERIALIZABLE。  
- 并发问题：脏读、不可重复读、幻读；锁机制（行锁、表锁、意向锁）。  
- 死锁定位与处理：`SHOW ENGINE INNODB STATUS`，合理加锁顺序与重试机制。

### 4. 架构与运维
- **读写分离**：主从复制拓扑、延迟一致性问题与解决方案。  
- **分库分表**：水平分片与垂直拆分策略。  
- **备份恢复**：mysqldump、xtrabackup、逻辑 vs 物理备份。  
- **权限与安全**：最小权限原则、SQL 注入防护（参数化查询）。  
- **监控与性能指标**：QPS、慢查询、InnoDB Buffer Pool 命中率、连接数。

### 5. Go 语言实战要点
- 使用 **database/sql**：连接、执行、查询、扫描。  
- **连接池配置**：`db.SetMaxOpenConns`、`db.SetMaxIdleConns`、`db.SetConnMaxLifetime`。  
- **Context** 管理：在查询和事务中使用 `context.Context` 控制超时与取消。  
- **预处理语句**（PreparedStatement）：防止 SQL 注入。  
- **Null 类型处理**：`sql.NullString`、`sql.NullInt64`、`sql.NullFloat64`。  
- 常用库：`sqlx`（增强扫描）、`gorm` / `ent`（ORM 优劣势）。  
- 迁移工具：`golang-migrate`, `goose` 等版本化管理 schema。

---

## 二、面试高频问题与要点

| 序号 | 问题 | 关键要点 |
|:---:|:---|:---|
|1|**解释事务的隔离级别及并发问题**|列出四种级别及对应的脏读、不可重复读、幻读。MySQL 默认 REPEATABLE READ。|
|2|**索引的原理与使用时机**|B-tree 索引原理、覆盖索引、唯一索引，写时成本；低基数字段不必索引。|
|3|**如何分析慢查询并优化？**|开启慢查询日志；用 EXPLAIN 分析执行计划；加索引或改写 SQL；分页优化。|
|4|**JOIN 性能如何优化？**|保证 ON 和 WHERE 的字段有索引；控制返回列；使用子查询或预聚合。|
|5|**如何在 Go 中安全使用事务？**|示例代码：`db.BeginTx(ctx, opts)`, `defer` 中处理 Commit/Rollback；捕获 panic。|
|6|**防止 SQL 注入的方法有哪些？**|使用参数化查询（`?` 占位符或 `$1, $2`）、ORM 的预处理功能，不拼接字符串。|
|7|**数据库连接池配置的最佳实践？**|根据 DB 最大连接数和 QPS 设置 max open/idle；合理设置连接回收。|
|8|**如何处理大文件（视频）存储？**|文件存对象存储（S3/MinIO），DB 存元数据和路径；前端使用 CDN/签名 URL。|
|9|**读写分离的常见问题及解决**|主从延迟导致读脏；可短线程读主或使用应用层缓存。|
|10|**Describe EXPLAIN 结果中的 key, type, rows 字段含义**|key: 使用的索引；type: 访问类型；rows: 扫描行数；Extra: 额外信息（Using index、Using filesort）。|

---

## 三、进阶加分题

1. **覆盖索引示例**：请写出一个覆盖索引用于某个查询，并说明为什么无需回表。  
2. **分页方案对比**：offset 分页与 keyset 分页的 SQL 示例与性能差异说明。  
3. **实现带重试的事务**：在 Go 中编写 retry 机制，捕获死锁错误并指数退避重试。

---

> 建议：将上述问答手写至少三遍，并在本地 MySQL 中复现核心 SQL 示例，提高面试自信。祝你成功！🚀