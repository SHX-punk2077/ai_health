# 架构说明

## 分层与依赖方向

```
router  →  handler  →  service  →  repository  →  MySQL
              ↓            ↓
           dto        pkg(cache/jwt/aiclient)
```

| 层 | 目录 | 只允许做的事 |
| --- | --- | --- |
| router | `internal/router` | 路由注册、中间件编排 |
| handler | `internal/handler` | 参数绑定、校验、响应封装 |
| service | `internal/service` | 业务规则、事务、缓存读写 |
| repository | `internal/repository` | SQL / GORM 查询 |
| entity | `internal/model/entity` | 表结构映射，无业务方法 |

禁止反向依赖：repository 不 import service，entity 不 import dto。

## 关键决策

1. **不启用 AutoMigrate**：schema 唯一来源是 `migrations/`，由 golang-migrate 执行。
2. **手写 TableName()**：GORM 默认把 `SysUser` 映射成 `sys_users`，与脚本中的单数表名冲突。
3. **JWT 白名单**：access token 的 jti 同时写入 Redis（`aihealth:auth:access:{uid}:{jti}`），
   登出/封号可立即失效，不必等 TTL；权限则缓存 10 分钟，改权限后最多 10 分钟生效。
4. **ID 序列化为字符串**：BIGINT UNSIGNED 超过 JS `Number.MAX_SAFE_INTEGER`，
   出参统一用 `json:"id,string"`，前端类型定义为 `string`。
5. **DECIMAL 用 shopspring/decimal**：浮点会丢精度，健康指标必须精确。
6. **健康记录不软删**：`health_record` 无 `deleted_at`，删除即物理删除，符合数据留痕的最小实现。
7. **大模型走 OpenAI 兼容协议 + provider 预置**：DeepSeek / 通义千问 / 智谱 GLM / Kimi 都提供
   `/chat/completions` 兼容端点，所以客户端只实现一份协议，切换厂商仅改 `provider` + `api_key`；
   `aiPresets` 在 `internal/config/config.go` 里预置 `base_url` / `model`，避免配置漂移。

## AI 调用链

```
handler.Chat → service.AIService（拼上下文）→ pkg/aiclient（HTTP/SSE）→ 厂商接口
                      ↓                        ↓
              repository.AIRepo           Redis：ctx 摘要 / 防重锁 / 日调用配额
                      ↓
            ai_conversation + ai_message
```

- 上下文顺序：系统提示词 → 档案摘要 → 近 7 天指标 → 最近 `history_limit` 条对话 → 本次提问。
- 落库：user 消息与 assistant 消息同一事务写入，`sequence_no` 会话内自增；上游失败则不写半截消息。
- 限流：全局 300 req/min/IP，`/ai/chat` 另加 20 req/min/用户，避免刷爆上游额度。
- 密钥只从 `AIH_AI_API_KEY` 注入，日志与响应均不回显。

## 中间件顺序

`RequestID → Logger → Recovery → CORS → (全局限流) → JWTAuth → RequirePerm`

CORS 必须在鉴权前，否则预检请求会因缺少 Authorization 被判 401。
