# 健康数据隐私与安全约束

> 适用范围：`health_profile`、`health_record` 及后续运动建议、AI 会话、RAG 知识库对健康数据的读取。
> 本文记录当前实现、已检查项和仍然存在的风险；本次不实现字段级加密、完整审计或软删除。

## 1. 基本原则

1. **最小必要**：只读取完成当前功能所需的最小字段、最小时间范围和最小记录集。
2. **身份只来自 JWT**：所有健康接口从 JWT 中间件取得 `uid`，请求体/query 中的 `userId` 不作为归属依据。
3. **用户隔离不可绕过**：Repository 的查询、更新、删除都带 `user_id`；管理员角色也不能直接读取其他用户的健康记录。
4. **普通日志不打印健康数据**：不记录请求体、query、完整档案、完整记录或指标数值。
5. **AI 受控读取**：AI 只能通过专门的健康上下文组装层获取必要数据，禁止把完整档案或全部历史记录直接交给第三方模型。
6. **提醒不是诊断**：异常提醒只做工程侧风险提示，不能替代医生诊断，也不能凭单次测量认定疾病。

## 2. 十项检查结果

| # | 检查项 | 当前结论 | 证据 / 说明 |
| --- | --- | --- | --- |
| 1 | 日志是否输出完整健康档案 | 否 | `internal/middleware/logger.go` 只记录 requestId、method、path、status、cost、ip、uid；健康 handler/service/repo 没有打印档案的日志 |
| 2 | 日志是否输出完整健康记录 | 否 | 访问日志不含 body/query；Service 的错误只含指标名，不含测量数值；GORM 日志已参数化 |
| 3 | 错误响应是否泄露 SQL/内部信息 | 否 | `pkg/response.Fail` 只输出 code/message/requestId；`errcode.Error.Detail` 不进入响应；HTTP 测试断言错误响应不含 `select`、`health_record`、`sql`、`mysql` |
| 4 | 健康接口是否存在越权访问 | 未发现 | 健康路由全部经过 JWT 中间件；Handler 只使用 `currentUID(c)`；Repository 查询/更新/删除均带 `user_id` |
| 5 | 是否可通过修改 URL ID 获取他人数据 | 否 | `GET/PUT/DELETE /health/records/:id` 均使用 `WHERE id = ? AND user_id = ?`；不存在与不属于当前用户统一返回 40004 |
| 6 | 健康数据是否进入普通 Redis 缓存 | 当前否 | 健康模块未调用 Redis；`consts.KeyProfile` 虽已声明但未被引用；后续若增加缓存，必须单独评审 TTL、键隔离和脱敏 |
| 7 | 后续 AI 是否可直接读取全部健康数据 | 当前 AI 仍未读取 | `ai_service.go` 仍是 stub；后续必须通过受控上下文组装层，按会话目的取最小字段，不得直接查全表或把完整档案拼进 Prompt |
| 8 | 用户 ID 是否始终来自 JWT | 是 | `middleware/auth.go` 写入 `consts.CtxUID`；健康 DTO 没有 `userId` 字段；更新接口把 `userId/user_id` 视为受保护字段并明确拒绝 |
| 9 | 敏感字段是否出现在调试日志 | 未发现 | 病史、过敏史、备注等字段没有专门日志；GORM 参数化日志只输出占位符，不展开参数值；调试时仍禁止手动打印请求体 |
| 10 | 删除后的备份与保留策略 | **存在风险，尚未建立策略** | `health_record` 为物理删除且没有 `deleted_at`；数据库备份、binlog、慢查询/审计日志可能保留删除前的数据，当前没有保留期限、加密或清理流程 |

## 3. 已实施的加固

### 3.1 访问日志最小化

`backend/internal/middleware/logger.go` 只输出请求元数据，不读取 `c.Request.Body`，也不记录 `RawQuery`。
新增 `backend/internal/middleware/logger_test.go` 的 `TestLoggerDoesNotLogHealthPayload`：
用健康记录请求体、query 和备注构造请求，断言访问日志字段只有白名单键，且不含体重、血糖、备注、token 等值。

注意：该测试约束的是应用访问日志；反向代理、网关、WAF、MySQL general log 不在其覆盖范围内。

### 3.2 错误响应不泄露内部细节

`backend/internal/pkg/response/response.go` 的 `Fail` 只序列化：

```json
{ "code": 40010, "message": "...", "requestId": "..." }
```

`errcode.Error` 的 `Detail` 只用于服务端排查，不会写入 HTTP 响应。现有 HTTP 健康测试对越界数据、
不存在记录和跨用户访问都检查了响应体，不包含 SQL、表名或数据库错误。

### 3.3 GORM 参数化日志

`backend/internal/bootstrap/db.go` 的 `newDBLogger` 设置了：

- `ParameterizedQueries: true`
- 慢查询阈值默认 200ms
- 非 debug 模式只输出 Warn 及以上
- `IgnoreRecordNotFoundError: true`

因此应用 GORM 日志中的 SQL 使用 `?` 占位符，不会展开健康数值。
`backend/internal/bootstrap/db_log_test.go` 的 `TestDBLoggerParameterizesSensitiveValues` 对此做了断言。

限制：参数化只覆盖 GORM 自身日志。MySQL 的 general log、slow query log、performance_schema、
云数据库审计、binlog 仍可能记录实际参数或行数据，需要数据库侧另行配置和授权。

### 3.4 用户归属与越权防护

- 路由：`backend/internal/router/v1_health.go` 的 health group 统一挂 JWT 中间件。
- Handler：`backend/internal/handler/health_handler.go` 使用 `currentUID(c)`；没有从 body/query 取用户 ID。
- Repository：`GetProfile/ListRecords/GetRecord/CreateRecord/DeleteRecord/UpdateRecord/PreviousWeight/
  LatestMetrics/LatestMeasuredAt/Trend` 都显式带 `user_id`。
- 管理员：健康路由没有管理员旁路；RBAC 权限位不改变健康数据的归属条件。
- 记录更新：`user_id、id、created_at、source` 出现即返回 10001，不允许通过更新篡改归属或来源。

### 3.5 当前没有做的内容

- 没有字段级加密、透明数据加密（TDE）或密钥管理。
- 没有软删除、回收站、删除审计或保留期限策略。
- 没有独立的健康数据访问审计表。
- 没有把健康数据写入 Redis 缓存。
- 没有把健康数据发送给任何外部模型；AI 模块仍为 stub。

这些不是“已经安全”的证明，而是本次范围外的明确缺口。

## 4. 存储、日志、备份风险

| 面 | 风险 | 当前状态 | 建议 |
| --- | --- | --- | --- |
| 数据库存储 | 健康数据明文落盘；数据库账号或备份泄露会暴露病史、过敏史、血压、血糖等 | MySQL 8 默认存储；未启用 TDE | 生产启用磁盘/备份加密；限制 DBA 与备份账号；敏感字段评估字段级加密 |
| 应用日志 | 手动调试、异常堆栈或第三方库可能打印请求对象 | 访问日志已最小化；GORM 已参数化；无健康业务日志 | 禁止在健康模块打印 body/entity；日志采集侧配置敏感字段过滤与保留期限 |
| 数据库日志 | general log、slow log、audit、binlog 可能含参数或行数据 | 未在应用层控制 | 生产关闭不必要的 general log；slow log 限制访问；审计日志单独授权与加密 |
| 备份 | 物理删除后，备份仍可能保留已删除记录 | 无备份保留策略 | 明确备份保留期、删除请求与备份的联动、过期销毁和访问审批 |
| Redis | 后续若缓存健康摘要，可能被其他业务或调试命令读取 | 当前未缓存 | 使用独立 key 前缀、短 TTL、按 uid 隔离；禁止缓存完整档案；不写入普通业务缓存 |
| 第三方模型 | Prompt 可能包含完整病史或全部记录 | 当前未接入 | 经受控上下文组装层，只传当前问题所需字段；脱敏/摘要；记录调用目的与授权 |

## 5. AI 与后续模块的约束

后续第五阶段（运动建议）、第六阶段（AI 会话）、第七阶段（RAG）必须遵守：

1. **受控上下文组装层**：新增独立的健康上下文读取组件，输入为 `uid` + 当前任务目的，
   输出为最小字段集合；禁止在 AI Service 中直接 `SELECT * FROM health_profile/health_record`。
2. **目的限定**：运动建议只需要与运动风险相关的必要指标；AI 问答按问题检索必要片段；
   RAG 只索引经过授权和脱敏的知识，不把用户完整健康档案写入共享知识库。
3. **不传完整档案**：不把 `medicalHistory、allergyHistory` 等敏感文本整段拼进第三方 Prompt；
   需要时先做字段级筛选、摘要或用户确认。
4. **不绕过隔离**：AI 工具/函数调用也必须携带当前 JWT 用户身份，不能使用管理员权限跨用户读取。
5. **可解释与可关闭**：向用户说明数据用途；提供不使用健康数据的选项；记录数据使用范围而不是完整内容。
6. **提醒非诊断**：AI 输出涉及异常指标时，必须保留“不是诊断、建议咨询专业人员”的边界。

## 6. 验证方式

```powershell
cd backend
go test ./internal/middleware -run TestLoggerDoesNotLogHealthPayload -count=1
go test ./internal/bootstrap -run TestDBLoggerParameterizesSensitiveValues -count=1
go test ./internal/router -run 'TestHTTPHealth' -count=1
```

完整回归：

```powershell
gofmt -l ./internal ./cmd
go vet ./...
go test ./... -count=1 -p 1
```

测试库使用 `ai_health_test`，Redis 测试固定 DB 15；不得把测试指向开发库或生产库。
