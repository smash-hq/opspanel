# 构建阶段
FROM golang:1.26-alpine AS builder

WORKDIR /app

# 复制依赖文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源码
COPY . .

# 编译（静态链接，CGO 禁用）
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -o opspanel ./cmd/server/

# 运行阶段
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata && \
    adduser -D -g '' appuser

WORKDIR /app

# 从构建阶段复制二进制
COPY --from=builder /app/opspanel .

# 创建数据目录
RUN mkdir -p /app/data && chown -R appuser:appuser /app

USER appuser

EXPOSE 18080

VOLUME ["/app/data"]

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:18080/api/v1/health || exit 1

ENTRYPOINT ["./opspanel"]
