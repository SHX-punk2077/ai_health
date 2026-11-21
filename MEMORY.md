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

## 6. 提交记录

- f10b567 feat(model): 完成 13 张表的 GORM 模型与一致性测试
- ac94d53 test(model): 补充 DECIMAL、软删与表集合契约测试

文档入口：

- README.md
- docs/database.md：字段映射、ER 图、关联、连接/事务、测试矩阵
- docs/database-review.md：SQL 审查发现（P1/P2），未修改 DDL
