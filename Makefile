.DEFAULT_GOAL := help
.PHONY: all build build-all build-linux build-linux-arm64 build-darwin build-windows \
        run dev start stop status restart \
        test fmt fmt-check vet version clean deps docker help

# ============================================================
# 变量
# ============================================================
BINARY_NAME   := smp-server
MAIN_PATH     := ./cmd/server
VERSION       ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GIT_COMMIT    ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
PORT          ?= 18080
DOCKER_IMAGE  ?= smash-hq/server-management:latest
LDFLAGS       := -w -s -X main.version=$(VERSION) -X main.gitCommit=$(GIT_COMMIT)
PID_FILE      := /tmp/$(BINARY_NAME).pid

# ============================================================
# 帮助
# ============================================================
## help: 显示所有可用目标（默认）
help:
	@echo "Server Plane $(VERSION) - 服务器管理面板"
	@echo ""
	@echo "用法: make <目标>"
	@echo ""
	@echo "目标:"
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*?##/ { printf "  %-22s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

# ============================================================
# 构建
# ============================================================
## all: 完整流程（格式化 + 静态检查 + 编译），提交前必跑
all: fmt vet build

## build: 编译当前平台二进制（注入版本号）
build:
	@echo "==> 编译 $(BINARY_NAME) ($(VERSION))..."
	CGO_ENABLED=0 go build -ldflags='$(LDFLAGS)' -o $(BINARY_NAME) $(MAIN_PATH)
	@echo "==> 编译完成: ./$(BINARY_NAME)"

## build-all: 交叉编译全平台产物（linux/darwin/windows）
build-all: build-linux build-linux-arm64 build-darwin build-windows

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

## build-darwin: 编译 macOS amd64 二进制
build-darwin:
	@echo "==> 编译 $(BINARY_NAME) darwin/amd64..."
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags='$(LDFLAGS)' -o $(BINARY_NAME)-darwin $(MAIN_PATH)
	@echo "==> 编译完成: ./$(BINARY_NAME)-darwin"

## build-windows: 编译 Windows amd64 二进制
build-windows:
	@echo "==> 编译 $(BINARY_NAME) windows/amd64..."
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags='$(LDFLAGS)' -o $(BINARY_NAME).exe $(MAIN_PATH)
	@echo "==> 编译完成: ./$(BINARY_NAME).exe"

## version: 显示当前版本号
version:
	@echo "$(VERSION)"

# ============================================================
# 运行管理
# ============================================================
## run: 编译并前台启动服务（Ctrl+C 停止；启动前优雅停止旧进程）
run: build
	@echo "==> 启动服务 (端口 $(PORT))..."
	@-$(MAKE) --no-print-directory stop
	@echo "==> 服务已启动: http://localhost:$(PORT)"
	@./$(BINARY_NAME)

## dev: 开发模式（go run 直接启动，改前端刷新即生效）
dev:
	@echo "==> 开发模式启动 (端口 $(PORT))..."
	go run $(MAIN_PATH)

## start: 编译并后台启动服务（日志写入 /tmp/$(BINARY_NAME).log）
start: build
	@echo "==> 后台启动服务 (端口 $(PORT))..."
	@-$(MAKE) --no-print-directory stop
	@nohup ./$(BINARY_NAME) > /tmp/$(BINARY_NAME).log 2>&1 & \
	echo $$! > $(PID_FILE)
	@echo "==> 服务已启动: http://localhost:$(PORT) (PID: $$(cat $(PID_FILE)))"

## stop: 优雅停止服务（等待进程退出，超时 kill -9 兜底）
stop:
	@if [ -f $(PID_FILE) ]; then \
		PIDS=$$(cat $(PID_FILE)); rm -f $(PID_FILE); \
	else \
		PIDS=$$(lsof -ti:$(PORT)); \
	fi; \
	if [ -z "$$PIDS" ]; then \
		echo "==> 服务未运行"; \
		exit 0; \
	fi; \
	echo "==> 优雅停止服务 (PID: $$(echo $$PIDS | tr '\n' ' '))..."; \
	for pid in $$PIDS; do kill $$pid 2>/dev/null; done; \
	for i in $$(seq 1 50); do \
		ALIVE=0; \
		for pid in $$PIDS; do kill -0 $$pid 2>/dev/null && ALIVE=1; done; \
		[ $$ALIVE -eq 0 ] && { echo "==> 已停止"; exit 0; }; \
		sleep 0.2; \
	done; \
	echo "==> 优雅关闭超时 (10s)，强制终止..."; \
	for pid in $$PIDS; do kill -9 $$pid 2>/dev/null; done; \
	echo "==> 已强制停止"

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

## restart: 重启服务（先优雅停止，再后台启动）
restart: stop
	@$(MAKE) --no-print-directory start

# ============================================================
# 质量
# ============================================================
## test: 运行测试
test:
	@echo "==> 运行测试..."
	go test ./...

## fmt: 格式化代码
fmt:
	@echo "==> 格式化代码..."
	go fmt ./...

## fmt-check: 检查代码格式（CI 用，不修改文件）
fmt-check:
	@echo "==> 检查代码格式..."
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "以下文件未通过 gofmt 检查:"; echo "$$unformatted"; \
		exit 1; \
	fi; \
	echo "==> 格式检查通过"

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
	rm -f $(PID_FILE) /tmp/$(BINARY_NAME).log
	@echo "==> 清理完成"
