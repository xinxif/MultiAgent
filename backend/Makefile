# Agent Makefile —— 常用命令
# 用法：make help 查看所有命令

APP_NAME   := agent
MAIN_PKG   := ./cmd/server
WORKER_PKG := ./cmd/worker
CLI_PKG    := ./cmd/cli
BIN_DIR    := bin

.DEFAULT_GOAL := help

## help: 查看所有可用命令
help:
	@echo "Agent 常用命令："
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':' | sed -n '1,$$p'

## run: 启动 API 服务（开发模式）
run:
	go run $(MAIN_PKG)

## run-worker: 启动异步任务 Worker（文档 ETL / 评估）
run-worker:
	go run $(WORKER_PKG)

## run-cli: 启动终端对话 Agent
run-cli:
	go run $(CLI_PKG)

## build: 编译三个二进制
build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(APP_NAME) $(MAIN_PKG)
	go build -o $(BIN_DIR)/$(APP_NAME)-worker $(WORKER_PKG)
	go build -o $(BIN_DIR)/$(APP_NAME)-cli $(CLI_PKG)

## compose-up: 启动全部基础设施（含观测栈）
compose-up:
	docker compose --profile obs -f deployments/docker-compose.yaml up -d

## compose-core: 只启动核心基础设施（PG/Redis/MinIO）
compose-core:
	docker compose -f deployments/docker-compose.yaml --profile core up -d

## compose-down: 停止并移除全部容器
compose-down:
	docker compose -f deployments/docker-compose.yaml down -v

## migrate: 执行数据库迁移并种子管理员账号（DSN 从 configs/config.yaml 或 NX_ 环境变量读取）
migrate:
	go run ./cmd/migrate

## test: 运行全部测试
test:
	go test ./... -race -timeout 120s

## cover: 生成测试覆盖率
cover:
	go test ./... -race -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

## lint: 代码静态检查（需安装 golangci-lint）
lint:
	golangci-lint run ./...

## swag: 生成 Swagger 文档（需安装 swag cli）
swag:
	swag init -g cmd/server/main.go -o api/openapi

.PHONY: help run run-worker run-cli build compose-up compose-core compose-down migrate test cover lint swag
