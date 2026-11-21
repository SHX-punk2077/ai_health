# 数据表与 GORM 模型

> **Schema 唯一来源**：`backend/migrations/000001_init.up.sql`（由 `ai_health_mysql8_init.sql` 整理）。
> 生产环境禁用 `AutoMigrate`，模型标签只声明既有结构；任何表结构变更必须先写 migration。
> 表结构审查结论与改进建议见 [database-review.md](database-review.md)。

## 1. 13 张表与 entity 对应

| # | 表 | entity | 主键 | 软删除 | 关键关联 |
| --- | --- | --- | --- | --- | --- |
| 1 | `sys_user` | [sys_user.go](../backend/internal/model/entity/sys_user.go) | `id` | 是（`deleted_at`） | 角色、档案、记录、OAuth、AI 会话 |
| 2 | `sys_oauth_identity` | [sys_oauth_identity.go](../backend/internal/model/entity/sys_oauth_identity.go) | `id` | 否 | `user_id` → `sys_user.id` |
| 3 | `sys_role` | [sys_role.go](../backend/internal/model/entity/sys_role.go) | `id` | 否 | 用户、菜单（多对多） |
| 4 | `sys_menu` | [sys_menu.go](../backend/internal/model/entity/sys_menu.go) | `id` | 否 | `parent_id` 自关联、角色（多对多） |
| 5 | `sys_user_role` | [sys_user_role.go](../backend/internal/model/entity/sys_user_role.go) | `id` | 否 | 用户-角色关联实体 |
| 6 | `sys_role_menu` | [sys_role_menu.go](../backend/internal/model/entity/sys_role_menu.go) | `id` | 否 | 角色-菜单关联实体 |
| 7 | `health_profile` | [health_profile.go](../backend/internal/model/entity/health_profile.go) | `id` | 否 | `user_id` 唯一，一人一档 |
| 8 | `health_record` | [health_record.go](../backend/internal/model/entity/health_record.go) | `id` | 否 | `user_id` → `sys_user.id` |
| 9 | `sport_type` | [sport_type.go](../backend/internal/model/entity/sport_type.go) | `id` | 否 | 运动建议、疾病注意事项 |
| 10 | `sport_info` | [sport_info.go](../backend/internal/model/entity/sport_info.go) | `id` | 否 | `sport_type_id` → `sport_type.id` |
| 11 | `sport_disease_detail` | [sport_disease_detail.go](../backend/internal/model/entity/sport_disease_detail.go) | `id` | 否 | `sport_type_id` → `sport_type.id` |
| 12 | `ai_conversation` | [ai_conversation.go](../backend/internal/model/entity/ai_conversation.go) | `id` | 是（`deleted_at`） | `user_id` → `sys_user.id`，含消息 |
| 13 | `ai_message` | [ai_message.go](../backend/internal/model/entity/ai_message.go) | `id` | 否 | `conversation_id` → `ai_conversation.id` |

公共约定写在 [entity.go](../backend/internal/model/entity/entity.go)：每表手写 `TableName()`，每列显式 `column` / `type`，
索引名与外键名照抄 migration。关联字段只用于 `Preload` / `Joins`，新增关联统一 `json:"-"`，避免污染接口响应。

## 2. 数据库字段与 Go 类型映射

### 2.1 标量类型

| MySQL 类型 | Go 类型 | 说明 |
| --- | --- | --- |
| `BIGINT UNSIGNED` | `uint64` | 主键、外键；JSON 输出用 `string` 避免 JS 精度丢失（现有 DTO 约定） |
| `INT UNSIGNED` | `uint32` | 计数、饮水量、token 数 |
| `SMALLINT UNSIGNED` | `uint16` | 血压、心率、时长 |
| `TINYINT UNSIGNED` | `uint8` | 状态、性别、风险等级等枚举 |
| `TINYINT(1)` | `bool` | 仅 `sys_menu.is_hidden` / `is_enabled` 这类真布尔 |
| `DECIMAL(p,s)` | `*decimal.Decimal` | 本项目所有 DECIMAL 列均可空；非空场景用 `decimal.Decimal`，**禁止 float** |
| `DATETIME(3)` | `time.Time` | 非空时间列；毫秒精度；统一 UTC 存取 |
| `DATE` | `*time.Time` | 例如 `health_profile.birth_date` |
| `VARCHAR(n)` | `string` / `*string` | 非空用 `string`，可空用指针 |
| `TEXT` / `MEDIUMTEXT` | `string` / `*string` | 可空文本用指针；注意 MEDIUMTEXT 上限 16MB |
| `deleted_at DATETIME(3)` | `gorm.DeletedAt` | 仅 `sys_user`、`ai_conversation` |

### 2.2 可空、时间与 DECIMAL 细则

- **可空列一律用指针**：`*string`、`*uint8`、`*uint16`、`*uint32`、`*time.Time`、`*decimal.Decimal`。
  非空列不使用指针，避免业务层反复判空。
- **DECIMAL 用 shopspring/decimal**：读写都走字符串/精确十进制，不经过二进制浮点；
  `DECIMAL(5,2)`、`DECIMAL(6,2)`、`DECIMAL(4,2)` 的标度已在列定义中固定。
- **UTC 是唯一存储基准**：`DATETIME(3)` 不带时区，应用写入 UTC，展示层再转 `Asia/Shanghai`。
  `bootstrap.NewDB` 会强制 `parseTime=true`、`loc=UTC`、`time_zone='+00:00'`，即使部署 DSN 误写 `loc=Local`。
- **毫秒精度**：时间标签使用 `type:datetime(3)`；GORM 写入用 `autoCreateTime:milli` / `autoUpdateTime:milli`。
- **扫描 NULL 的注意点**：GORM v1.25 对 `*decimal.Decimal` 的 NULL 扫描，在目标字段已有非 nil 值时可能保留旧值。
  查询请使用新的零值结构体；测试 `TestNullableFields` 已覆盖。
- **JSON 序列化**：`SysUser.PasswordHash` 为 `json:"-"`；所有关联字段为 `json:"-"`。
  `SportInfo.SportType` 保留 `json:"sportType,omitempty"` 供现有 DTO 使用。

## 3. 关联关系

```mermaid
erDiagram
    SYS_USER ||--o{ SYS_OAUTH_IDENTITY : "1:N 第三方身份"
    SYS_USER ||--o{ SYS_USER_ROLE : "1:N 授权"
    SYS_ROLE ||--o{ SYS_USER_ROLE : "1:N 被授权"
    SYS_ROLE ||--o{ SYS_ROLE_MENU : "1:N 授权"
    SYS_MENU ||--o{ SYS_ROLE_MENU : "1:N 被授权"
    SYS_MENU ||--o{ SYS_MENU : "parent_id 自关联"
    SYS_USER ||--o| HEALTH_PROFILE : "1:1 当前档案"
    SYS_USER ||--o{ HEALTH_RECORD : "1:N 测量历史"
    SYS_USER ||--o{ AI_CONVERSATION : "1:N 会话"
    AI_CONVERSATION ||--o{ AI_MESSAGE : "1:N 消息"
    SPORT_TYPE ||--o{ SPORT_INFO : "1:N 运动建议"
    SPORT_TYPE ||--o{ SPORT_DISEASE_DETAIL : "1:N 疾病注意"
```

### 3.1 GORM 关联字段

| 模型字段 | 类型 | GORM 标签要点 |
| --- | --- | --- |
| `SysUser.Roles` | `[]SysRole` | `many2many:sys_user_role` + `joinForeignKey:UserID` / `joinReferences:RoleID` |
| `SysRole.Users` | `[]SysUser` | 同一 join 表的反向多对多 |
| `SysRole.Menus` | `[]SysMenu` | `many2many:sys_role_menu` |
| `SysMenu.Roles` | `[]SysRole` | 同一 join 表的反向多对多 |
| `SysMenu.Children` | `[]*SysMenu` | `foreignKey:ParentID`，自关联；保留 `json:"children,omitempty"` |
| `SysUser.Profile` | `*HealthProfile` | has one，`foreignKey:UserID`，外键 `fk_health_profile_user` |
| `HealthProfile.User` | `*SysUser` | belongs to，`references:ID` |
| `HealthProfile.Records` | `[]HealthRecord` | has many，`foreignKey:UserID;references:UserID;constraint:-`（按用户而非档案关联，避免生成错误的 `health_record.user_id → health_profile.user_id` 外键） |
| `SysUser.OauthIdentities` | `[]SysOauthIdentity` | has many，`fk_oauth_user` |
| `SysUser.HealthRecords` | `[]HealthRecord` | has many，`fk_health_record_user` |
| `SysUser.Conversations` | `[]AIConversation` | has many，`fk_ai_conversation_user` |
| `SportType.Infos` | `[]SportInfo` | has many，`fk_sport_info_type`（RESTRICT） |
| `SportType.DiseaseDetails` | `[]SportDiseaseDetail` | has many，`fk_sport_disease_type`（RESTRICT） |
| `AIConversation.Messages` | `[]AIMessage` | has many，`fk_ai_message_conversation`；保留 `json:"messages,omitempty"` |
| `AIMessage.Conversation` | `*AIConversation` | belongs to，`references:ID` |

关联表必须用 `SetupJoinTable` 注册，禁止 GORM 推导中间表：

```go
entity.SetupJoinTables(db) // 注册 sys_user_role / sys_role_menu 四组多对多
```

### 3.2 外键动作（与 migration 一致）

| 外键 | 动作 |
| --- | --- |
| 用户相关（OAuth、授权、档案、记录、AI 会话） | `ON DELETE CASCADE ON UPDATE CASCADE` |
| `sys_menu.parent_id` | `ON DELETE SET NULL ON UPDATE CASCADE` |
| `sport_info.sport_type_id` / `sport_disease_detail.sport_type_id` | `ON DELETE RESTRICT ON UPDATE CASCADE` |
| `ai_message.conversation_id` | `ON DELETE CASCADE ON UPDATE CASCADE` |

> 软删除（`sys_user`、`ai_conversation`）不会触发数据库级联；只有物理删除才会。
> 测试 `TestUserSoftDelete` 明确断言了这一点。

## 4. 连接、事务与并发

- **连接**：`bootstrap.NewDB` 使用 GORM MySQL Driver；DSN 从 `AIH_MYSQL_DSN` / `configs/config.yaml` 读取，
  代码不硬编码账号密码。连接池 `MaxOpen=25`、`MaxIdle=10`，`ConnMaxLifetime=1h`、`ConnMaxIdleTime=10m`。
- **超时与检查**：DSN 未配置 `timeout` 时补 5s；启动时 `PingContext` 5s 超时；`CloseDB` 在进程退出时释放连接。
- **日志**：release 用 `Warn`，debug 用 `Info`；慢查询阈值来自 `mysql.slow_threshold`（默认 200ms）。
- **禁止 AutoMigrate**：`NewDB` 不调用 `AutoMigrate`；测试库由 migration 初始化。
- **注册事务**：`UserRepo.CreateWithRole` 在同一事务内创建用户并绑定默认角色；任一步失败整体回滚。
- **AI 消息并发**：`AIRepo.AppendMessage` 先对 `ai_conversation` 行加 `SELECT ... FOR UPDATE`，
  再计算 `MAX(sequence_no)+1` 并插入；`uk_ai_message_conversation_sequence` 是最终唯一性保障。
- **健康档案与历史分开**：`health_profile` 是当前档案，`health_record` 是每次测量快照；
  新增历史记录**不自动**同步档案，业务需要时由 service 显式更新。

## 5. 测试

测试使用**独立测试库**，库名必须以 `_test` 结尾；测试会删除并重建 13 张表，
**绝不能**把 `AIH_TEST_MYSQL_DSN` 指向开发库。

```bash
# Linux / macOS
export AIH_TEST_MYSQL_DSN='root:your_password@tcp(127.0.0.1:3306)/ai_health_test?charset=utf8mb4&parseTime=True&loc=UTC'
# Windows PowerShell
# $env:AIH_TEST_MYSQL_DSN='root:your_password@tcp(127.0.0.1:3306)/ai_health_test?charset=utf8mb4&parseTime=True&loc=UTC'

cd backend
go test ./internal/model/entity -count=1 -v
```

| 验收点 | 测试 |
| --- | --- |
| 13 表字段/索引/外键与 migration 一致 | `TestGORMEntitySchemaMatchesMigration`（离线，不需数据库） |
| Model 增查 | `TestModelCreateAndQuery` |
| User-Role 多对多 | `TestUserRoleManyToMany` |
| Role-Menu 多对多 | `TestRoleMenuManyToMany` |
| 用户软删除 | `TestUserSoftDelete` |
| HealthProfile 一对一 | `TestHealthProfileOneToOne` |
| HealthRecord 一对多 | `TestHealthRecordOneToMany` |
| DECIMAL 精度 | `TestDecimalPrecision` |
| NULL 读写 | `TestNullableFields` |
| AIConversation-AIMessage | `TestAIConversationMessages` |
| 外键约束 | `TestForeignKeyConstraints` |
| 多对多增查删 | `TestManyToManyAssociationLifecycle` |
| 注册事务回滚 | `TestCreateWithRoleTransaction` |
| AI 消息并发序号 | `TestAppendMessageSequenceConcurrent` |
| DSN UTC/超时归一化 | `TestNormalizeDSNForcesUTCAndTimeout` |

> `schema_consistency_test.go` 直接解析 migration 的 `CREATE TABLE`，逐表比较列顺序、类型、非空、默认值、
> 自增、索引（唯一性与排序）、外键名与级联动作；改表后该测试会先失败，提醒同步模型与 migration。

## 6. 本地运行

```bash
# 1) 建库并执行迁移（二选一）
#    A. 项目 compose（会启动 MySQL/Redis 并执行 migrate）
docker compose -f deploy/docker-compose.yml up -d mysql redis migrate

#    B. 已有 MySQL：先建库，再用 migrate CLI
mysql -uroot -p -e 'CREATE DATABASE IF NOT EXISTS ai_health CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;'
migrate -path backend/migrations -database 'mysql://root:your_password@tcp(127.0.0.1:3306)/ai_health?multiStatements=true' up

# 2) 注入敏感配置（configs/config.yaml 中的 dsn/secret 默认为空）
export AIH_MYSQL_DSN='root:your_password@tcp(127.0.0.1:3306)/ai_health?charset=utf8mb4&parseTime=True&loc=UTC'
export AIH_JWT_SECRET='please-change-me-to-at-least-32-bytes'
# 可选：AI 默认 mock，无需密钥；接真实模型见 README

# 3) 启动
cd backend && go run ./cmd/api
```
