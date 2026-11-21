# 数据库结构审查（对应 000001_init.up.sql）

审查对象：`backend/migrations/000001_init.up.sql`（13 张表，283 行）。
原则：**不擅自修改表名、字段、索引、主外键和类型**；以下问题先记录，是否改表由业务确认后再写新 migration。
本次模型层已按现有 DDL 落地，未修改任何表结构。

## 结论

- 13 张表的主键、字符集、存储引擎、索引与外键整体完整；所有外键列都有可用索引
  （显式索引或联合唯一索引的左前缀）。
- 金额/健康指标没有使用 FLOAT/DOUBLE，DECIMAL 标度满足现有单位。
- 需要优先确认的是：软删除与唯一键的冲突、健康敏感字段的访问控制、AI 消息并发生成序号。
  其中 AI 消息并发已在 `AIRepo.AppendMessage` 用会话行锁解决，不依赖改表。
- 其余问题多为业务语义和后续扩展，不影响当前 13 张表的模型映射。

## 发现的问题与建议

| 级别 | 位置 | 问题 | 影响 | 建议 |
| --- | --- | --- | --- | --- |
| P1 | `sys_user` 第 18–22 行 | `deleted_at` 软删与 `uk_sys_user_username/email/phone` 唯一键共存 | 软删后用户名/手机号/邮箱仍被占用，既不能重新注册，也不能直接恢复同名账号 | 明确策略：若保留占位，文档写清“注销后不可复用”；若允许复用，增加仅对未删除行生效的函数唯一索引（MySQL 8），或删除时把三列改写为 `deleted_<id>` |
| P2 | `sys_user` 第 11–12、21–22 行 | 可空唯一列允许重复 NULL，但空串 `''` 会参与唯一冲突 | 注册时若把空邮箱/手机号写成空串，第二个用户会撞唯一键 | DTO 校验后统一把空串转 NULL；仓储层写入前归一化，禁止把 `""` 写入这三列 |
| P2 | `sys_oauth_identity` 第 37 行 | 唯一键只有 `(provider, open_id)`，没有 `(user_id, provider)` | 同一用户可绑定同一平台的多个账号；是否允许取决于产品 | 若“一人一平台一账号”，补 `UNIQUE(user_id, provider)`；否则在绑定服务里显式限制 |
| P2 | `sys_menu` 第 83–84 行 | `parent_id` 自关联为 `ON DELETE SET NULL` | 删除父菜单会把子菜单提升为根节点，而不是级联删除；树结构可能变成多个孤立根 | 明确菜单删除语义；需要级联时由 service 递归删除或改外键动作（改表需新 migration）。应用层还要禁止 `parent_id = id` 和环 |
| P2 | `health_profile` 第 129–130 行 | `age` 与 `birth_date` 同时存在 | 两个字段可能不一致；年龄会随时间变化 | 以 `birth_date` 为唯一事实来源，`age` 仅作快照或改为生成列；至少规定写入时二者同步 |
| P2 | `health_profile` 第 137 行 | 血脂只有 `total_cholesterol_mmol`，注释也标注“原血脂字段需确认具体指标” | 无法表达 HDL/LDL/甘油三酯，趋势分析维度不足 | 与需求方确认；若需要完整血脂四项，新增列或拆 `health_metric` 表（新 migration） |
| P1 | `health_profile` 第 148–149 行 | `medical_history`、`allergy_history` 为明文 TEXT，属敏感健康信息 | 数据库泄露/日志打印/越权查询会直接暴露病史 | 应用层加密（如 AES-GCM）或使用列级加密；严格按 user_id 鉴权、禁止全表导出、日志脱敏；增加访问审计 |
| P2 | `health_record` 第 159–189 行 | 历史记录只关联 `user_id`，没有 `profile_id`，也没有医疗史/过敏史快照 | 档案被修改后，无法还原某次测量当时的完整健康背景；`HealthProfile.Records` 只能按 user_id 关联 | 若只需测量指标，保持现状；若需要“当时档案快照”，增加 `profile_id` 或 `snapshot_json`（新 migration） |
| P2 | `ai_conversation` 第 259 行、`ai_message` 第 268–282 行 | 会话软删，但消息没有 `deleted_at` | 直接按 `conversation_id` 查消息仍能读到已软删会话的内容 | 所有消息查询 JOIN `ai_conversation` 并过滤 `deleted_at IS NULL`；或删除会话时同步软删消息（需加列） |
| P1（已缓解） | `ai_message` 第 275、278 行 | `sequence_no` 需应用层计算 `MAX+1`，并发会撞唯一键 | 高并发下出现 1062，消息写入失败 | 已实现 `AIRepo.AppendMessage`：事务内 `SELECT ... FOR UPDATE` 锁会话行再取号；唯一索引保留为最终防线 |
| P3 | `sport_info` 第 214 行 | `suitable_heart_rate` 是展示文本，不是数值区间 | 无法直接用于计算或筛查 | 保持“仅展示”定位；个体化心率由业务按年龄/静息心率计算，不把该字段当医疗处方 |
| P3 | `sport_disease_detail` 第 230–245 行 | `disease_name` 直接存字符串，没有疾病字典表 | 同一疾病多种写法会重复，统计困难 | 当前数据量小可接受；需要规范化时增加 `disease` 字典表和 `disease_id` |
| P3 | `sys_user` 第 14 行 | `status` 约定 1 正常 / 0 禁用，但代码 `IsDisabled()` 用 `== 0` 判断 | 非法值（如 2）会被当成正常 | 写入侧校验只允许 0/1；读取侧可改为 `status != 1` 更防御 |

## 已确认无需修改的点

- 所有表都有主键；`sys_user_role`、`sys_role_menu` 使用代理主键 + 联合唯一，适合 GORM 的 `SetupJoinTable`。
- 外键动作与业务语义基本一致：用户删除级联清理个人数据；运动字典被引用时 RESTRICT，避免误删。
- `health_record` 的 `(user_id, measured_at DESC)` 索引覆盖“按用户取最近记录”的主查询；
  `ai_message` 的 `(conversation_id, sequence_no)` 唯一索引与 `(conversation_id, created_at)` 索引覆盖会话消息读取。
- `DATETIME(3)` 毫秒精度、`utf8mb4_0900_ai_ci`、`InnoDB`、`ROW_FORMAT=DYNAMIC` 符合 MySQL 8 默认与中文场景。
- 只有 `sys_user`、`ai_conversation` 使用软删除，符合“用户与 AI 会话可恢复、健康/运动数据物理删除”的约定。

## 本次未做的修改

- 未改任何 `CREATE TABLE`、索引或外键。
- 未新增 `deleted_at` 到其他表。
- 未把 `health_record` 改成按 `profile_id` 关联；当前模型用 `user_id` 关联，与现有 SQL 一致。
- 上述 P1/P2 项如需落表，请先确认业务语义，再新增 `000003_*.up.sql` / `.down.sql`，并同步 entity 与 `schema_consistency_test.go`。
