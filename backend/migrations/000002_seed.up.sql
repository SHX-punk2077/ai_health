-- 000002_seed.up.sql
-- 基础数据：3 个角色、17 条菜单与授权、5 种运动类型、5 条运动建议、4 条疾病禁忌。
-- 不预置管理员账号：首个管理员请用注册接口建号后手工绑定 super_admin 角色。

-- 注意：不插入管理员用户。请在部署时通过安全的初始化流程创建账号，
-- 并使用 bcrypt/Argon2 生成 password_hash。
-- ============================================================

INSERT INTO sys_role (id, role_code, role_name, description) VALUES
  (1, 'super_admin', '超级管理员', '拥有系统全部管理权限'),
  (2, 'admin', '管理员', '负责日常系统管理'),
  (3, 'user', '普通用户', '普通健康管理用户');

INSERT INTO sys_menu
(id, parent_id, menu_type, name, title, path, component, redirect, icon, permission, sort_order, is_hidden, is_enabled)
VALUES
  (1, NULL, 1, 'Dashboard', '首页', '/', 'Layout', '/dashboard', 'HomeFilled', NULL, 1, 0, 1),
  (2, 1, 2, 'DashboardIndex', '数据概览', '/dashboard', 'dashboard/index', NULL, 'DataLine', 'dashboard:view', 1, 0, 1),
  (3, NULL, 1, 'System', '系统管理', '/system', 'Layout', NULL, 'Setting', NULL, 10, 0, 1),
  (4, 3, 2, 'UserManagement', '用户管理', '/system/users', 'system/user/index', NULL, 'User', 'user:list', 1, 0, 1),
  (5, 3, 2, 'RoleManagement', '角色管理', '/system/roles', 'system/role/index', NULL, 'UserFilled', 'role:list', 2, 0, 1),
  (6, 3, 2, 'MenuManagement', '菜单管理', '/system/menus', 'system/menu/index', NULL, 'Menu', 'menu:list', 3, 0, 1),
  (7, NULL, 1, 'Health', '健康管理', '/health', 'Layout', NULL, 'FirstAidKit', NULL, 20, 0, 1),
  (8, 7, 2, 'HealthProfile', '健康档案', '/health/profile', 'health/profile/index', NULL, 'Document', 'health:profile:read', 1, 0, 1),
  (9, 7, 2, 'HealthRecords', '健康记录', '/health/records', 'health/records/index', NULL, 'TrendCharts', 'health:record:list', 2, 0, 1),
  (10, NULL, 1, 'Sport', '运动管理', '/sport', 'Layout', NULL, 'Basketball', NULL, 30, 0, 1),
  (11, 10, 2, 'SportInfo', '运动建议', '/sport/info', 'sport/info/index', NULL, 'Guide', 'sport:info:list', 1, 0, 1),
  (12, 10, 2, 'SportDisease', '疾病注意事项', '/sport/disease', 'sport/disease/index', NULL, 'Warning', 'sport:disease:list', 2, 0, 1),
  (13, NULL, 2, 'AIChat', 'AI健康问答', '/ai/chat', 'ai/chat/index', NULL, 'ChatDotRound', 'ai:chat:use', 40, 0, 1),
  (14, NULL, 3, 'UserCreate', '新增用户', NULL, NULL, NULL, NULL, 'user:create', 100, 1, 1),
  (15, NULL, 3, 'UserUpdate', '修改用户', NULL, NULL, NULL, NULL, 'user:update', 101, 1, 1),
  (16, NULL, 3, 'UserDelete', '删除用户', NULL, NULL, NULL, NULL, 'user:delete', 102, 1, 1),
  (17, NULL, 3, 'RoleAssign', '分配角色', NULL, NULL, NULL, NULL, 'role:assign', 103, 1, 1);

-- 超级管理员拥有全部已初始化菜单/权限
INSERT INTO sys_role_menu (role_id, menu_id)
SELECT 1, id FROM sys_menu;

-- 管理员拥有日常管理功能，不授予超级管理员角色管理能力以外的隐式权限
INSERT INTO sys_role_menu (role_id, menu_id)
SELECT 2, id FROM sys_menu
WHERE permission IS NULL
   OR permission NOT IN ('role:assign');

-- 普通用户仅拥有用户侧功能
INSERT INTO sys_role_menu (role_id, menu_id)
SELECT 3, id FROM sys_menu
WHERE name IN ('Dashboard', 'DashboardIndex', 'Health', 'HealthProfile',
               'HealthRecords', 'AIChat');

INSERT INTO sport_type (id, type_code, type_name, description) VALUES
  (1, 'walking', '步行', '低门槛有氧运动，可根据个人情况调整速度与时长'),
  (2, 'running', '跑步', '有氧运动，运动强度应结合个人健康状况评估'),
  (3, 'cycling', '骑行', '可选择室内或户外骑行，注意安全与强度控制'),
  (4, 'swimming', '游泳', '全身性运动，注意水域安全及个人身体状况'),
  (5, 'yoga', '瑜伽', '注重柔韧性、平衡和呼吸，动作应量力而行');

INSERT INTO sport_info
(sport_type_id, suitable_time, suitable_heart_rate, suitable_frequency, recommended_speed, duration_minutes, intensity, description)
VALUES
  (1, '根据个人作息选择', '以个人耐受程度为准', '可从每周数次开始', '舒适、可交谈的步行速度', 30, 'low', '从较短时长开始，逐渐增加活动量。'),
  (2, '避开极端天气时段', '需结合年龄及健康状况评估', '根据体能逐步安排', '循序渐进，不追求速度', 20, 'moderate', '初学者可采用跑走结合；有慢性病或不适者先咨询专业人员。'),
  (3, '选择光线和路况良好的时段', '以个人耐受程度为准', '根据体能安排', '保持可控强度', 30, 'moderate', '佩戴头盔，遵守交通规则，注意补水。'),
  (4, '选择有救生设施的场所', '不建议仅凭心率判断强度', '根据体能安排', '以安全和舒适为先', 20, 'moderate', '避免独自游泳；不熟悉水性者应在专业指导下进行。'),
  (5, '选择安静且安全的环境', '通常以呼吸平稳为宜', '可按个人情况安排', '动作平稳、避免疼痛', 30, 'low', '避免强行拉伸，存在关节或脊柱问题时应调整动作。');

INSERT INTO sport_disease_detail
(sport_type_id, disease_name, recommended_method, precautions, risk_level)
VALUES
  (1, '高血压', '从轻到中等强度步行开始，逐渐增加时长', '运动方案应结合血压控制情况制定；出现胸痛、明显气促或头晕应停止并就医', 2),
  (1, '糖尿病', '规律监测血糖，并根据医嘱安排运动', '使用胰岛素或易发生低血糖者应提前制定低血糖预防方案', 2),
  (2, '心血管疾病', '仅在医生评估并允许后选择适宜运动强度', '避免未经评估的高强度运动；出现胸痛、晕厥等症状立即停止并寻求医疗帮助', 3),
  (5, '骨关节疾病', '选择无痛或低冲击动作，必要时在专业人员指导下练习', '避免疼痛范围内的强行拉伸和高风险体式', 2);
