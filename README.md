# AI 健康管理系统（ai_health）

毕业设计项目：健康档案 / 健康记录 / 运动建议 / AI 健康问答。
建表脚本来源 `ai_health_mysql8_init.sql`（已整理为 `backend/migrations` 下的版本迁移）。

技术栈：Vue3 + TypeScript + Element Plus / Go + Gin / GORM / MySQL 8.0 / Redis /
JWT + Redis 白名单 / OpenAI 兼容接口 / Swagger / golang-migrate / Docker Compose

## 目录

```
ai_health/                 # 仓库根
├── backend/               # Go 服务端（独立 go module: ai_health）
│   ├── cmd/api/           # HTTP 服务入口
│   ├── cmd/migrate/       # 迁移入口（容器内用 migrate 镜像亦可）
│   ├── internal/          # config / bootstrap / router / handler / service / repository / model / dto / middleware / pkg
│   ├── migrations/        # golang-migrate 版本脚本
│   ├── configs/           # config.yaml
│   └── Dockerfile
├── frontend/              # Vue3 + TS 前端（独立 package.json）
├── deploy/                # docker-compose / nginx / .env.example
├── docs/                  # 架构、接口、数据表文档
├── Makefile               # 顶层快捷命令
└── README.md
```

## 快速开始

```bash
# 1. 起数据库与缓存（含一次性迁移）
docker compose -f deploy/docker-compose.yml up -d mysql redis migrate

# 2. 后端：先配 JWT 密钥（>=32 字节）
#    Windows PowerShell:  $env:AIH_JWT_SECRET="..."
#    bash:                export AIH_JWT_SECRET="..."
cd backend && go mod tidy && go run ./cmd/api

# 3. 前端（默认 5173，/api 代理到 8080）
cd frontend && npm install && npm run dev
```

一键全栈：`docker compose -f deploy/docker-compose.yml up -d --build`（前端 80，后端 8080）。

> 脚本不预置管理员账号。首次用 `POST /api/v1/auth/register` 注册后，
> 在 MySQL 里执行 `INSERT INTO sys_user_role(user_id, role_id) VALUES(<你的id>, 1);` 绑定 `super_admin`，
> 重新登录即可获得全部权限位。

## 仓库与验证

代码托管：<https://github.com/SHX-punk2077/ai_health>（分支 `main`）

`bash
# 后端：编译 + 静态检查（依赖钉在 go 1.25，务必带 GOTOOLCHAIN=local）
cd backend
GOTOOLCHAIN=local go build ./... && GOTOOLCHAIN=local go vet ./...

# 前端：类型检查 + 生产构建
cd frontend
npm install && npm run type-check && npm run build
`

已完成验证：后端 build/vet 均通过且 gofmt 已格式化；前端 vue-tsc 类型检查与 vite build 均通过。
## 约定

- 分层：`router → handler → service → repository`，反向依赖禁止。
- `model/entity` 只做表映射，出入参一律走 `internal/dto`。
- 统一响应体 `{code,message,data,requestId}`，错误码见 `internal/pkg/errcode`。
- 生产禁用 AutoMigrate，schema 只由 `backend/migrations` 演进。
