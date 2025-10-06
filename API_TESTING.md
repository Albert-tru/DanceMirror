# DanceMirror API 测试指南

## 📋 测试文件说明

### 1. `api-test.http` - REST Client 测试文件

使用 VS Code 的 REST Client 插件进行 API 测试。

#### 安装 REST Client 插件
1. 打开 VS Code
2. 按 `Ctrl+Shift+X` 打开扩展市场
3. 搜索 "REST Client"
4. 安装 Huachao Mao 的 REST Client 插件

#### 使用方法
1. 在 VS Code 中打开 `api-test.http` 文件
2. 确保服务器正在运行: `make run` 或 `./bin/dancemirror`
3. 点击请求上方的 "Send Request" 链接
4. 或者按 `Ctrl+Alt+R` (Windows/Linux) 或 `Cmd+Alt+R` (Mac)

#### 测试流程
```
1. 用户注册    → POST /api/v1/register
2. 用户登录    → POST /api/v1/login (会自动提取 token)
3. 获取视频列表 → GET /api/v1/videos (使用 token)
4. 上传视频    → POST /api/v1/videos/upload (需要实际视频文件)
5. 获取视频详情 → GET /api/v1/videos/{id}
6. 删除视频    → DELETE /api/v1/videos/{id}
```

#### 变量说明
- `@baseUrl` - API 基础 URL
- `@token` - 自动从登录响应中提取
- `@testEmail` - 测试用户邮箱
- `@testPassword` - 测试用户密码

### 2. `db-queries.sql` - 数据库查询文件

包含常用的数据库查询语句。

#### 使用方法

**方式 1: 命令行执行**
```bash
# 执行单个查询
mysql -u dmuser -pDance@2025 dancemirror -e "SELECT * FROM users;"

# 执行文件中的查询
mysql -u dmuser -pDance@2025 dancemirror < db-queries.sql
```

**方式 2: MySQL 客户端**
```bash
# 进入 MySQL
mysql -u dmuser -pDance@2025 dancemirror

# 在 MySQL 中执行
source db-queries.sql;
```

**方式 3: VS Code MySQL 扩展**
1. 安装 MySQL 扩展
2. 连接到数据库
3. 打开 `db-queries.sql` 并执行查询

### 3. `test_api.sh` - Shell 脚本测试

自动化 API 测试脚本。

#### 使用方法
```bash
# 给予执行权限
chmod +x test_api.sh

# 运行测试
./test_api.sh
```

## 🚀 快速开始

### 1. 启动服务器
```bash
# 构建并运行
make run

# 或者直接运行
./bin/dancemirror

# 或者后台运行
nohup ./bin/dancemirror > server.log 2>&1 &
```

### 2. 验证服务器运行
```bash
# 检查进程
ps aux | grep dancemirror

# 检查日志
tail -f server.log
```

### 3. 运行 API 测试

**使用 REST Client (推荐)**
- 打开 `api-test.http`
- 依次点击 "Send Request"

**使用 curl**
```bash
# 注册
curl -X POST http://localhost:8080/api/v1/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"Test123","firstName":"Test","lastName":"User"}'

# 登录
curl -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"Test123"}'

# 获取视频 (需要 token)
curl -X GET http://localhost:8080/api/v1/videos \
  -H "Authorization: Bearer YOUR_TOKEN_HERE"
```

### 4. 查看数据库

```bash
# 查看所有用户
mysql -u dmuser -pDance@2025 dancemirror -e "SELECT * FROM users;"

# 查看所有视频
mysql -u dmuser -pDance@2025 dancemirror -e "SELECT * FROM videos;"

# 查看统计信息
mysql -u dmuser -pDance@2025 dancemirror -e "
  SELECT 
    (SELECT COUNT(*) FROM users) as total_users,
    (SELECT COUNT(*) FROM videos) as total_videos;
"
```

## 📊 测试场景

### 正常流程测试
1. ✅ 用户注册
2. ✅ 用户登录获取 token
3. ✅ 使用 token 访问受保护的 API
4. ✅ 上传视频
5. ✅ 获取视频列表
6. ✅ 删除视频

### 异常流程测试
1. ❌ 重复注册（应该失败）
2. ❌ 错误密码登录（应该失败）
3. ❌ 未授权访问（应该失败）
4. ❌ 无效 token（应该失败）
5. ❌ 删除不存在的视频（应该失败）

## 🔧 故障排查

### 服务器无响应
```bash
# 检查服务器是否运行
curl http://localhost:8080/api/v1/videos

# 查看错误日志
tail -f server.log

# 重启服务器
pkill dancemirror
./bin/dancemirror
```

### 数据库连接失败
```bash
# 测试数据库连接
mysql -u dmuser -pDance@2025 dancemirror -e "SELECT 1;"

# 检查数据库状态
sudo systemctl status mysql

# 查看迁移状态
make migrate-status
```

### Token 认证失败
1. 确保从登录响应中获取了正确的 token
2. 检查 token 格式: `Authorization: Bearer <token>`
3. 确认 JWT_SECRET 配置正确
4. 检查 token 是否过期（默认 72 小时）

## 📝 常用命令

```bash
# 数据库管理
make migrate-up          # 应用迁移
make migrate-down        # 回滚迁移
make migrate-status      # 查看迁移状态

# 应用管理
make build              # 构建应用
make run                # 运行应用
make test               # 运行测试
make clean              # 清理构建文件

# 服务器管理
pkill dancemirror       # 停止服务器
./bin/dancemirror &     # 后台启动
ps aux | grep dance     # 查看进程
```

## 🎯 下一步

- [ ] 创建测试视频文件进行上传测试
- [ ] 测试大文件上传
- [ ] 测试并发请求
- [ ] 添加性能测试
- [ ] 添加安全测试
- [ ] 编写自动化测试脚本

