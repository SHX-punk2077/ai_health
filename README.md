# AI 健康管理系统（ai_health）

毕业设计项目：健康档案 / 健康记录 / 运动建议 / AI 健康问答。
建表脚本来源 `ai_health_mysql8_init.sql`（已整理为 `backend/migrations` 下的版本迁移）。

技术栈：Vue3 + TypeScript + Element Plus / **Go 1.27** + Gin / GORM / MySQL 8.0 / Redis /
JWT + Redis 白名单 / **国内大模型（DeepSeek / 通义千问 / 智谱 GLM / Kimi，OpenAI 兼容协议）** /
Swagger / golang-migrate / Docker Compose

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
├── docs/                  # 架构、接口、数据表、大模型接入文档
├── Makefile               # 顶层快捷命令
└── README.md
```

## 环境要求

| 组件 | 版本 | 说明 |
| --- | --- | --- |
| Go | **1.27.4**（≥1.27） | `backend/go.mod` 已声明 `go 1.27.4`，低于此版本无法编译 |
| Node.js | 22.x（≥20） | 前端构建与 vite dev |
| npm | 10+ | 或 pnpm，`package-lock.json` 用 npm 生成 |
| Docker / Compose | 24+ / v2 | 起 MySQL、Redis、迁移容器与全栈编排 |
| MySQL | 8.0 | 由 compose 提供，本地已有实例也可复用 |
| Redis | 7 | 同上 |

检查本机版本：`go version`、`node -v`、`docker compose version`。

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

## AI 模型接入（国内优先）

默认 `provider: mock`，不调用外部接口，克隆下来即可跑通全链路；接入真实大模型只需切换
`provider` 并填入 `API_KEY`——国内四家都提供 OpenAI 兼容的 `/chat/completions` 协议，
所以 `base_url` / `model` 留空时会由服务端按 provider 自动预置。

| provider | 厂商 | 预置 base_url | 预置模型 | 申请入口 |
| --- | --- | --- | --- | --- |
| `deepseek` | DeepSeek 深度求索 | `https://api.deepseek.com/v1` | `deepseek-chat` | platform.deepseek.com |
| `qwen` | 阿里云百炼 · 通义千问 | `https://dashscope.aliyuncs.com/compatible-mode/v1` | `qwen-plus` | bailian.console.aliyun.com |
| `glm` | 智谱 AI · GLM | `https://open.bigmodel.cn/api/paas/v4` | `glm-4-flash` | open.bigmodel.cn |
| `moonshot` | 月之暗面 · Kimi | `https://api.moonshot.cn/v1` | `moonshot-v1-8k` | platform.moonshot.cn |
| `openai` | OpenAI（备用） | `https://api.openai.com/v1` | `gpt-4o-mini` | platform.openai.com |
| `mock` | 本地假数据 | — | — | 无需密钥 |

切换示例（以 DeepSeek 为例，其余只改 provider 名）：

```bash
# 本地运行
export AIH_AI_PROVIDER=deepseek
export AIH_AI_API_KEY=sk-xxxxxxxxxxxxxxxx
cd backend && go run ./cmd/api

# Docker Compose：写进 deploy/.env
AI_PROVIDER=deepseek
AI_API_KEY=sk-xxxxxxxxxxxxxxxx
```

需要换模型版本但保持同一厂商时，只填 `AIH_AI_MODEL`（如 `deepseek-reasoner`、`qwen-max`、`glm-4-plus`）；
需要接自建/中转网关时，填 `AIH_AI_BASE_URL` 覆盖预置端点即可。
安全约定：密钥只走环境变量 `AIH_AI_API_KEY`，`configs/config.yaml` 里保持空串，禁止提交真实密钥。

详见 [docs/ai.md](docs/ai.md)。

## 仓库与验证

代码托管：<https://github.com/SHX-punk2077/ai_health>（分支 `main`）

```bash
# 后端：编译 + 静态检查 + 单测（go.mod 要求 go 1.27.4）
cd backend
go build ./... && go vet ./... && go test ./...

# 前端：类型检查 + 生产构建
cd frontend
npm install && npm run type-check && npm run build
```

已完成验证（Go 1.27.4）：后端 `go build` / `go vet` / `go test` 全部通过且 gofmt 已格式化；前端 vue-tsc 类型检查与 vite build 均通过。

## 约定

- 分层：`router → handler → service → repository`，反向依赖禁止。
- `model/entity` 只做表映射，出入参一律走 `internal/dto`。
- 统一响应体 `{code,message,data,requestId}`，错误码见 `internal/pkg/errcode`。
- 生产禁用 AutoMigrate，schema 只由 `backend/migrations` 演进。
