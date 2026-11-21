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

RBAC 规则（服务端强制执行，不依赖前端隐藏菜单）：

- 权限位由 `middleware.RequirePerm` 校验；`super_admin` 的角色权限为 `*`，放行全部权限位。
- 只有启用角色会参与角色编码 / 权限位计算；禁用菜单不产生权限位。
- `/menus/my` 只返回「启用角色 + 启用菜单」，并过滤按钮节点。
- 角色菜单授权、用户角色分配都是全量覆盖，删除与写入在同一事务内完成。
- 变更角色状态 / 菜单授权 / 用户角色后，相关用户的权限缓存立即失效。

| 方法 | 路径 | 权限位 | 说明 |
| --- | --- | --- | --- |
| GET | /users | user:list | 分页，支持 keyword / status / roleId |
| GET | /users/:id | user:list | 用户详情 |
| POST | /users | user:create | 建号；roleIds 为空时默认 user |
| PUT | /users/:id | user:update | 改资料；roleIds 字段被忽略 |
| DELETE | /users/:id | user:delete | 软删除，并踢出该用户全部会话 |
| PATCH | /users/:id/status | user:update | 启用 / 禁用；禁用后旧令牌立即失效 |
| PATCH | /users/:id/password | user:update | 管理员重置密码，旧令牌立即失效 |
| GET | /users/:id/roles | user:list | 用户已分配角色（含禁用角色） |
| PUT | /users/:id/roles | role:assign | 全量覆盖用户角色 |
| GET | /roles | role:list | 角色列表，支持 keyword / status |
| GET | /roles/:id | role:list | 角色详情 |
| POST | /roles | role:create | 新建角色，可同时授权菜单 |
| PUT | /roles/:id | role:update | 改角色；menuIds 为 nil 不动授权，[] 清空 |
| DELETE | /roles/:id | role:delete | 删除角色；仍被用户使用则拒绝 |
| GET | /roles/:id/menus | role:list | 角色已授权菜单 ID |
| PUT | /roles/:id/menus | role:assign | 全量覆盖角色菜单授权 |
| GET | /menus | menu:list | 平铺菜单列表，支持 onlyEnabled |
| GET | /menus/tree | menu:list | 菜单树（含按钮） |
| GET | /menus/:id | menu:list | 菜单详情 |
| GET | /menus/my | 仅登录 | 当前用户侧边栏菜单树 |
| POST | /menus | menu:create | 新建目录 / 菜单 / 按钮 |
| PUT | /menus/:id | menu:update | 修改菜单；禁用父级会级联禁用后代 |
| DELETE | /menus/:id | menu:delete | 删除菜单；存在子节点则拒绝 |

### 角色接口

**GET /roles?keyword=ops&status=1** — `data` 为 `RoleItem[]`：

```json
[{ "id": "2", "roleCode": "admin", "roleName": "管理员", "description": "", "status": 1 }]
```

**POST /roles**

```json
{
  "roleCode": "auditor",
  "roleName": "审计员",
  "description": "只读审计",
  "status": 1,
  "menuIds": [1, 2, 5]
}
```

- roleCode 必须匹配 `^[a-z][a-z0-9_]{2,49}$`，创建后不可修改；重复返回 30004。
- `super_admin` 是内置角色，不能重复创建（30006）。
- menuIds 由后端查库校验，任一 ID 不存在返回 10001。
- status=0 创建禁用角色时，禁用状态会真实落库。

**PUT /roles/:id** — 可传 roleName、description、status、menuIds。
**禁止把 super_admin 置为禁用**（30006）。`menuIds` 不传=保持不变，传 `[]`=清空。

**DELETE /roles/:id** — 内置 `super_admin` / `user` 不可删除（30006）；
仍被用户引用时返回 30005（并发下由删除语句的 NOT EXISTS 条件兜底）。

**GET /roles/:id/menus** — `data`：

```json
{ "roleId": 4, "menuIds": [1, 2, 5] }
```

**PUT /roles/:id/menus**

```json
{ "menuIds": [1, 2, 5] }
```

- `menuIds` 必填；传 `[]` 表示清空授权。
- 角色已禁用时拒绝授权（30007）；任一菜单不存在返回 10001。
- 成功后该角色下所有用户的权限缓存立即失效。

### 菜单接口

**GET /menus?onlyEnabled=true** — 平铺列表；`data` 为 `MenuNode[]`，字段：
`id, parentId, menuType, name, title, path, component, redirect, icon, permission,
sortOrder, isHidden, isEnabled, children`。`menuType`：1 目录、2 菜单、3 按钮。

**GET /menus/tree** — 同字段的树形结构，根节点 `parentId` 为 null，子节点在 `children`。

**POST /menus**

```json
{
  "parentId": 1,
  "menuType": 2,
  "name": "AuditLog",
  "title": "审计日志",
  "path": "/audit",
  "component": "audit/index",
  "icon": "Document",
  "permission": "audit:list",
  "sortOrder": 90,
  "isHidden": false,
  "isEnabled": true
}
```

- `menuType=3`（按钮）必须提供 `permission`，否则 10001。
- 父级必须存在、启用且不能是按钮；否则 30010。
- `name` 全局唯一，重复返回 30008。

**PUT /menus/:id** — 部分更新；`parentId=0` 表示移动到根。
父级不能是自身或后代（30011）；改成按钮前必须没有子节点（10001）。
把 `isEnabled` 置为 false 时，全部后代在同一事务内级联禁用；重新启用父级不会自动恢复后代。父级仍处于禁用状态时，单独启用子级会返回 30010。

**DELETE /menus/:id** — 存在直接子节点时返回 30009；删除成功后关联的
`sys_role_menu` 由外键级联清理，并失效相关用户权限缓存。

### 用户管理接口

**GET /users?page=1&pageSize=10&keyword=&status=&roleId=** — `data` 为分页结构：

```json
{ "list": [{ "id": "3", "username": "alice", "status": 1, "roles": ["user"] }],
  "total": 1, "page": 1, "pageSize": 10 }
```

page 默认 1，pageSize 默认 10、上限 100；非法分页参数自动归一化。

**POST /users**

```json
{ "username": "bob", "password": "Passw0rd!23", "nickname": "鲍勃",
  "email": "bob@example.com", "phone": "", "status": 1, "roleIds": [3] }
```

- 未传 roleIds / 传空数组时绑定默认 `user` 角色。
- 角色必须存在且启用（30007）；非 super_admin 不能授予 `super_admin`（20005）。

**PUT /users/:id** — 可改 nickname/email/phone/avatar/status；**roleIds 被忽略**，
分配角色只能走 `PUT /users/:id/roles`，避免资料接口被用来提权。
不能禁用自己；不能禁用最后一个启用的 super_admin（30012）。

**PATCH /users/:id/status** — body `{"status":0}` 或 `{"status":1}`；禁用后立即吊销该用户全部令牌。

**PATCH /users/:id/password** — body `{"newPassword":"NewPassw0rd!45"}`；成功后旧令牌全部失效。

**GET /users/:id/roles** — 返回 `RoleItem[]`（含禁用角色，便于管理端修正）。

**PUT /users/:id/roles**

```json
{ "roleIds": [2, 3] }
```

- `roleIds` 必填，`[]` 表示清空全部角色。
- 非 super_admin 不能修改 super_admin 用户的角色，也不能授予 super_admin。
- 移除目标用户最后一个 super_admin 角色时返回 30012。

## 健康 `/api/v1/health`

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | /profile | 我的健康档案（含 BMI） |
| PUT | /profile | 保存/更新档案 |
| GET | /records | 分页，含 startAt/endAt |
| POST | /records | 新增记录 |
| DELETE | /records/:id | 删除记录 |
| GET | /trend?metric=&limit= | 指标趋势 |

## 运动 `/api/v1/sport`

> 路径已从复数的 `/api/v1/sports/*` 统一为单数 `/api/v1/sport/*`；旧路径不再注册（404）。
> 前端当前没有任何运动接口调用；该模块接口仍为阶段五实现，现阶段返回 10003。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | /types | 运动类型字典 |
| GET | /infos | 运动建议（可选 typeCode） |
| GET | /infos/:id | 运动建议详情（阶段五） |
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
