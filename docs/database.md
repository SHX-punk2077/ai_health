# 数据表与代码对应

| 表 | entity | migration | 说明 |
| --- | --- | --- | --- |
| sys_user | `sys_user.go` | 000001 | 软删 `deleted_at` |
| sys_oauth_identity | `sys_oauth_identity.go` | 000001 | provider+open_id 唯一 |
| sys_role | `sys_role.go` | 000001 | role_code 唯一 |
| sys_menu | `sys_menu.go` | 000001 | parent_id 自关联，menu_type 1/2/3 |
| sys_user_role | `sys_user_role.go` | 000001 | 联合唯一 |
| sys_role_menu | `sys_role_menu.go` | 000001 | 联合唯一 |
| health_profile | `health_profile.go` | 000001 | 一人一份 |
| health_record | `health_record.go` | 000001 | 按 measured_at 倒序 |
| sport_type | `sport_type.go` | 000002 | 5 条种子 |
| sport_info | `sport_info.go` | 000002 | 5 条种子 |
| sport_disease_detail | `sport_disease_detail.go` | 000002 | risk_level 1/2/3 |
| ai_conversation | `ai_conversation.go` | 000001 | 软删 |
| ai_message | `ai_message.go` | 000001 | conversation_id+sequence_no 唯一 |

## 类型映射

| MySQL | Go |
| --- | --- |
| BIGINT UNSIGNED | uint64 |
| TINYINT UNSIGNED | uint8 |
| SMALLINT UNSIGNED | uint16 |
| INT UNSIGNED | uint32 |
| DECIMAL(p,s) | *decimal.Decimal |
| DATETIME(3) | time.Time |
| DATE | *time.Time |
| TEXT / MEDIUMTEXT | string |
| 可空列 | 指针 |
