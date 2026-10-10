.PHONY: all build build-linux build-linux-arm64 build-darwin build-windows run dev stop clean test fmt vet deps docker help

# ============================================================
# 变量
# ============================================================
BINARY_NAME   := smp-server
MAIN_PATH     := ./cmd/server
PORT          ?= 18080
DOCKER_IMAGE  ?= smash-hq/server-management:latest
LDFLAGS       := -w -s

# ============================================================
# 帮助
# ============================================================
## help: 显示所有可用目标
help:
	@echo "Server Plane - 服务器管理面板"
	@echo ""
	@echo "用法: make <目标>"
	@echo ""
	@echo "目标:"
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*?##/ { printf "  %-20s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

# ============================================================
# 构建
# ============================================================
## all: 完整流程（格式化 + 检查 + 编译）
all: fmt vet build

## build: 编译当前平台二进制
build:
	@echo "==> 编译 $(BINARY_NAME)..."
	CGO_ENABLED=0 go build -ldflags='$(LDFLAGS)' -o $(BINARY_NAME) $(MAIN_PATH)
	@echo "==> 编译完成: ./$(BINARY_NAME)"

## build-linux: 编译 Linux amd64 二进制（静态链接）
build-linux:
	@echo "==> 编译 $(BINARY_NAME) linux/amd64..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags='$(LDFLAGS) -extldflags "-static"' -o $(BINARY_NAME) $(MAIN_PATH)
	@echo "==> 编译完成: ./$(BINARY_NAME)"

## build-linux-arm64: 编译 Linux arm64 二进制（鲲鹏/飞腾）
build-linux-arm64:
	@echo "==> 编译 $(BINARY_NAME) linux/arm64..."
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags='$(LDFLAGS) -extldflags "-static"' -o $(BINARY_NAME)-arm64 $(MAIN_PATH)
	@echo "==> 编译完成: ./$(BINARY_NAME)-arm64"

## build-darwin: 编译 macOS 二进制
build-darwin:
	@echo "==> 编译 $(BINARY_NAME) darwin/amd64..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags='$(LDFLAGS)' -o $(BINARY_NAME)-darwin $(MAIN_PATH)
	@echo "==> 编译完成: ./$(BINARY_NAME)-darwin"

## build-windows: 编译 Windows 二进制
build-windows:
	@echo "==> 编译 $(BINARY_NAME) windows/amd64..."
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags='$(LDFLAGS)' -o $(BINARY_NAME).exe $(MAIN_PATH)
	@echo "==> 编译完成: ./$(BINARY_NAME).exe"

# ============================================================
# 运行
# ============================================================
## run: 编译并启动服务
run: build
	@echo "==> 启动服务 (端口 $(PORT))..."
	@lsof -ti:$(PORT) | xargs kill -9 2>/dev/null || true
	@sleep 1
	./$(BINARY_NAME)

## dev: 开发模式（直接 go run）
dev:
	@echo "==> 开发模式启动 (端口 $(PORT))..."
	go run $(MAIN_PATH)/main.go

## stop: 停止服务
stop:
	@echo "==> 停止服务 (端口 $(PORT))..."
	@lsof -ti:$(PORT) | xargs kill -9 2>/dev/null || echo "端口 $(PORT) 未占用"
	@rm -f $(BINARY_NAME)

# ============================================================
# 质量
# ============================================================
## test: 运行测试
test:
	@echo "==> 运行测试..."
	go test -v ./...

## fmt: 格式化代码
fmt:
	@echo "==> 格式化代码..."
	go fmt ./...

## vet: 静态检查
vet:
	@echo "==> 静态检查..."
	go vet ./...

# ============================================================
# 依赖
# ============================================================
## deps: 下载并整理依赖
deps:
	@echo "==> 下载依赖..."
	go mod download
	go mod tidy

# ============================================================
# Docker
# ============================================================
## docker: 构建 Docker 镜像
docker:
	@echo "==> 构建 Docker 镜像 $(DOCKER_IMAGE)..."
	docker build -t $(DOCKER_IMAGE) .

# ============================================================
# 清理
# ============================================================
## clean: 清理构建产物
clean:
	@echo "==> 清理..."
	rm -f $(BINARY_NAME) $(BINARY_NAME)-arm64 $(BINARY_NAME)-darwin $(BINARY_NAME).exe
	rm -f /tmp/smp-server
	rm -rf /tmp/go-build-*/ 2>/dev/null || true
	@echo "==> 清理完成"
