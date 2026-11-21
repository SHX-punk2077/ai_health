# API 清单

统一响应体：

```json
{ "code": 0, "message": "ok", "data": {}, "requestId": "uuid" }
```

鉴权：`Authorization: Bearer <accessToken>`。错误码分段见 `internal/pkg/errcode`。

## 认证 `/api/v1/auth`

| 方法 | 路径 | 鉴权 | 说明 |
| --- | --- | --- | --- |
| POST | /register | 否 | 注册，事务内创建用户并绑定 user 角色 |
| POST | /login | 否 | 账号（用户名/邮箱/手机号）+ 密码 |
| POST | /refresh | 否 | 刷新令牌，refresh 一次性 |
| POST | /logout | 是 | 清空该用户全部会话 |
| GET | /me | 是 | 当前用户 + 角色 + 权限位 |
| GET | /profile | 是 | 与 /me 等价，保留旧路径兼容 |
| PUT | /password | 是 | 修改密码，成功后本用户全部旧令牌失效 |

### 阶段一接口详情（已实现）

**POST /auth/register**

```json
{ "username": "alice", "password": "Passw0rd!23", "nickname": "爱丽丝", "email": "", "phone": "" }
```

- username 3~32 字符，password 8~64 字符；nickname/email/phone 可省略。
- 成功 `data` 为 UserInfo（含 `roles:["user"]`），**不含 passwordHash**。
- 错误：10001 参数错误、30001 用户名/邮箱/手机号已存在、10004 事务失败。

**POST /auth/login**

```json
{ "account": "alice", "password": "Passw0rd!23" }
```

```json
{ "code": 0, "message": "ok", "data": {
  "accessToken": "eyJ...", "refreshToken": "eyJ...", "tokenType": "Bearer",
  "expiresIn": 7200, "refreshExpiresIn": 604800,
  "user": { "id": "1", "username": "alice", "roles": ["user"], "permissions": [] }
}}
```

- 错误：20004 用户名或密码错误、30003 账号已禁用、20006 连续失败超过 5 次（按 IP，15 分钟）。

**POST /auth/refresh** — body `{"refreshToken":"..."}`，一次性使用，旧 refresh 立即作废；
错误 20002 令牌无效、20003 已失效需重新登录。

**POST /auth/logout** — 无 body；清空该用户 access/refresh 白名单与角色权限缓存。

**GET /auth/me** — 返回 UserInfo；401 未登录 / 20003 令牌已吊销。

**PUT /auth/password**

```json
{ "oldPassword": "Passw0rd!23", "newPassword": "NewPassw0rd!45" }
```

- 成功返回空 data；**成功后该用户既有 access/refresh 全部失效，必须重新登录**。
- 错误：20004 旧密码错误、10001 新旧密码相同或长度不合规。

## 我的资料 `/api/v1/users`

| 方法 | 路径 | 鉴权 | 说明 |
| --- | --- | --- | --- |
| GET | /profile | 是 | 当前用户资料（与 /auth/me 同构） |
| PUT | /profile | 是 | 改昵称 / 邮箱 / 手机号 / 头像 |
| GET | /roles | 是 | 我的角色明细 |

**PUT /users/profile**：请求体只接受 `nickname、email、phone、avatar`。
即使前端额外传 `status、roleIds、permissions` 也会被**忽略**——资料接口不能用于提权。
邮箱/手机号传空串表示清空（写 NULL），冲突返回 30001。

## 系统管理 `/api/v1`

| 方法 | 路径 | 权限位 |
| --- | --- | --- |
| GET | /users | 登录即可（管理端，阶段三实现） |
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
