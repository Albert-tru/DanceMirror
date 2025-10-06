# 🕺 DanceMirror - 舞蹈镜像学习平台

一个专为舞蹈学习设计的视频分享和练习平台，支持慢速播放、镜面翻转和 AB 循环等功能。

## ✨ 功能特点

### 🎯 核心功能
- **用户系统**: 完整的注册/登录功能，JWT 认证
- **视频管理**: 上传、浏览、播放舞蹈视频
- **增强播放器**: 专为舞蹈学习设计的视频播放器

### 🎬 播放器特色功能
- **⏱️ 播放速度调节**: 0.5x - 1.5x，每次增加 0.1x，共 11 档速度
- **🪞 镜面翻转**: 一键切换镜像模式，方便对镜练习
- **🔄 AB 循环**: 设置起止点，重复练习难点动作

## 🏗️ 技术栈

### 后端
- **语言**: Go 1.20+
- **框架**: Gorilla Mux (路由)
- **数据库**: MySQL 8.0
- **认证**: JWT (JSON Web Tokens)
- **文件上传**: Multipart Form Data

### 前端
- **纯原生**: HTML5 + CSS3 + JavaScript
- **视频播放**: HTML5 Video API
- **存储**: LocalStorage (Token 管理)

### 数据库
- **迁移工具**: golang-migrate
- **表设计**: users, videos, schema_migrations

## 📦 项目结构

```
DanceMirror/
├── cmd/
│   ├── main.go              # 应用入口
│   ├── api/
│   │   └── api.go          # API 路由配置
│   └── migrate/
│       ├── main.go         # 数据库迁移工具
│       └── migrations/     # 迁移文件
├── config/
│   └── env.go              # 环境配置
├── db/
│   └── db.go               # 数据库连接
├── service/
│   ├── auth/               # JWT 认证
│   ├── user/               # 用户管理
│   └── video/              # 视频管理
├── types/
│   └── types.go            # 类型定义
├── utils/
│   └── utils.go            # 工具函数
├── static/
│   ├── index.html          # 主页面
│   └── video-player.html   # 增强播放器
├── uploads/                # 视频文件存储
├── .env                    # 环境变量
├── go.mod                  # Go 模块
└── Makefile               # 构建脚本
```

## 🚀 快速开始

### 1. 环境要求
- Go 1.20 或更高版本
- MySQL 8.0 或更高版本
- Git

### 2. 克隆项目
```bash
git clone https://github.com/Albert-tru/DanceMirror.git
cd DanceMirror
```

### 3. 配置环境变量
复制 `.env.example` 到 `.env` 并修改配置：
```bash
cp .env.example .env
```

编辑 `.env` 文件：
```env
APP_ENV=development
APP_PORT=8080

DB_HOST=localhost
DB_PORT=3306
DB_USER=dmuser
DB_PASSWORD=Dance@2025
DB_NAME=dancemirror

JWT_SECRET=your-super-secret-jwt-key
JWT_EXPIRATION=72h

UPLOAD_DIR=./uploads
MAX_UPLOAD_SIZE=524288000
```

### 4. 创建数据库和用户
```sql
CREATE DATABASE dancemirror;
CREATE USER 'dmuser'@'localhost' IDENTIFIED BY 'Dance@2025';
GRANT ALL PRIVILEGES ON dancemirror.* TO 'dmuser'@'localhost';
FLUSH PRIVILEGES;
```

### 5. 运行数据库迁移
```bash
make migrate-up
```

### 6. 构建并运行
```bash
# 构建
make build

# 运行
make run

# 或者直接运行
./bin/dancemirror
```

### 7. 访问应用
- **主页**: http://localhost:8080/static/index.html
- **增强播放器**: http://localhost:8080/static/video-player.html

## 📚 API 文档

### 用户认证

#### 注册
```http
POST /api/v1/register
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "password123",
  "firstName": "John",
  "lastName": "Doe"
}
```

#### 登录
```http
POST /api/v1/login
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "password123"
}

Response:
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

### 视频管理

#### 获取视频列表
```http
GET /api/v1/videos
Authorization: Bearer <token>
```

#### 上传视频
```http
POST /api/v1/videos/upload
Authorization: Bearer <token>
Content-Type: multipart/form-data

title: "我的舞蹈视频"
description: "描述"
video: <file>
```

#### 获取视频详情
```http
GET /api/v1/videos/{id}
Authorization: Bearer <token>
```

#### 删除视频
```http
DELETE /api/v1/videos/{id}
Authorization: Bearer <token>
```

## 🛠️ Makefile 命令

```bash
# 构建
make build

# 运行
make run

# 测试
make test

# 清理
make clean

# 数据库迁移
make migrate-up        # 应用迁移
make migrate-down      # 回滚迁移
make migrate-status    # 查看状态
```

## 📖 文档

- [前端使用指南](FRONTEND_GUIDE.md)
- [API 测试指南](API_TESTING.md)
- [数据库迁移验证](MIGRATION_VERIFICATION.md)
- [JWT 修复报告](JWT_FIX_REPORT.md)

## 🎯 开发路线图

### Phase 1: MVP - 基础视频管理 ✅ (已完成)
- [x] 用户注册/登录系统
- [x] 数据库设计
- [x] 视频上传功能
- [x] 视频播放器基础
- [x] 播放速度调节 (0.5x-1.5x)
- [x] 镜面翻转功能
- [x] AB 循环功能

### Phase 2: 用户体验优化 (计划中)
- [ ] 用户个人主页
- [ ] 视频缩略图
- [ ] 上传进度条
- [ ] 视频搜索和过滤
- [ ] 响应式设计优化

### Phase 3: 社交功能 (计划中)
- [ ] 视频评论
- [ ] 点赞和收藏
- [ ] 用户关注
- [ ] 动态通知

### Phase 4: 高级功能 (计划中)
- [ ] 练习记录和统计
- [ ] AI 动作分析
- [ ] 视频转码和压缩
- [ ] CDN 加速

## 🤝 贡献

欢迎贡献代码！请：
1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 打开 Pull Request

## 📄 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件

## 👤 作者

**Albert-tru**
- GitHub: [@Albert-tru](https://github.com/Albert-tru)

## 🙏 致谢

- 感谢所有贡献者
- 感谢开源社区的支持

## 📞 联系方式

如有问题或建议，请：
- 提交 Issue
- 发送邮件到项目维护者

---

⭐ 如果这个项目对你有帮助，请给个星标！
