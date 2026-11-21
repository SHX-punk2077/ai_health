# 文档索引

| 文档 | 内容 |
| --- | --- |
| [api.md](./api.md) | 接口清单与鉴权约定 |
| [database.md](./database.md) | 13 张表与 entity/migration 的对应关系 |
| [architecture.md](./architecture.md) | 分层职责、依赖方向与关键决策 |
| [ai.md](./ai.md) | 大模型接入：国内厂商预置、Prompt 设计、限流与密钥安全 |

Swagger 由 `swag init -g cmd/api/main.go -o docs` 生成，产物为 `docs/docs.go`、`docs/swagger.json`、`docs/swagger.yaml`。
生成后在 `cmd/api/main.go` 注册 `/swagger/*any` 路由即可访问 `http://127.0.0.1:8080/swagger/index.html`。
