.PHONY: all build build-linux build-linux-arm64 build-darwin build-windows run dev stop status restart clean test fmt vet deps docker help

# ============================================================
# 变量
# ============================================================
BINARY_NAME   := smp-server
MAIN_PATH     := ./cmd/server
PORT          ?= 18080
DOCKER_IMAGE  ?= smash-hq/server-management:latest
LDFLAGS       := -w -s
PID_FILE      := /tmp/$(BINARY_NAME).pid

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
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*?##/ { printf "  %-22s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

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
# 运行管理
# ============================================================
## run: 编译并启动服务（前台运行）
run: build
	@echo "==> 启动服务 (端口 $(PORT))..."
	@lsof -ti:$(PORT) | xargs kill -9 2>/dev/null || true
	@sleep 1
	@echo "==> 服务已启动: http://localhost:$(PORT)"
	@./$(BINARY_NAME)

## dev: 开发模式（直接 go run，无编译步骤）
dev:
	@echo "==> 开发模式启动 (端口 $(PORT))..."
	go run $(MAIN_PATH)/main.go

## start: 编译并后台启动服务
start: build
	@echo "==> 后台启动服务 (端口 $(PORT))..."
	@lsof -ti:$(PORT) | xargs kill -9 2>/dev/null || true
	@sleep 1
	@nohup ./$(BINARY_NAME) > /tmp/$(BINARY_NAME).log 2>&1 &
	@echo $$! > $(PID_FILE)
	@echo "==> 服务已启动: http://localhost:$(PORT) (PID: $$(cat $(PID_FILE))"

## stop: 停止后台服务
stop:
	@if [ -f $(PID_FILE) ]; then \
		PID=$$(cat $(PID_FILE)); \
		if kill -0 $$PID 2>/dev/null; then \
			echo "==> 停止服务 (PID: $$PID)"; \
			kill $$PID; \
			rm -f $(PID_FILE); \
			echo "==> 已停止"; \
		else \
			echo "==> 进程 $$PID 不存在"; \
			rm -f $(PID_FILE); \
		fi \
	else \
		echo "==> 通过端口 $(PORT) 查找服务..."; \
		lsof -ti:$(PORT) | xargs kill -9 2>/dev/null || echo "端口 $(PORT) 未占用"; \
	fi

## status: 查看服务运行状态
status:
	@if [ -f $(PID_FILE) ]; then \
		PID=$$(cat $(PID_FILE)); \
		if kill -0 $$PID 2>/dev/null; then \
			echo "==> 服务运行中 (PID: $$PID, 端口: $(PORT))"; \
		else \
			echo "==> 进程 $$PID 不存在（残留 PID 文件）"; \
		fi \
	elif lsof -ti:$(PORT) > /dev/null 2>&1; then \
		echo "==> 端口 $(PORT) 被占用"; \
	else \
		echo "==> 服务未运行"; \
	fi

## restart: 重启服务
restart: stop
	@sleep 1
	@$(MAKE) start

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
	rm -f $(PID_FILE) /tmp/$(BINARY_NAME).log /tmp/smp-server
	rm -rf /tmp/go-build-*/ 2>/dev/null || true
	@echo "==> 清理完成"
