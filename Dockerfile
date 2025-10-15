# 多阶段构建：先编译 Go 二进制，再打包到最小运行镜像
# Stage 1: 构建阶段
FROM golang:1.20-alpine AS builder

# 安装构建依赖
RUN apk add --no-cache git make

WORKDIR /app

# 复制 go.mod 和 go.sum 并下载依赖（利用 Docker 缓存层）
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建二进制文件（静态链接以便在 alpine 中运行）
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/bin/dancemirror ./cmd/main.go

# Stage 2: 运行阶段（包含 ffmpeg，为服务端转码做准备）
FROM alpine:latest

# 安装运行时依赖：ca-certificates（HTTPS）、ffmpeg（视频处理）、tzdata（时区）
RUN apk --no-cache add ca-certificates ffmpeg tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone && \
    apk del tzdata

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/bin/dancemirror /app/dancemirror

# 复制静态文件和配置（如果需要）
COPY --from=builder /app/static /app/static
COPY --from=builder /app/.env /app/.env

# 创建 uploads 目录
RUN mkdir -p /app/uploads

# 暴露端口
EXPOSE 8080

# 健康检查（Docker 原生支持）
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

# 运行服务
CMD ["/app/dancemirror"]
