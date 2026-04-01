.PHONY: help build run test clean install lint

help: ## 显示帮助信息
	@echo "可用命令:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## 编译项目
	@echo "编译智能体..."
	go build -o bin/agent cmd/agent/main.go

run: ## 运行智能体
	@echo "运行智能体..."
	go run cmd/agent/main.go

test: ## 运行测试
	@echo "运行测试..."
	go test -v ./...

test-coverage: ## 运行测试并生成覆盖率报告
	@echo "生成测试覆盖率..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

clean: ## 清理构建文件
	@echo "清理..."
	rm -rf bin/
	rm -f coverage.out coverage.html

install: ## 安装依赖
	@echo "安装依赖..."
	go mod download
	go mod tidy

lint: ## 运行代码检查
	@echo "运行 golangci-lint..."
	golangci-lint run

example: ## 运行示例
	@echo "运行示例..."
	go run examples/basic/main.go

.DEFAULT_GOAL := help
