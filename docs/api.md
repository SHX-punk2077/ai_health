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

> 全部接口**需要登录**，但不需要 RBAC 权限位；普通用户只能操作自己的数据。
> 数据归属只取 JWT 中的 uid，请求体/query 没有 `userId` 字段，额外传了也会被忽略。
> 业务错误沿用项目约定：HTTP 200 + `body.code`（认证 401、权限 403、限流 429 例外）。
> 时间统一按 UTC 存取；日期参数格式为 `YYYY-MM-DD`。

| 方法 | 路径 | 登录 | 权限 | 说明 |
| --- | --- | --- | --- | --- |
| GET | /profile | 是 | 仅登录 | 当前用户健康档案（含 BMI） |
| POST | /profile | 是 | 仅登录 | 创建档案，一人一份，严格创建 |
| PUT | /profile | 是 | 仅登录 | 部分更新档案 |
| GET | /records | 是 | 仅登录 | 分页查询当前用户记录 |
| POST | /records | 是 | 仅登录 | 新增一条测量记录 |
| GET | /records/:id | 是 | 仅登录 | 查询单条记录，校验归属 |
| PUT | /records/:id | 是 | 仅登录 | 部分更新记录，校验归属与受保护字段 |
| DELETE | /records/:id | 是 | 仅登录 | 物理删除记录，校验归属 |
| GET | /summary | 是 | 仅登录 | 首页健康数据概览，按指标取最新有效值 |
| GET | /trends | 是 | 仅登录 | 指标趋势聚合 |
| GET | /trend | 是 | 仅登录 | `/trends` 的旧路径别名，参数与响应相同 |

### 字段与精度约定

- 所有 DECIMAL 指标在请求体中用**字符串**传递，例如 `"heightCm": "175.50"`；
  响应也返回两位小数字符串，避免 float64 精度损失。
- 指标未填写时 JSON 为 `null`，不会自动写成 0；查询结果不会补零。
- 创建时 `null` 与空字符串都表示“该列没有值”。更新时**未出现的字段或 `null` 表示保持不变**；
  文本/日期/DECIMAL 字符串字段显式传 `""` 才会清成 NULL。压力、心率、饮水量、年龄、性别等
  整数/枚举字段当前只能改值，不能通过空串清空（避免把“未填写”误判成 0）。
- `health_profile` 独有 `realName、age、birthDate、gender、medicalHistory、allergyHistory`；
  `health_record` 没有这些字段，两个请求结构不能混用。
- 数据库列上限仍然生效（身高/体重 999.99，血糖/总胆固醇 9999.99，视力/睡眠 99.99，最多两位小数）；
  业务层另有更窄的**工程数据范围**，见下一节。两者都不代表医学正常范围。
- `bmi` 由服务端用身高、体重推导，身高缺失或为 0 时返回 `null`；不落库。

### 健康指标工程校验与异常提醒

> 以下范围是工程数据校验：用于拦截明显不可能、字段精度无法表达或业务上无意义的数据，
> **不是医学正常范围，也不构成诊断标准**。可疑但合法的数据允许保存，并通过 `warnings` 提示关注。

| 指标 | 工程允许范围 | 说明 |
| --- | --- | --- |
| heightCm | 30.00–250.00 | cm，两位小数 |
| weightKg | 2.00–500.00 | kg，两位小数 |
| bloodSugarMmol | 0.50–50.00 | mmol/L，两位小数 |
| totalCholesterolMmol | 1.00–30.00 | mmol/L，两位小数 |
| systolicPressure | 40–300 | mmHg，且必须大于舒张压 |
| diastolicPressure | 20–200 | mmHg，且必须小于收缩压 |
| heartRate | 20–300 | bpm |
| sleepDurationHours | 0.00–24.00 | h，两位小数 |
| visionLeft / visionRight | 0.00–20.00 | 两位小数 |
| dailyWaterMl | 0–20000 | ml |
| age | 0–150 | 岁 |
| gender / smokingStatus / drinkingStatus / exerciseHabit | 0、1、2 | 枚举 |
| sleepQuality | ≤30 字符 | 文本 |
| remark | ≤500 字符 | 文本 |

- 空字符串清空、未传字段、显式 `null` 不参与数值范围校验；只校验本次实际提供的非 NULL 值。
- 创建和更新档案、创建和更新记录都会执行硬性校验；历史数据不会被回填或改写。
- 血压只更新一侧时，会用数据库中已有的另一侧合并后校验，避免“只改收缩压绕过关联规则”。
- 违反硬性范围返回 `40010`；错误消息只说明字段和工程范围，不回显完整请求或数据库 SQL。

**可疑数据提醒**（仅创建/更新记录时返回，不落库）：

- 收缩压 ≥140 或舒张压 ≥90：偏高提醒；收缩压 <90 或舒张压 <60：偏低提醒。
- 血糖 >7.00 或 <3.90：提醒结合测量场景复测。当前数据模型没有空腹/餐后字段，
  所以不会输出确定性的医学判断。
- 心率 >100 或 <60：偏快/偏慢提醒。
- 睡眠时长 >10.00 或 <4.00：偏长/偏短提醒。
- 体重与 **measured_at 顺序上的前一条有效体重**比较（不是 created_at）；间隔 ≤7 天且
  绝对变化 ≥2.00 kg 或 ≥5.00% 时提醒；没有有效历史体重时不比较。
- 提醒对象形如：

```json
{
  "metric": "systolicPressure",
  "level": "warning",
  "message": "本次收缩压测量值需要关注，建议在安静状态下复测并结合其他测量结果评估；该提醒不构成医学诊断"
}
```

- `warnings` 只出现在创建/更新记录的响应中，正常数据不返回该字段（`omitempty`）；
  记录列表、详情和趋势不回放历史提醒。提醒不是诊断，不能替代医生判断。

### 健康档案

**GET /api/v1/health/profile**

- 登录：是；权限：仅登录。
- 请求参数：无。
- 成功：`data` 为 Profile。
- 错误：40002 尚未创建档案；20001 未登录；20003 令牌已失效。

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "id": "1",
    "userId": "2",
    "realName": "张三",
    "age": 34,
    "birthDate": "1992-02-03",
    "gender": 2,
    "heightCm": "175.50",
    "weightKg": "70.25",
    "bloodSugarMmol": "5.60",
    "systolicPressure": 120,
    "diastolicPressure": 80,
    "totalCholesterolMmol": null,
    "heartRate": 72,
    "visionLeft": "1.20",
    "visionRight": "1.00",
    "sleepDurationHours": "7.50",
    "sleepQuality": "良好",
    "smokingStatus": 0,
    "drinkingStatus": 1,
    "exerciseHabit": 2,
    "foodPreferences": "清淡",
    "dailyWaterMl": 1800,
    "medicalHistory": null,
    "allergyHistory": null,
    "bmi": "22.81",
    "createdAt": "2026-11-21T10:00:00Z",
    "updatedAt": "2026-11-21T10:00:00Z"
  }
}
```

**POST /api/v1/health/profile**

- 登录：是；权限：仅登录。
- 请求体：可只传部分字段；空对象也会创建一份全 NULL 档案（字段由用户后续补充）。
- 成功：200 + Profile。
- 错误：40003 已存在（包括并发重复创建，由唯一键 `uk_health_profile_user_id` 兜底）；
  40005 生日格式错；40010 数值超范围；10001 请求体非法。

```json
{
  "realName": "张三",
  "age": 33,
  "birthDate": "1992-02-03",
  "gender": 2,
  "heightCm": "175.50",
  "weightKg": "70.25",
  "bloodSugarMmol": "5.60",
  "systolicPressure": 120,
  "diastolicPressure": 80,
  "heartRate": 72,
  "sleepDurationHours": "7.50",
  "dailyWaterMl": 1800
}
```

- 请求体里的 `userId` 不是 DTO 字段，传入会被 JSON 解码忽略，归属永远是 JWT 用户。

**PUT /api/v1/health/profile**

- 登录：是；权限：仅登录。
- 请求体：与创建相同，但只更新出现的字段；`""` 清空文本/日期/DECIMAL 列。
- 成功：更新后重新查询并返回完整 Profile。
- 错误：40002 尚未创建档案；40005 日期格式错；40010 数值非法；10001 请求体非法。
- 不允许改变归属：更新的 SQL 始终带 `user_id = JWT uid`，且 update map 不包含 `user_id`。

### 健康记录

**POST /api/v1/health/records**

- 登录：是；权限：仅登录。
- 请求体：

```json
{
  "measuredAt": "2026-11-21T10:00:00Z",
  "heightCm": "175.50",
  "weightKg": "70.25",
  "bloodSugarMmol": "5.60",
  "systolicPressure": 120,
  "diastolicPressure": 80,
  "heartRate": 72,
  "sleepDurationHours": "7.50",
  "source": "manual",
  "remark": "晨起测量"
}
```

- `measuredAt` 必填，支持 RFC3339（推荐，带时区）或 `YYYY-MM-DD HH:mm:ss` / `YYYY-MM-DDTHH:mm:ss`
  （无时区按 UTC 解释）；不得晚于服务器当前时间。
- `source` 可省略，默认 `manual`；允许 `manual、device、import、ai`。
- 不要求用户已有健康档案；没有任何指标值的记录也允许写入（用于补录/设备占位）。
- 新增记录**不会自动覆盖健康档案**：档案是当前可编辑快照，记录是历史测量，自动同步会丢掉档案里
  记录没有的字段；本阶段不引入隐式同步，也没有多表写入事务。
- 成功：200 + Record；记录归属为 JWT 用户。
- 错误：40001 测量时间晚于当前；40009 测量时间格式错；40010 数值/来源非法；10001 请求体非法。

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "id": "101",
    "userId": "2",
    "heightCm": "175.50",
    "weightKg": "70.25",
    "bloodSugarMmol": "5.60",
    "systolicPressure": 120,
    "diastolicPressure": 80,
    "totalCholesterolMmol": null,
    "heartRate": 72,
    "visionLeft": null,
    "visionRight": null,
    "sleepDurationHours": "7.50",
    "sleepQuality": null,
    "smokingStatus": null,
    "drinkingStatus": null,
    "exerciseHabit": null,
    "foodPreferences": null,
    "dailyWaterMl": null,
    "measuredAt": "2026-11-21T10:00:00Z",
    "source": "manual",
    "remark": "晨起测量",
    "createdAt": "2026-11-21T10:00:00Z",
    "updatedAt": "2026-11-21T10:00:00Z"
  }
}
```

**GET /api/v1/health/records**

- 登录：是；权限：仅登录。
- Query：`page`、`pageSize`（默认 1/10，pageSize 上限 100，越界自动归一化）、
  `startAt`、`endAt`（`YYYY-MM-DD`，**含 endAt 当天**）、`metric`（指标名，只返回该列非 NULL 的记录）。
- 排序：`measured_at DESC, id DESC`。
- 成功：`data` 为分页结构；`list` 为 Record 数组（下面示例只列部分字段，完整字段见 POST 响应）。
- 错误：40005 日期格式错；40008 startAt 晚于 endAt；40006 不支持的指标；10001 分页/其他参数非法。

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "list": [
      { "id": "101", "userId": "2", "weightKg": "70.25",
        "measuredAt": "2026-11-21T10:00:00Z", "source": "manual" }
    ],
    "total": 1,
    "page": 1,
    "pageSize": 10
  }
}
```

**GET /api/v1/health/records/:id**

- 登录：是；权限：仅登录。
- 路径参数：`id` 为记录 ID。
- 成功：200 + Record。
- 错误：40004 记录不存在或不属于当前用户（不区分“不存在”和“别人的”，避免泄露存在性）；
  10001 id 非法。

**PUT /api/v1/health/records/:id**

- 登录：是；权限：仅登录。路径参数 `id` 为目标记录 ID。
- 只允许更新当前 JWT 用户自己的记录；SQL 始终同时带 `id` 和 `user_id`。
- 可更新字段：`heightCm、weightKg、bloodSugarMmol、systolicPressure、diastolicPressure、
  totalCholesterolMmol、heartRate、visionLeft、visionRight、sleepDurationHours、sleepQuality、
  smokingStatus、drinkingStatus、exerciseHabit、foodPreferences、dailyWaterMl、measuredAt、remark`。
- **受保护字段**：`id、userId、user_id、createdAt、created_at、source`。请求体出现任一字段
  （包括显式 `null`）返回 `10001`，不会静默忽略；`user_id`、记录 ID、创建时间和来源都不能改。
- 三态更新语义：
  - 字段未出现：保持原值。
  - 显式 `null`：保持原值。
  - DECIMAL/文本字段传 `""`：清空为 NULL（`remark、sleepQuality、foodPreferences` 同样）。
  - 整数/枚举字段显式 `null` 仍保持原值；当前没有“清空整数/枚举”的接口语义。
- `measuredAt` 可以修改，格式和未来时间规则与创建一致；空字符串不是合法时间，返回 `40009`。
- 请求体为空对象（没有任何可更新字段）返回 `10001`。
- 更新成功后重新查询并返回完整 Record；响应可带 `warnings`，规则见上一节。
- 错误：`40004` 记录不存在或不属于当前用户；`40001` 测量时间在未来；`40009` 时间格式错；
  `40010` 数值/枚举/关联校验失败；`10001` 受保护字段、空对象、id 或 JSON 非法。
- 并发语义：更新在单个数据库事务内完成，先对目标记录执行 `SELECT ... FOR UPDATE`
  （条件同时包含记录 id 和 JWT uid），锁定后重新读取数据库中的完整记录，再合并请求字段、
  执行血压等跨字段校验并写入；校验失败会回滚，不会留下半更新状态。因此并发更新收缩压和
  舒张压时，后到的事务会基于前一个事务提交后的另一侧值校验，不会产生“收缩压不大于舒张压”
  的非法组合。
- 同一字段仍是最后写入者生效：没有版本号/乐观锁，也没有“先读后写”的冲突报错；
  不同字段使用列级更新，不会互相覆盖未提交字段。档案更新（PUT /profile）采用相同的事务
  + 行锁方式。

```json
{
  "weightKg": "72.00",
  "systolicPressure": 145,
  "diastolicPressure": 95,
  "remark": "重新测量后的数据"
}
```

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "id": "101",
    "userId": "2",
    "weightKg": "72.00",
    "systolicPressure": 145,
    "diastolicPressure": 95,
    "measuredAt": "2026-11-21T10:00:00Z",
    "source": "manual",
    "remark": "重新测量后的数据",
    "warnings": [
      {
        "metric": "systolicPressure",
        "level": "warning",
        "message": "本次收缩压测量值需要关注，建议在安静状态下复测并结合其他测量结果评估；该提醒不构成医学诊断"
      },
      {
        "metric": "diastolicPressure",
        "level": "warning",
        "message": "本次舒张压测量值需要关注，建议在安静状态下复测并结合其他测量结果评估；该提醒不构成医学诊断"
      }
    ]
  }
}
```

> 上例只展示部分字段；实际响应包含完整 Record 字段。`warnings` 仅在存在提醒时出现。

**DELETE /api/v1/health/records/:id**

- 登录：是；权限：仅登录。
- 删除策略：`health_record` **没有 deleted_at，执行物理删除**；SQL 同时带 `id` 和 `user_id`。
- 成功：200，`data` 为 `{"id":"101"}`。
- 错误：40004 记录不存在或不属于当前用户；10001 id 非法。

### 健康汇总

**GET /api/v1/health/summary**

- 登录：是；权限：仅登录。只查询 JWT 当前用户，不要求已创建健康档案。
- 每个指标独立取“该列非 NULL、处于工程允许范围、measured_at 最新、id 最新”的记录；
  不同指标的测量时间可以不同。数据库中的 0、负数等历史脏值视为无效，会继续向更早记录查找，
  不会作为最新有效值返回。
- 最新一条记录缺少某指标时，继续向更早记录查找该指标，不会把之前的有效值丢掉。
- 最新测量值只来自 `health_record`：`latestHeight、latestWeight` 等不会拿档案值冒充“最新测量”。
- `lastMeasuredAt` 是当前用户最近一条记录的 `measured_at`（即使该记录没有任何概览指标）。
- 没有数据时对应字段为 `null`，不会补 0；`measuredAt`、`sources` 为 `{}`。
- `measuredAt` 和 `sources` 只包含来自记录的指标；档案补缺不伪造测量时间。
- BMI 优先使用记录中的最新身高、体重；缺失的一侧才用健康档案补缺。档案补缺值同样必须
  落在工程范围内，不会用档案中的 0 或负数计算。`bmiSource`：
  `record`（两侧都来自记录）、`record+profile`（一侧记录一侧档案）、`profile`（两侧都来自档案）。
  只有身高、体重都落在工程范围内且均大于 0 时才计算 BMI；否则 `bmi` 与 `bmiSource` 均为 `null`。
- 实现上使用 1 次多指标 UNION 查询 + 1 次档案查询 + 1 次最近测量时间查询，不逐指标发 SQL。
- 错误：20001 未登录；20002 无效令牌；20003 令牌失效；10004 内部错误（不回显 SQL）。

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "bmi": "22.81",
    "latestHeight": "175.50",
    "latestWeight": "70.25",
    "latestBloodSugar": "5.60",
    "latestSystolicPressure": 120,
    "latestDiastolicPressure": 80,
    "latestHeartRate": 72,
    "latestSleepDurationHours": "7.50",
    "lastMeasuredAt": "2026-11-21T10:00:00Z",
    "measuredAt": {
      "heightCm": "2026-11-20T08:00:00Z",
      "weightKg": "2026-11-21T10:00:00Z",
      "bloodSugarMmol": "2026-11-21T10:00:00Z",
      "systolicPressure": "2026-11-18T09:00:00Z",
      "diastolicPressure": "2026-11-18T09:00:00Z",
      "heartRate": "2026-11-21T10:00:00Z",
      "sleepDurationHours": "2026-11-20T22:00:00Z"
    },
    "sources": {
      "heightCm": "record",
      "weightKg": "record",
      "bloodSugarMmol": "record",
      "systolicPressure": "record",
      "diastolicPressure": "record",
      "heartRate": "record",
      "sleepDurationHours": "record"
    },
    "bmiSource": "record"
  }
}
```

### 健康趋势

**GET /api/v1/health/trends**

- 登录：是；权限：仅登录。
- Query：
  - `metric` 必填，取值见下表；
  - `startDate`、`endDate`：`YYYY-MM-DD`，默认 `endDate=今天`、`startDate=endDate-89 天`；
  - `granularity`：`day`（默认）、`week`（周一开始）、`month`；
  - `limit`：可选，只保留最近 N 个聚合点；最大 1000，0/省略表示不截断。
- 规则：只查当前用户；日期范围含首尾，最多 366 个自然日；按桶升序返回；
  `week` 的 `time` 为该周周一，`month` 的 `time` 为当月 1 日；
  NULL 不参与聚合，缺失的日期/桶不会补 0；空数据返回 `points: []`。
- 聚合在 MySQL 中完成（`GROUP BY` + `AVG` + `COUNT`），不会把全部历史记录加载到内存。
- 成功：`data` 为 Trend。
- 错误：40006 非法指标；40007 非法粒度；40005 日期格式错；40008 范围倒置/超过 366 天；10001 limit 非法。

| metric | 含义 | 单位 |
| --- | --- | --- |
| heightCm | 身高 | cm |
| weightKg | 体重 | kg |
| bloodSugarMmol | 血糖 | mmol/L |
| systolicPressure | 收缩压 | mmHg |
| diastolicPressure | 舒张压 | mmHg |
| totalCholesterolMmol | 总胆固醇 | mmol/L |
| heartRate | 心率 | bpm |
| visionLeft | 左眼视力 | 空字符串 |
| visionRight | 右眼视力 | 空字符串 |
| sleepDurationHours | 睡眠时长 | h |
| dailyWaterMl | 饮水量 | ml |

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "metric": "weightKg",
    "metricName": "体重",
    "unit": "kg",
    "granularity": "day",
    "startDate": "2026-08-24",
    "endDate": "2026-11-21",
    "points": [
      { "time": "2026-11-20", "value": "70.25", "sampleCount": 1 },
      { "time": "2026-11-21", "value": "70.80", "sampleCount": 2 }
    ]
  }
}
```

### 健康模块错误码

| code | 含义 | 典型场景 |
| --- | --- | --- |
| 40001 | 测量时间不能晚于当前时间 | measuredAt 在未来 |
| 40002 | 健康档案不存在 | GET/PUT 档案但未建档 |
| 40003 | 健康档案已存在 | 重复/并发 POST 档案 |
| 40004 | 健康记录不存在 | 记录不存在或不属于当前用户 |
| 40005 | 日期格式不正确 | startAt/endAt/birthDate 不是 YYYY-MM-DD |
| 40006 | 不支持的健康指标 | metric 不在 11 个指标内 |
| 40007 | 聚合粒度非法 | granularity 不是 day/week/month |
| 40008 | 时间范围非法 | start > end 或超过 366 个自然日 |
| 40009 | 测量时间格式不正确 | measuredAt 无法解析 |
| 40010 | 健康指标数值不合法 | 超工程范围/超过两位小数/年龄或性别越界/收缩压不大于舒张压/来源非法 |
| 10001 | 参数错误 | JSON、分页、id、limit 非法；记录更新出现受保护字段或空对象 |

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
