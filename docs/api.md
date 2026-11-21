# API 清单

统一响应体：

```json
{ "code": 0, "message": "ok", "data": {}, "requestId": "uuid" }
```

鉴权：`Authorization: Bearer <accessToken>`。错误码分段见 `internal/pkg/errcode`。

## 认证 `/api/v1/auth`

| 方法 | 路径 | 鉴权 | 说明 |
| --- | --- | --- | --- |
| POST | /register | 否 | 注册，自动绑定 user 角色 |
| POST | /login | 否 | 账号（用户名/邮箱/手机号）+ 密码 |
| POST | /refresh | 否 | 刷新令牌，refresh 一次性 |
| POST | /logout | 是 | 清空该用户全部会话 |
| GET | /profile | 是 | 当前用户 + 角色 + 权限位 |

## 系统管理 `/api/v1`

| 方法 | 路径 | 权限位 |
| --- | --- | --- |
| GET | /users | 登录即可 |
| POST | /users | user:create |
| PUT | /users/:id | user:update |
| DELETE | /users/:id | user:delete |
| PATCH | /users/:id/status | user:update |
| PATCH | /users/:id/password | user:update |
| POST | /users/:id/roles | role:assign |
| GET | /roles | 登录即可 |
| POST | /roles | role:create |
| PUT | /roles/:id | role:update |
| DELETE | /roles/:id | role:delete |
| POST | /roles/:id/menus | role:assign |
| GET | /menus/tree | 登录即可 |
| GET | /menus/my | 登录即可 |
| POST/PUT/DELETE | /menus | menu:create / menu:update / menu:delete |

## 健康 `/api/v1/health`

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | /profile | 我的健康档案（含 BMI） |
| PUT | /profile | 保存/更新档案 |
| GET | /records | 分页，含 startAt/endAt |
| POST | /records | 新增记录 |
| DELETE | /records/:id | 删除记录 |
| GET | /trend?metric=&limit= | 指标趋势 |

## 运动 `/api/v1/sports`

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | /types | 运动类型字典 |
| GET | /infos | 运动建议（可选 typeCode） |
| GET | /diseases | 疾病禁忌（可选 diseaseName、riskLevel） |
| POST | /infos | sport:create |
| PUT | /infos/:id | sport:update |

## AI `/api/v1/ai`

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | /conversations | 会话列表（updated_at 倒序） |
| POST | /conversations | 新建会话 |
| GET | /conversations/:id/messages | 消息分页 |
| DELETE | /conversations/:id | 软删会话 |
| POST | /chat | 提问，按用户限流 20 次/分钟 |
`/chat` 的 `data` 结构：

```json
{
  "conversationId": "1001",
  "userMessageId": "5001",
  "assistantMessageId": "5002",
  "answer": "……",
  "model": "deepseek-chat",
  "tokens": 320
}
```

`stream=true` 时改为 `text/event-stream`，逐条推送 `data: {"delta":"..."}`，以 `data: [DONE]` 结束。

### AI 上游说明

后端统一走 OpenAI 兼容协议，国内厂商优先：`deepseek`（默认推荐）/ `qwen`（通义千问）/ `glm`（智谱）
/ `moonshot`（Kimi），`openai` 仅作备用，`mock` 用于离线开发。切换方式见 [ai.md](./ai.md)。
上游失败对外只暴露 `60001 AI 服务暂不可用`，不转发厂商原始报文与密钥信息。
