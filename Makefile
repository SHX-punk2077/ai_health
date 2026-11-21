# 顶层编排：make api / make web / make up
SHELL := /bin/bash
BACKEND := backend
FRONTEND := frontend

.PHONY: help api web tidy build fmt vet test seed swag migrate-up migrate-down migrate-create up down logs ps

help:
	@echo "api            启动后端 (go run ./cmd/api)"
	@echo "web            启动前端 (vite dev)"
	@echo "tidy           整理 Go 依赖"
	@echo "build          编译后端到 backend/bin"
	@echo "fmt / vet/test Go 格式化 / 静态检查 / 单测"
	@echo "seed           开发演示数据（默认 dry-run；make seed ARGS=-yes 写入）"
	@echo "swag           生成 Swagger 文档"
	@echo "migrate-up     执行数据库迁移"
	@echo "up / down      启动 / 停止 docker compose 全栈"
	@echo "logs           查看 api 容器日志"

api:
	cd $(BACKEND) && go run ./cmd/api

web:
	cd $(FRONTEND) && npm run dev

tidy:
	cd $(BACKEND) && go mod tidy

build:
	cd $(BACKEND) && go build -o bin/ai_health.exe ./cmd/api

fmt:
	cd $(BACKEND) && gofmt -s -w .

vet:
	cd $(BACKEND) && go vet ./...

test:
	cd $(BACKEND) && go test ./... -count=1 -p 1

seed:
	cd $(BACKEND) && go run ./cmd/seed $(ARGS)

swag:
	cd $(BACKEND) && swag init -g cmd/api/main.go -o docs

migrate-up:
	docker compose -f deploy/docker-compose.yml run --rm migrate

migrate-down:
	docker compose -f deploy/docker-compose.yml run --rm migrate -path /migrations -database "mysql://root:${MYSQL_ROOT_PASSWORD:-root}@tcp(mysql:3306)/ai_health?multiStatements=true" down 1

migrate-create:
	cd $(BACKEND) && migrate create -ext sql -dir migrations -seq $(name)

up:
	docker compose -f deploy/docker-compose.yml up -d --build

down:
	docker compose -f deploy/docker-compose.yml down

logs:
	docker compose -f deploy/docker-compose.yml logs -f api

ps:
	docker compose -f deploy/docker-compose.yml ps
