.DEFAULT_GOAL := help
.PHONY: all build build-all build-linux build-linux-arm64 build-darwin build-windows \
        run dev start stop status restart \
        test fmt fmt-check vet version clean deps docker help

# ============================================================
# Variables 变量
# ============================================================
BINARY_NAME   := opspanel
MAIN_PATH     := ./cmd/server
VERSION       ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GIT_COMMIT    ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
PORT          ?= 18080
DOCKER_IMAGE  ?= smash-hq/opspanel:latest
LDFLAGS       := -w -s -X main.version=$(VERSION) -X main.gitCommit=$(GIT_COMMIT)
PID_FILE      := $(BINARY_NAME).pid

# ============================================================
# Help 帮助
# ============================================================
## help: Show available targets (default) | 显示所有可用目标（默认）
help:
	@echo "OpsPanel $(VERSION) - Server Management Panel"
	@echo ""
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS = "^## |: "} /^## [a-zA-Z0-9_-]+:/ { printf "  %-22s %s\n", $$2, $$3 }' $(MAKEFILE_LIST)

# ============================================================
# Build 构建
# ============================================================
## all: Full pipeline (fmt + vet + build), run before commit | 完整流程（格式化 + 静态检查 + 编译），提交前必跑
all: fmt vet build

## build: Build binary for current platform (version injected) | 编译当前平台二进制（注入版本号）
build:
	@echo "==> Building $(BINARY_NAME) ($(VERSION))..."
	CGO_ENABLED=0 go build -ldflags='$(LDFLAGS)' -o $(BINARY_NAME) $(MAIN_PATH)
	@echo "==> Build complete: ./$(BINARY_NAME)"

## build-all: Cross-compile for all platforms (linux/darwin/windows) | 交叉编译全平台产物（linux/darwin/windows）
build-all: build-linux build-linux-arm64 build-darwin build-windows

## build-linux: Build Linux amd64 binary (static linking) | 编译 Linux amd64 二进制（静态链接）
build-linux:
	@echo "==> Building $(BINARY_NAME) linux/amd64..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags='$(LDFLAGS) -extldflags "-static"' -o $(BINARY_NAME) $(MAIN_PATH)
	@echo "==> Build complete: ./$(BINARY_NAME)"

## build-linux-arm64: Build Linux arm64 binary (Kunpeng/Phytium) | 编译 Linux arm64 二进制（鲲鹏/飞腾）
build-linux-arm64:
	@echo "==> Building $(BINARY_NAME) linux/arm64..."
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags='$(LDFLAGS) -extldflags "-static"' -o $(BINARY_NAME)-arm64 $(MAIN_PATH)
	@echo "==> Build complete: ./$(BINARY_NAME)-arm64"

## build-darwin: Build macOS amd64 binary | 编译 macOS amd64 二进制
build-darwin:
	@echo "==> Building $(BINARY_NAME) darwin/amd64..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags='$(LDFLAGS)' -o $(BINARY_NAME)-darwin $(MAIN_PATH)
	@echo "==> Build complete: ./$(BINARY_NAME)-darwin"

## build-windows: Build Windows amd64 binary | 编译 Windows amd64 二进制
build-windows:
	@echo "==> Building $(BINARY_NAME) windows/amd64..."
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags='$(LDFLAGS)' -o $(BINARY_NAME).exe $(MAIN_PATH)
	@echo "==> Build complete: ./$(BINARY_NAME).exe"

## version: Show current version | 显示当前版本号
version:
	@echo "$(VERSION)"

# ============================================================
# Run Management 运行管理
# ============================================================
## run: Build and run in foreground (Ctrl+C to stop; gracefully stops old process first) | 编译并前台启动服务（Ctrl+C 停止；启动前优雅停止旧进程）
run: build
	@echo "==> Starting service (port $(PORT))..."
	@-$(MAKE) --no-print-directory stop
	@echo "==> Service started: http://localhost:$(PORT)"
	@./$(BINARY_NAME)

## dev: Dev mode (go run; frontend changes take effect on refresh) | 开发模式（go run 直接启动，改前端刷新即生效）
dev:
	@echo "==> Starting dev mode (port $(PORT))..."
	go run $(MAIN_PATH)

## start: Build and start in background (logs to /tmp/opspanel.log) | 编译并后台启动服务（日志写入 /tmp/opspanel.log）
start: build
	@echo "==> Starting service in background (port $(PORT))..."
	@-$(MAKE) --no-print-directory stop
	@nohup ./$(BINARY_NAME) > /tmp/$(BINARY_NAME).log 2>&1 & \
	echo $$! > $(PID_FILE)
	@echo "==> Service started: http://localhost:$(PORT) (PID: $$(cat $(PID_FILE)))"

## stop: Graceful stop by PID file, wait for exit, kill -9 on timeout | 优雅停止服务（按 PID 文件停止，等待退出，超时 kill -9 兜底）
stop:
	@if [ ! -f $(PID_FILE) ]; then \
		echo "==> PID file not found ($(PID_FILE)), service not running"; \
		exit 0; \
	fi; \
	PID=$$(cat $(PID_FILE) 2>/dev/null); rm -f $(PID_FILE); \
	if [ -z "$$PID" ] || ! kill -0 $$PID 2>/dev/null; then \
		echo "==> Process not found or already exited (PID file cleaned)"; \
		exit 0; \
	fi; \
	if ! ps -p $$PID -o comm= 2>/dev/null | grep -q "$(BINARY_NAME)"; then \
		echo "==> PID $$PID occupied by another process, skip stopping (PID file cleaned)"; \
		exit 0; \
	fi; \
	echo "==> Gracefully stopping service (PID: $$PID)..."; \
	kill $$PID 2>/dev/null; \
	for i in $$(seq 1 50); do \
		kill -0 $$PID 2>/dev/null || { echo "==> Stopped"; exit 0; }; \
		sleep 0.2; \
	done; \
	echo "==> Graceful shutdown timeout (10s), force killing..."; \
	kill -9 $$PID 2>/dev/null; \
	echo "==> Force stopped"

## status: Show service status | 查看服务运行状态
status:
	@if [ -f $(PID_FILE) ]; then \
		PID=$$(cat $(PID_FILE)); \
		if kill -0 $$PID 2>/dev/null; then \
			echo "==> Service running (PID: $$PID, port: $(PORT))"; \
		else \
			echo "==> Process $$PID not found (stale PID file)"; \
		fi \
	elif lsof -ti:$(PORT) > /dev/null 2>&1; then \
		echo "==> Port $(PORT) is occupied"; \
	else \
		echo "==> Service not running"; \
	fi

## restart: Restart service (graceful stop, then background start) | 重启服务（先优雅停止，再后台启动）
restart: stop
	@$(MAKE) --no-print-directory start

# ============================================================
# Quality 质量
# ============================================================
## test: Run tests | 运行测试
test:
	@echo "==> Running tests..."
	go test ./...

## fmt: Format code | 格式化代码
fmt:
	@echo "==> Formatting code..."
	go fmt ./...

## fmt-check: Check code format (for CI, read-only) | 检查代码格式（CI 用，不修改文件）
fmt-check:
	@echo "==> Checking code format..."
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "The following files failed gofmt:"; echo "$$unformatted"; \
		exit 1; \
	fi; \
	echo "==> Format check passed"

## vet: Run static analysis | 静态检查
vet:
	@echo "==> Running go vet..."
	go vet ./...

# ============================================================
# Dependencies 依赖
# ============================================================
## deps: Download and tidy dependencies | 下载并整理依赖
deps:
	@echo "==> Downloading dependencies..."
	go mod download
	go mod tidy

# ============================================================
# Docker
# ============================================================
## docker: Build Docker image | 构建 Docker 镜像
docker:
	@echo "==> Building Docker image $(DOCKER_IMAGE)..."
	docker build -t $(DOCKER_IMAGE) .

# ============================================================
# Clean 清理
# ============================================================
## clean: Remove build artifacts | 清理构建产物
clean:
	@echo "==> Cleaning..."
	rm -f $(BINARY_NAME) $(BINARY_NAME)-arm64 $(BINARY_NAME)-darwin $(BINARY_NAME).exe
	rm -f $(PID_FILE) /tmp/$(BINARY_NAME).log
	@echo "==> Clean complete"
