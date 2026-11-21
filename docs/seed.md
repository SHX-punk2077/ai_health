# 开发演示数据（Seed）

> 本文说明如何为本项目生成 Vue3 前端联调用的健康演示数据。
> 演示数据只用于本地开发环境，**不得写入测试库、生产库、远程数据库或任何包含真实用户数据的数据库**。

## 1. 与测试数据分离

| 类型 | 数据库 | 生成方式 | 用途 |
| --- | --- | --- | --- |
| 单元/集成测试 | `ai_health_test` | `go test`，测试内 TRUNCATE 并重建数据 | 验证接口、边界、隔离 |
| 开发演示数据 | `ai_health_dev` 或 `ai_health_demo` | `cmd/seed` | 前端联调、页面演示 |
| 生产数据 | 生产库 | 禁止使用本工具 | 真实业务 |

测试库必须继续使用 `ai_health_test`；公开的 Seed 命令会主动拒绝库名包含 `test` 的 DSN。
演示数据不参与自动化测试，测试也不依赖演示数据。Seed 的内部数据库测试使用
`apply(..., allowTest=true)` 临时允许 `_test` 库，该开关不暴露给命令行。

## 2. 安全限制

命令只从环境变量 `AIH_SEED_MYSQL_DSN` 读取数据库连接，不读取配置文件中的默认 DSN。

执行前会检查：

1. DSN 不能为空，且必须是合法 MySQL DSN。
2. 库名必须以 `_dev` 或 `_demo` 结尾。
3. 库名中出现 `prod`、`production`、`test` 时直接拒绝（`_test` 仅限内部测试调用）。
   检查的是**库名**，不是用户名或密码。
4. 数据库主机只能是本机回环地址：`localhost`、`127.0.0.1`、`::1`，或本机 Unix socket
   （`/var/run/mysqld/`、`/run/mysqld/`、`/tmp/` 下）。
   `host.docker.internal`、Docker 服务名、`0.0.0.0`、生产域名、公网 IP 等一律拒绝。
   Docker 中的 MySQL 需要把端口映射到主机 `127.0.0.1`，不能直接写容器服务名。
5. 默认是 **dry-run**，只打印目标库、锚点、随机种子和三个演示用户的记录数，不连接数据库。
6. 只有显式传入 `-yes` 才建立数据库连接并写入。
7. `Apply` 在事务开始前会再次校验**实际连接**：dialector 必须是 MySQL、DSN 必须满足上述规则，
   且 `SELECT DATABASE()` 必须与 DSN 中的库名一致。不能只靠 `Run` 的前置检查绕过。
8. 写入在单个事务中完成；只 upsert 三个指定演示用户，只删除/重建这三个用户的健康档案和健康记录。
9. 不对任何表执行 `TRUNCATE`，不清理其他用户、角色、菜单、运动或 AI 数据。
10. 不使用真实用户健康数据；演示账号密码只用于本地开发。

> 当前允许的开发库名示例：`ai_health_dev`、`ai_health_demo`。
> `ai_health`（无后缀）、`ai_health_test`、`ai_health_prod`、任何远程主机都会被拒绝。

### 2.1 演示用户所有权标记

Seed **不能只凭用户名**判断一个账号是不是演示账号。每个演示用户必须满足：

- 用户名是 `demo_user_a`、`demo_user_b`、`demo_user_c` 之一；
- 在 `sys_user_role` 中**恰好只有一个角色**，且该角色是 Seed 独占的 `demo_seed` 角色。

`demo_seed` 角色由 Seed 首次执行时创建：

- `status=0`（禁用），不会出现在管理端的可选角色列表中；
- 不绑定任何菜单，不具备任何 RBAC 权限；
- `role_name`、`description` 是固定所有权标记，Seed 每次都会校验；
- 普通注册只会绑定 `user` 角色，不会获得 `demo_seed`。

因此：

- 首次执行会创建三个用户并只绑定 `demo_seed`，不会绑定 `user`。
- 同名账号如果是普通用户（只有 `user` 角色），或带有 `admin`、`super_admin` 等额外角色，
  Seed 会直接拒绝，**不会修改密码、昵称、状态、软删除标记、角色或健康数据**。
- 已存在的 `demo_seed` 角色如果被启用、绑定了菜单，或名称/描述被改过，Seed 会拒绝使用，
  不会静默修复一个可能已被他人占用的角色。
- 旧版本 Seed 生成的演示用户只绑定了 `user` 角色，新版本会将其视为“非演示用户”并拒绝。
  这类环境需要人工确认后清理旧的演示账号及健康数据，再重新执行 Seed；不要通过放宽校验来接管。
- 如果同名用户恰好只有 `demo_seed` 角色，Seed 会将其视为自己创建的账号，可以重置演示密码、
  昵称、启用状态并重建健康数据。

## 3. 前置条件

1. 本机已有 MySQL 8.0，并创建了开发库：

   ```sql
   CREATE DATABASE ai_health_dev CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;
   ```

2. 对开发库执行迁移（不要对生产库执行 Seed）：

   ```powershell
   cd backend
   go run ./cmd/migrate -path migrations -database "mysql://root:<password>@tcp(127.0.0.1:3306)/ai_health_dev?multiStatements=true" up
   ```

3. 迁移后不需要预先创建 `user` 角色；Seed 会自行创建并校验 `demo_seed` 角色。
   如果该角色已存在但不满足第 2.1 节的所有权条件，Seed 会拒绝执行。

## 4. 执行

### 4.1 先 dry-run

```powershell
cd backend
$env:AIH_SEED_MYSQL_DSN="root:<password>@tcp(127.0.0.1:3306)/ai_health_dev?charset=utf8mb4&parseTime=True&loc=UTC&timeout=5s"
go run ./cmd/seed
```

输出示例（只列计数，不打印健康数值）：

```text
目标数据库: ai_health_dev
锚点日期: 2026-11-21（记录范围截止 2026-11-20）
随机种子: 20261121
演示用户: demo_user_a（档案: 是，记录: 92 条）
演示用户: demo_user_b（档案: 是，记录: 11 条）
演示用户: demo_user_c（档案: 否，记录: 24 条）
dry-run: 未写入数据库；确认执行请加 -yes。
```

dry-run 不会连接数据库，也不会校验实际库名；真正的数据库连接校验发生在 `-yes` 之后。

### 4.2 确认后写入

```powershell
go run ./cmd/seed -yes
```

可复现地指定锚点和随机种子：

```powershell
go run ./cmd/seed -yes -anchor 2026-11-21 -random-seed 20261121
```

- `-anchor` 格式为 `YYYY-MM-DD`，按 UTC 解释，不能晚于今天。
- 相同 `-anchor` 和 `-random-seed` 会生成相同的逻辑演示数据：三个用户的档案内容、
  记录数量、测量时间、指标值和来源一致。
- 但数据库自增 ID、已存在用户的 `created_at`、以及档案/记录重建后的行 ID 不保证与上次相同；
  这是“可复现的逻辑数据”，不是数据库层面的逐字节相同。
- 不同种子会改变随机指标，但三个用户的结构和边界场景仍保留。
- 重复执行是幂等的：每个演示用户仍只有一份档案，健康记录会先清理再重建，不会不断累加。
- 如果中途任何一步失败，整个事务回滚，不会留下部分用户或部分健康数据。

### 4.3 演示账号

| 用户名 | 昵称 | 密码 | 说明 |
| --- | --- | --- | --- |
| `demo_user_a` | 演示用户A（正常） | `Demo#2026` | 有档案；连续约 90 天记录，含体重、血压、血糖、心率、睡眠、饮水等 |
| `demo_user_b` | 演示用户B（数据不完整） | `Demo#2026` | 有档案；只有部分指标，其他列为 NULL |
| `demo_user_c` | 演示用户C（无档案） | `Demo#2026` | 无健康档案，只有健康记录 |

> `Demo#2026` 只是本地演示密码，禁止用于生产或其他共享环境。
> 密码以 bcrypt 哈希写入 `sys_user.password_hash`，Seed 不会输出明文。

三个用户都只绑定禁用的 `demo_seed` 角色；他们可以正常登录并访问“仅登录”的健康接口，
但不会获得管理端菜单或 RBAC 权限。

## 5. 数据覆盖场景

三个用户合计 127 条健康记录：

- 用户 A（92 条）：覆盖体重、血压、血糖、心率、睡眠、饮水；部分日期故意缺记录；
  同一天多次测量；同一 `measured_at` 的多条记录；历史补录；`manual/device/import/ai` 多种来源；
  包含血压偏高、血糖偏高、心率偏快、睡眠偏短等可触发 `warnings` 的场景。
- 用户 B（11 条）：档案存在但只有身高、体重、睡眠等部分指标；多数记录只有体重，
  用于验证 NULL 展示、缺失指标不补 0、汇总按指标分别取最新有效值。
- 用户 C（24 条）：没有档案，只有体重、血压、心率记录；用于验证“无档案用户”的
  汇总、列表和趋势，以及 BMI 无法计算时返回 `null`。
- 用户 A 另有一条 `anchor-120` 天的补录记录，用于验证历史补录不会打乱最新值选择。

## 6. 实现位置

- `backend/internal/seed/data.go`：确定性计划构造（`BuildPlan`）和三个用户的模拟数据。
- `backend/internal/seed/seed.go`：DSN/Host 校验、实际库名校验、dry-run、事务写入、
  `demo_seed` 角色创建与所有权校验、清理与用户/角色处理。
- `backend/cmd/seed/main.go`：命令行入口，只读取 `AIH_SEED_MYSQL_DSN`。
- `backend/internal/seed/seed_test.go`：确定性、DSN/Host 白名单、dry-run 不连库、计划校验测试。
- `backend/internal/seed/seed_db_test.go`：真实测试库上的首次写入、幂等、隔离、同名冲突拒绝、
  额外角色拒绝、事务回滚和公开 Apply 拒绝测试库的集成测试。

## 7. 故障排查

| 现象 | 原因 / 处理 |
| --- | --- |
| `必须通过 AIH_SEED_MYSQL_DSN 显式提供开发数据库 DSN` | 没有设置环境变量；按第 4 节设置后重试 |
| `开发数据库名必须以 _dev 或 _demo 结尾` | 库名不符合约定；不要改代码放宽限制，改用 `ai_health_dev` |
| `拒绝包含 test 的数据库` | 指向了测试库；测试数据请用 `go test` 生成 |
| `禁止连接生产数据库` | 库名包含 prod/production；立即停止，检查 DSN |
| `数据库主机 ... 不是本机回环地址` | DSN 指向了远程主机/Docker 服务名；改为 `127.0.0.1` 或 `localhost` |
| `实际连接的数据库 ... 与 DSN 数据库 ... 不一致` | 连接被代理/配置改写；检查实际 MySQL 连接 |
| `角色 demo_seed 已存在但未禁用` / `元数据与 seed 所有权标记不一致` | 角色被手工修改或占用；人工确认后清理该角色再执行 |
| `用户名 ... 已被非演示用户占用或带有额外角色` | 同名账号不是 Seed 创建的，或带有 admin/user 等额外角色；人工确认后清理旧账号，不要放宽校验 |
| `请先执行数据库迁移` | 开发库缺少基础表；先运行迁移 |
| 重复执行后记录数没有翻倍 | 正常；Seed 对三个演示用户做清理后重建 |