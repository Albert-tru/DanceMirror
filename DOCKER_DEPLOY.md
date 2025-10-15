# Docker 部署指南

本文档说明如何使用 Docker 和 Docker Compose 部署 DanceMirror 项目。

## 前置要求

- Docker 20.10+ 
- Docker Compose 2.0+

检查版本：
```bash
docker --version
docker-compose --version
```

## 快速开始

### 1. 使用 Docker Compose（推荐）

一键启动应用和数据库：

```bash
# 启动所有服务（后台运行）
docker-compose up -d

# 查看日志
docker-compose logs -f app

# 查看服务状态
docker-compose ps

# 停止服务
docker-compose down

# 停止并删除数据卷（⚠️ 会删除数据库数据）
docker-compose down -v
```

服务启动后访问：
- 应用：http://localhost:8080
- 健康检查：http://localhost:8080/healthz

### 2. 仅构建镜像

如果只想构建 Docker 镜像（不启动服务）：

```bash
# 构建镜像
docker build -t dancemirror:latest .

# 查看镜像
docker images | grep dancemirror

# 运行容器（需要手动配置数据库连接）
docker run -d \
  --name dancemirror \
  -p 8080:8080 \
  -e DB_HOST=host.docker.internal \
  -e DB_PORT=3306 \
  -e DB_USER=root \
  -e DB_PASSWORD=MySQL666 \
  -e DB_NAME=dancemirror \
  -v $(pwd)/uploads:/app/uploads \
  dancemirror:latest
```

### 3. 数据库迁移

如果需要手动运行数据库迁移：

```bash
# 进入应用容器
docker-compose exec app sh

# 在容器内运行迁移
./dancemirror migrate up

# 或者从宿主机直接运行
docker-compose exec app ./dancemirror migrate up
```

## 配置说明

### 环境变量

在 `docker-compose.yml` 中配置或创建 `.env` 文件：

```env
PORT=8080
DB_HOST=db
DB_PORT=3306
DB_USER=root
DB_PASSWORD=your-secure-password
DB_NAME=dancemirror
JWT_SECRET=your-jwt-secret-key
JWT_EXPIRATION=3600
UPLOAD_DIR=/app/uploads
```

### 持久化存储

Docker Compose 自动创建以下卷：
- `db-data`：MySQL 数据库数据
- `./uploads`：用户上传的视频文件（挂载到宿主机）

### 网络

所有服务运行在 `dancemirror-network` 网络中，容器间可通过服务名互相访问。

## 生产部署建议

### 1. 使用外部数据库

修改 `docker-compose.yml` 移除 `db` 服务，并配置 `app` 服务连接外部数据库：

```yaml
services:
  app:
    environment:
      - DB_HOST=your-production-db-host
      - DB_PASSWORD=${DB_PASSWORD}  # 从环境变量或 secrets 读取
```

### 2. 使用对象存储

上传文件建议使用 S3/MinIO 等对象存储，而不是本地磁盘。

### 3. 反向代理和 HTTPS

在生产环境前置 Nginx 或 Traefik 处理 HTTPS：

```yaml
services:
  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - ./certs:/etc/nginx/certs:ro
    depends_on:
      - app
```

### 4. 资源限制

为容器设置资源限制：

```yaml
services:
  app:
    deploy:
      resources:
        limits:
          cpus: '1.0'
          memory: 512M
        reservations:
          cpus: '0.5'
          memory: 256M
```

### 5. 健康检查和重启策略

Docker Compose 已配置健康检查和自动重启（`restart: unless-stopped`）。

## 常见问题

### 端口被占用

如果 8080 或 3306 端口被占用，修改 `docker-compose.yml` 中的端口映射：

```yaml
ports:
  - "8081:8080"  # 宿主机 8081 映射到容器 8080
```

### 数据库连接失败

确保：
1. 数据库服务已启动：`docker-compose ps`
2. 健康检查通过：`docker-compose logs db`
3. 应用等待数据库就绪（`depends_on` 配置）

### 查看日志

```bash
# 查看所有服务日志
docker-compose logs -f

# 仅查看应用日志
docker-compose logs -f app

# 查看最近 100 行
docker-compose logs --tail=100 app
```

### 进入容器调试

```bash
# 进入应用容器
docker-compose exec app sh

# 进入数据库容器
docker-compose exec db mysql -uroot -pMySQL666 dancemirror
```

## 停止和清理

```bash
# 停止服务
docker-compose stop

# 停止并删除容器
docker-compose down

# 停止、删除容器和数据卷
docker-compose down -v

# 删除镜像
docker rmi dancemirror:latest
```

## 性能优化

### 多阶段构建优化

Dockerfile 已使用多阶段构建，最终镜像大小约 50-80MB。

### 缓存优化

构建时先复制 `go.mod` 和 `go.sum`，利用 Docker 缓存层加速构建。

### FFmpeg 集成

镜像已预装 ffmpeg，可用于服务端视频转码。验证：

```bash
docker-compose exec app ffmpeg -version
```

## 下一步

- 配置 CI/CD 自动构建和推送镜像
- 部署到 Kubernetes 或云平台
- 配置日志收集（ELK/Loki）
- 配置监控（Prometheus + Grafana）
