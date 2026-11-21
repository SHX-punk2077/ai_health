# 项目记忆：ai_health

> 供后续会话快速恢复上下文。只记录稳定事实、关键决策、验证方式与踩坑；真实密码、临时状态不写。
> 最后更新：2026-11-21

## 1. 项目基线

- 工作目录：D:\GoLand_workspace\Projects\ai_health
- Go module：ai_health（backend/go.mod，Go 1.27.4）
- 分层：router → handler → service → repository → MySQL
- entity 只做表映射，禁止 import dto；出入参走 internal/dto
- schema 唯一来源：backend/migrations（生产禁止 AutoMigrate）
- 远程：origin https://github.com/SHX-punk2077/ai_health.git，分支 main
- 约定：每完成一个重要模块提交并推送

13 张表：

sys_user、sys_oauth_identity、sys_role、sys_menu、sys_user_role、sys_role_menu、
health_profile、health_record、sport_type、sport_info、sport_disease_detail、
ai_conversation、ai_message

## 2. 数据模型层（已完成）

- 13 个 entity 均在 backend/internal/model/entity/，每表手写 TableName()，每列显式 column/type。
- 类型映射：BIGINT UNSIGNED→uint64；INT UNSIGNED→uint32；SMALLINT UNSIGNED→uint16；
  TINYINT UNSIGNED→uint8；TINYINT(1) 真布尔→bool；DECIMAL→*decimal.Decimal（禁止 float）；
  DATETIME(3)→time.Time；DATE→*time.Time；TEXT/MEDIUMTEXT→string；可空列→指针。
- created_at/updated_at：type:datetime(3);not null;autoCreateTime:milli/autoUpdateTime:milli。
- 软删除仅 sys_user、ai_conversation 使用 gorm.DeletedAt；其他表没有 deleted_at。
- 索引名、唯一索引、外键名与 ON DELETE/UPDATE 全部照抄 migration；关联字段新增一律 json:"-"。

关联：

- User 1-N OAuthIdentity / HealthRecord / AIConversation
- User 1-1 HealthProfile；HealthProfile 1-N HealthRecord（按 user_id 关联，不引入 profile_id）
- User N-N Role，Role N-N Menu，Menu 自关联 Children
- SportType 1-N SportInfo / SportDiseaseDetail
- AIConversation 1-N AIMessage

RBAC 关联表：

- 四组 many2many：SysUser.Roles、SysRole.Users、SysRole.Menus、SysMenu.Roles
- 通过 backend/internal/model/entity/joins.go 的 SetupJoinTables 注册
- join 表实体独立：SysUserRole、SysRoleMenu，均带 belongs-to
- 多对多标签使用 constraint:-，禁止 GORM 自建中间表

连接与事务：

- backend/internal/bootstrap/db.go：GORM MySQL Driver、UTC NowFunc、连接池、Ping、CloseDB
- DSN 由 AIH_MYSQL_DSN 注入；normalizeDSN 强制 parseTime、UTC、timeout、time_zone='+00:00'
- 启动不执行 AutoMigrate
- 注册建用户 + 绑默认角色在 CreateWithRole 同一事务
- AI 消息 AppendMessage 用会话行锁 + MAX(sequence_no)+1 保证会话内唯一

## 3. 关键步骤（2026-11-21）

1. 审查 backend/migrations/000001_init.up.sql（283 行、13 表、索引/外键/注释），未改 DDL。
2. 制定类型映射表与关联图，确认 GORM v1.25.12 的标签解析行为。
3. 重写 13 个 entity，补齐 belongs-to / has-one / has-many / many2many。
4. 新增 joins.go，用 SetupJoinTables 注册 4 组 RBAC 关联。
5. 补齐 bootstrap 连接层、DSN 归一化、CloseDB、连接池与日志级别。
6. 修复业务侧：注册事务、AI 消息并发序号、is_hidden/is_enabled 布尔化、来源/角色常量。
7. 增加离线 schema 一致性测试：列顺序/类型/非空/默认值/主键/自增/索引/外键逐表比对，13/13 PASS。
8. 增加真实 MySQL 集成测试：CRUD、软删/级联、一对一/一对多、DECIMAL 精度、NULL、
   多对多生命周期、外键 1452/1451、并发序号、注册事务。
9. 增加契约测试：DECIMAL 全部 decimal.Decimal、软删仅两张表、测试库恰好 13 张表无额外中间表。
10. 更新 README、docs/database.md、docs/database-review.md；gofmt/vet/test 全通过并推送。

## 4. 验证方式

本地依赖：MySQL 8 root @127.0.0.1:3306（密码只写在本机 backend/.tmp/local-credentials.md，该目录已被 .gitignore 忽略）；Redis 无密码 @127.0.0.1:6379。
开发库 ai_health、独立测试库 ai_health_test 均已按 migrations 建好 13 张表。

```powershell
cd D:\GoLand_workspace\Projects\ai_health\backend
$G='C:\Users\32951\go-toolchain\go'; $env:GOROOT=$G; $env:PATH="$G\bin;"+$env:PATH
$env:AIH_TEST_MYSQL_DSN='root:<password>@tcp(127.0.0.1:3306)/ai_health_test?charset=utf8mb4&parseTime=True&loc=UTC&timeout=5s'
gofmt -l ./internal ./cmd
go build ./...
go vet ./...
go test ./... -count=1 -p 1
```

注意：测试包之间会共用 ai_health_test，必须 `-p 1` 串行执行，否则 entity 测试重建表会打断其它包的用例。

- 测试库必须独立为 ai_health_test；测试会按 migration 重建 13 张表。
- 离线 schema 测试不需要数据库：TestGORMEntitySchemaMatchesMigration。
- 18 个顶层测试通过；entity 包约 7.6s，bootstrap/config 各约 0.8s/0.4s。
- 测试库表清单必须恰好 13 张，不能出现 GORM 自动生成的中间表。

## 5. 踩坑与决策

- GORM v1.25.12 的 *decimal.Decimal 扫描 NULL 时，如果目标字段已有非 nil 值会保留旧值；
  查询 NULL 列要用新的零值结构体。
- has-many 关系会把子表外键带回父 Model；schema 测试只比较 c.Schema.Table == parsed.Table 的本表外键。
- 关联字段不能直接出现在 API JSON；新增关联统一 json:"-"，保留 SportInfo.SportType 等既有字段。
- RBAC 必须用 SetupJoinTable 指向 sys_user_role / sys_role_menu；不要依赖 GORM 默认命名。
- health_record 没有 profile_id，按 user_id 与 health_profile 关联；HealthProfile.Records 用
  foreignKey:UserID;references:UserID;constraint:-，避免生成错误外键。
- 本地开发库凭据只记录在 gitignore 内的 backend/.tmp/local-credentials.md；MEMORY.md / docs / configs 一律用
  `<password>` 占位，真实值通过 AIH_MYSQL_DSN 注入，禁止进入任何会提交到远端的文件。
- 本机 Go 工具链若出现 Temp 写入失败，检查工具链目录的完整性级别；当前用 Medium IL 的
  Go 1.27.4 工具链构建，D:\go 原目录保留未动。
- 已确认 AutoMigrate 实际调用为 0，只有注释；生产 schema 只由 migration 演进。
- GORM 的 `default:1` 字段（sys_role.status、sys_menu.is_enabled、sys_user.status）在 Create 时会把零值回填成默认值。
  创建禁用角色 / 禁用菜单 / 禁用用户必须在 Create **之前**记录意图，再在事务内显式 UPDATE 回 0/false；
  不要在 Create 之后判断结构体字段，字段已被 GORM 改写。

## 6. 提交记录

- f10b567 feat(model): 完成 13 张表的 GORM 模型与一致性测试
- ac94d53 test(model): 补充 DECIMAL、软删与表集合契约测试
- defb821 feat(auth): implement authentication and user access control

## 7. RBAC（2026-11-21 已完成）

路由（统一 /api/v1，全部走 JWT）：

- /roles：GET role:list、GET /:id role:list、POST role:create、PUT /:id role:update、
  DELETE /:id role:delete、GET /:id/menus role:list、PUT /:id/menus role:assign。
- /menus：GET /tree menu:list、GET /my 仅登录、GET "" menu:list、GET /:id menu:list、
  POST menu:create、PUT /:id menu:update、DELETE /:id menu:delete。
- /users：GET /:id/roles user:list、PUT /:id/roles role:assign；其余管理接口见 docs/api.md。
- 运动模块路径统一为 /api/v1/sport/*（原 /api/v1/sports/* 已移除，旧路径 404）。

实现要点：

- service/role_service.go：角色 CRUD、角色菜单授权；roleCode 正则 `^[a-z][a-z0-9_]{2,49}$`；
  super_admin 不能重复创建 / 禁用 / 删除；user 内置角色不可删除；角色被活跃用户引用时拒绝删除，
  只剩软删用户时先清理其 sys_user_role 历史绑定再删除角色。
- service/menu_service.go：菜单 CRUD、树组装、父级与环校验；按钮必须有 permission；
  禁用父级级联禁用后代（父级禁用时子级不能单独启用）；删除有子节点的菜单拒绝；
  菜单变更后按 menu→role→user 失效权限缓存，级联禁用时覆盖根与全部后代关联的角色。
- service/user_service.go：管理端用户 CRUD / 启停 / 重置密码 / 角色分配；资料接口忽略 roleIds/status；
  只有 super_admin 能授予或撤销 super_admin；禁止删除/禁用自己；保留至少一个启用 super_admin。
- middleware/auth.go：JWTAuth 写 CtxUID/CtxRoles/CtxPerms；RequirePerm 服务端校验，
  super_admin 的 `*` 通配符放行；Redis 白名单不可用时当前降级为只验签名（已知风险）。
- repository：RoleRepo 增加 Search/ExistsCode/ListByIDs/CountUsers/UserIDs/DeleteUnused/
  CreateWithMenus/UpdateWithMenus；MenuRepo 增加 ExistsName/CountChildren/CountByIDs/RoleIDs/
  SetEnabled/UpdateWithDescendants；UserRepo 增加 RoleIDs/CreateWithRoles/CodesByUsers 批量角色。
- migrations/000003_rbac_permissions.up.sql：补 6 个按钮权限
  role:create/update/delete、menu:create/update/delete（id 18–23），授权 super_admin 与 admin；
  down 成对删除。未改表结构。

测试：

- backend/internal/service/rbac_service_test.go：角色列表/筛选/删除保护、角色创建与禁用、
  菜单树/父级/环/级联禁用、我的菜单过滤按钮、授权缓存失效、禁用菜单不计权限、
  用户角色分配保护、管理端用户 CRUD、事务回滚。
- backend/internal/router/authz_test.go：真实 JWT + RBAC 中间件，覆盖 401/403、管理员权限、
  super_admin `*`、禁用角色/菜单、/sport 新路径 200 与旧路径 404。
- 全量 `go test ./... -count=1 -p 1` 全绿；gofmt 无输出、go vet 通过。

## 8. 测试环境安全约定

- Redis 自动化测试固定使用 **DB 15** 并 FlushDB。DB 15 必须专用于测试，
  不得与其他程序 / 开发会话共用；生产与开发不得使用 DB 15。
- 测试库必须使用 `ai_health_test`，DSN 的库名必须以 `_test` 结尾，测试会 TRUNCATE 13 张表。
- 测试包共用同一 MySQL / Redis，必须 `-p 1` 串行执行。

文档入口：

- README.md
- docs/database.md：字段映射、ER 图、关联、连接/事务、测试矩阵
- docs/database-review.md：SQL 审查发现（P1/P2），未修改 DDL

## 9. 健康模块（2026-11-21 已完成）

路由（全部仅需登录，不需要 RBAC 权限位，数据按 JWT uid 隔离）：

- GET/POST/PUT /api/v1/health/profile
- GET/POST /api/v1/health/records、GET/DELETE /api/v1/health/records/:id
- GET /api/v1/health/trends；GET /api/v1/health/trend 为旧路径别名

实现要点：

- repository/health_repo.go：RecordFilter 的 EndAt 为开区间；ListRecords 走 user_id + measured_at；
  趋势在 MySQL 内 GROUP BY + AVG + COUNT，支持 day/week/month，week 从周一开始；
  11 个指标列白名单，禁止把外部 metric 直接拼 SQL；记录删除是物理删除。
- service/health_service.go：POST 档案严格创建，重复/并发由 uk_health_profile_user_id 兜底为 40003；
  PUT 为部分更新，update map 永不包含 user_id；文本/日期/DECIMAL 空串清 NULL；
  记录不要求先有档案，也不自动同步档案（档案是当前快照，记录可能补录历史，避免覆盖丢字段）；
  measuredAt 支持 RFC3339/两种无时区格式，无时区按 UTC，未来时间拒绝；source 只允许
  manual/device/import/ai；趋势默认最近 90 天，含首尾最多 366 个自然日，空数据返回 []。
- DTO：health_profile 与 health_record 的请求结构分离，档案独有字段绝不进入记录请求；
  DECIMAL 入参用字符串、响应用两位小数字符串，禁止 float64；Profile/Record 都是显式 DTO，不嵌 entity。
- 错误码：40001 测量时间在未来、40002 档案不存在、40003 档案已存在、40004 记录不存在、
  40005 日期格式、40006 指标非法、40007 粒度非法、40008 范围非法、40009 测量时间格式、40010 数值非法。
- 原有单表 CRUD 不引入事务；节点五为“锁记录→合并另一侧血压→校验→更新”增加了事务 + `SELECT ... FOR UPDATE`（见第 11 节）。

测试：

- backend/internal/service/health_service_test.go：档案 CRUD/并发唯一/隔离、记录 CRUD/分页/
  时间范围/指标过滤/NULL/DECIMAL/越权、measuredAt 校验、趋势 day/week/month/limit/空集合/范围边界。
- backend/internal/router/health_http_test.go：真实 JWT + Gin 全链路，覆盖 401/40002/40003/40004/
  40005/40006、userId 被忽略、pageSize 收敛、跨用户隔离、趋势空集合、坏 JSON/坏令牌。
- 全量 go test ./... -count=1 -p 1 通过；gofmt 无输出、go vet 通过。

## 10. 第四阶段增量优化（2026-11-21）

本次在原有健康模块上增量补齐，不新增 Migration、不修改历史 Migration / Entity / 表结构；
工作区仍未提交，未经用户确认不 commit。

新增接口（均为 JWT 当前用户，数据隔离不依赖管理员角色）：

- `PUT /api/v1/health/records/:id`：健康记录部分更新。
- `GET /api/v1/health/summary`：首页健康概览。

记录更新语义：

- `model/request/health.go` 的 `RecordUpdateReq` 使用 `pkg/optional.Optional[T]` 三态：
  字段未出现 / 显式 null 均保持原值，具体值才写入。
- DECIMAL 与文本（remark、sleepQuality、foodPreferences）传空串清 NULL；整数/枚举显式 null
  保持原值，当前没有清空整数/枚举的语义。
- `id、userId、user_id、createdAt、created_at、source` 为受保护字段，出现即 10001，不静默忽略。
- 空对象 10001；不存在或非本人记录统一 40004；更新后重查返回完整 Record。
- Repository 的 `UpdateRecord` 始终带 `id + user_id`，并二次拒绝受保护列。
- 更新在事务内先 `SELECT ... FOR UPDATE`（条件含 id + user_id），锁后重新读取并合并库中另一侧
  血压再校验；不同侧并发不会产生非法组合。没有版本号/乐观锁，同字段最后写入者生效；
  不同字段用列级更新，不会互相覆盖未提交字段。

健康指标校验与提醒：

- 规则集中在 `backend/internal/service/health_rule.go`，Handler 不写阈值。
- 工程硬范围（不是医学正常范围）：身高 30.00–250.00cm、体重 2.00–500.00kg、
  血糖 0.50–50.00mmol/L、总胆固醇 1.00–30.00mmol/L、视力 0.00–20.00、
  睡眠 0.00–24.00h、收缩压 40–300、舒张压 20–200、心率 20–300bpm、
  饮水 0–20000ml、年龄 0–150、枚举 0/1/2、sleepQuality≤30 字符、remark≤500 字符。
- 血压必须收缩压大于舒张压；记录/档案只改一侧时，用数据库已有另一侧合并后校验。
- 可疑数据允许保存，创建/更新记录响应可带 `warnings`（omitempty），查询历史不回放。
- 提醒阈值：收缩压≥140 或舒张压≥90/偏低、血糖>7.00 或<3.90、心率>100 或<60、
  睡眠>10 或<4、体重与 measured_at 顺序前一条有效体重比（≤7 天且≥2kg 或≥5%）。
- 血糖文案明确当前没有空腹/餐后场景，不输出确定性医学判断；所有提醒均声明不构成诊断。

健康汇总：

- 每指标独立取 `measured_at DESC, id DESC` 的最新“非 NULL 且在工程范围内”值；0、负数等历史
  脏值跳过，继续向更早记录找。最新记录缺某指标也会继续向更早记录找。
- 最新测量值只来自 health_record；无数据返回 null，不补 0。
- `lastMeasuredAt` 是最近一条记录的 measured_at；`measuredAt` / `sources` 只标记记录来源指标。
- BMI 优先记录身高/体重，缺失一侧才用档案补缺；档案值也必须落在工程范围且 >0，
  两侧都有效且 >0 才计算；`bmiSource` 为 record / record+profile / profile。
- Repository 用 1 次 UNION ALL 多指标查询 + 档案查询 + 最近测量时间查询，不逐指标发 SQL。

演示数据：

- `backend/cmd/seed` + `backend/internal/seed`，默认 dry-run，必须 `-yes` 才写库；
  DSN 只读 `AIH_SEED_MYSQL_DSN`，库名必须以 `_dev` / `_demo` 结尾，拒绝 `test`、`prod`；
  主机只允许 localhost/127.0.0.1/::1 或本机 Unix socket，拒绝远程主机、Docker 服务名和 0.0.0.0。
- `Apply` 内部再次校验 dialector 为 MySQL、DSN 规则，并要求 `SELECT DATABASE()` 等于 DSN 库名；
  不能只靠 `Run` 前置检查绕过。`_test` 只允许测试内部 `apply(..., allowTest=true)`。
- 演示用户所有权标记是 Seed 独占的 `demo_seed` 角色：禁用、无菜单、无权限；用户必须恰好只有该角色。
  同名普通用户（user 角色）或带 admin 等额外角色一律拒绝且不修改任何内容；旧版只绑 user 的演示用户
  需要人工清理后重建，不能靠用户名直接接管。
- 只 upsert `demo_user_a/b/c` 三个用户；事务内只清理/重建这三人的 profile 和 record，不 TRUNCATE。
- 相同 `-anchor` + `-random-seed` 生成相同逻辑数据（档案内容/记录数/时间/值/来源）；
  用户 created_at、自增 ID 不保证逐字节相同。A 92 条、B 11 条、C 24 条，合计 127 条。
- 演示密码只在 `docs/seed.md` 说明，仅限本地开发；不要把演示数据写入测试库或生产库。

隐私加固：

- `bootstrap.NewDB` 使用 `newDBLogger`，设置 GORM `ParameterizedQueries: true`；
  应用日志 SQL 只输出占位符，不展开健康数值（测试：`bootstrap/db_log_test.go`）。
- 访问日志仍只记 requestId/method/path/status/cost/ip/uid；新增
  `middleware/logger_test.go` 断言健康请求体、query、备注、token 不进入日志。
- `response.Fail` 只输出 code/message/requestId；错误 Detail 不入响应。
- 管理员没有健康数据旁路；健康模块不使用 Redis；AI 仍是 stub，未读取健康数据。
- 风险与原则见 `docs/health-privacy.md`：当前没有字段级加密、软删除、完整审计或备份保留策略；
  后续 AI 必须通过受控上下文组装层按最小必要读取，禁止把完整档案/全部记录直接传给第三方模型。

测试与文档：

- 新增 `pkg/optional`、记录更新、规则校验、汇总、seed、参数化日志、访问日志测试；
  原有健康模块测试继续通过。
- 节点五新增：记录更新并发/血压原子性/补录排序、Repository 事务回滚与锁归属、
  汇总脏值回退、Seed Apply 集成测试与 Host/实际库校验、10004/DB 错误不泄露。
- 文档：`docs/api.md` 已补 PUT/summary、错误码、warnings、工程范围、事务+行锁的并发语义、
  汇总对历史脏值的过滤；`docs/seed.md` 重写 demo_seed 所有权、Host 白名单、实际库校验；
  `docs/health-privacy.md` 新增；README 与 docs/README 增加入口。
- 全量 `gofmt -l ./internal ./cmd` 无输出、`go vet ./...` 通过、
  `go test ./... -count=1 -p 1` 全绿（测试库 ai_health_test，Redis DB15）。
- 本机 Go 工具链若再次出现写临时目录 Access denied，可使用 gitignored 的
  `.tmp/go-copy/go.exe` / `gofmt.exe` 并设置 `GOROOT` 与原工具链一致、`GOCACHE` 指向 `.tmp/gocache`。

## 11. 节点五修复（2026-11-21）

针对节点四只读审查提出的 P1/P2 做增量修复；没有新增 Migration，没有修改 Entity 或历史 Migration，
表结构不变，工作区仍未 commit。

P1-1 血压跨字段校验竞态：

- `repository/health_repo.go` 新增 `Transaction(ctx, fn)`、`LockRecord(ctx, uid, id)`、
  `LockProfile(ctx, uid)`；锁使用 `clause.Locking{Strength: "UPDATE"}`，
  记录锁条件同时包含 `id` 和 `user_id`，档案锁条件包含 `user_id`。
- `service/health_service.go` 的 `UpdateRecord` / `UpdateProfile` 把“锁定→重新读取→
  合并另一侧血压→校验→白名单更新→重查”全部放进同一事务；失败回滚，不留下半更新。
- 解析请求仍发生在锁后，保证跨用户更新先返回 40004，而不是先暴露参数错误。
- 测试：`health_record_update_test.go` 的并发不同值、血压原子性、补录排序；
  `repository/health_tx_test.go` 验证事务回滚与锁归属。
- 同字段仍无版本号/乐观锁，最后写入者生效。

P1-2 Seed 演示用户识别：

- 所有权标记改为 Seed 独占的 `demo_seed` 角色：创建时显式写 `status=0`，无菜单、无权限；
  已存在角色必须保持禁用、名称/描述匹配且无菜单，否则拒绝。
- 新用户只绑 `demo_seed`，不再绑 `user`；已存在用户必须恰好只有这一个角色。
  同名普通用户或带 admin 等额外角色直接拒绝，拒绝发生在修改密码/昵称/状态/健康数据之前。
- 旧版只绑 `user` 的演示用户会被视为非演示用户，需要人工清理后重建。
- 该方案不需要新增 `is_demo` 字段或 Migration。

P1-3 Seed Apply 集成测试：

- 新增 `backend/internal/seed/seed_db_test.go`，在 `ai_health_test` 上验证首次创建、
  二次幂等、普通用户完全不变、同名普通用户拒绝、演示用户带额外角色拒绝、
  中途数据库错误整体回滚、公开 Apply 拒绝测试库。

P2-1 汇总脏值：

- `repository.LatestMetrics` 增加 `MetricRange`；每个指标 SQL 在 `IS NOT NULL` 后
  加工程范围过滤，0/负数等历史脏值不会成为最新值。
- 档案补缺身高/体重也校验工程范围；BMI 必须身高、体重均在范围内且 >0 才计算。
- 测试覆盖最新 0/负数回退历史、档案补缺、最新缺体重回退更早体重、全无效返回 null。

P2-2 Seed 主机/实际库校验：

- 只允许 localhost/127.0.0.1/::1 或本机 Unix socket；拒绝远程主机、Docker 服务名、
  `host.docker.internal`、0.0.0.0 和公网地址。
- `Apply` 内部重新检查 MySQL dialector、DSN 规则和 `SELECT DATABASE()`；
  公开入口拒绝 `_test`，测试内部通过 `apply(..., true)` 使用测试库。
- 文档已同步，不再声称只检查“库名或 DSN”。

P2-3 演示年龄/生日：

- `seed/data.go` 用 `birthDateForAge(anchor, age)` 生成 A（34 岁）、B（42 岁）的生日，
  消除固定生日与年龄矛盾；新增一致性测试。

P2-4 测试缺口：

- 同时更新收缩/舒张压成功、不同值并发最后写入者、最新缺体重回退历史、历史补录排序与 warning；
- `response.Fail` 和 HTTP 10004 均验证不泄露 SQL、表名、地址、数据库内部错误；
- Seed 事务回滚集成测试。

P2-5 并发文档：

- `docs/api.md` 明确“事务 + SELECT ... FOR UPDATE、锁后合并校验、失败回滚”，
  不再写“并发请求可能基于旧值判断、调用方需接受竞态”；同字段仍最后写入者生效，无乐观锁。

验证：`gofmt -l ./internal ./cmd` 无输出、`go vet ./...` 通过、
`go test ./... -count=1 -p 1` 全绿（测试库 `ai_health_test`，Redis DB15）。
