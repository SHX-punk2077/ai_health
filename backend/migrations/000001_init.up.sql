-- 000001_init.up.sql
-- 由 ai_health_mysql8_init.sql 的建表段落整理而来，仅包含 CREATE TABLE。
-- 注意：执行迁移的 DSN 必须带 multiStatements=true。
-- 表顺序保证被引用表先建，外键才能成功。

CREATE TABLE sys_user (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '用户ID',
  username VARCHAR(50) NOT NULL COMMENT '用户名',
  password_hash VARCHAR(255) DEFAULT NULL COMMENT '密码哈希，不存储明文密码；第三方登录用户可为空',
  nickname VARCHAR(50) DEFAULT NULL COMMENT '昵称',
  phone VARCHAR(20) DEFAULT NULL COMMENT '手机号',
  email VARCHAR(100) DEFAULT NULL COMMENT '邮箱',
  avatar VARCHAR(500) DEFAULT NULL COMMENT '头像URL',
  status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '账号状态：1正常，0禁用',
  last_login_at DATETIME(3) DEFAULT NULL COMMENT '最后登录时间',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  deleted_at DATETIME(3) DEFAULT NULL COMMENT '软删除时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_sys_user_username (username),
  UNIQUE KEY uk_sys_user_email (email),
  UNIQUE KEY uk_sys_user_phone (phone),
  KEY idx_sys_user_status (status),
  KEY idx_sys_user_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci ROW_FORMAT=DYNAMIC
COMMENT='系统用户表';

CREATE TABLE sys_oauth_identity (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '第三方身份ID',
  user_id BIGINT UNSIGNED NOT NULL COMMENT '关联用户ID',
  provider VARCHAR(30) NOT NULL COMMENT '第三方平台：wechat等',
  open_id VARCHAR(128) NOT NULL COMMENT '第三方平台用户标识',
  union_id VARCHAR(128) DEFAULT NULL COMMENT '微信开放平台UnionID，可为空',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '绑定时间',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_oauth_provider_open_id (provider, open_id),
  KEY idx_oauth_user_id (user_id),
  KEY idx_oauth_union_id (union_id),
  CONSTRAINT fk_oauth_user FOREIGN KEY (user_id) REFERENCES sys_user(id)
    ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci ROW_FORMAT=DYNAMIC
COMMENT='第三方登录身份绑定表，例如微信登录';

-- ============================================================
-- 2. RBAC 权限
-- ============================================================

CREATE TABLE sys_role (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '角色ID',
  role_code VARCHAR(50) NOT NULL COMMENT '角色编码，如 super_admin、admin、user',
  role_name VARCHAR(50) NOT NULL COMMENT '角色名称',
  description VARCHAR(255) DEFAULT NULL COMMENT '角色描述',
  status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态：1启用，0禁用',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_sys_role_code (role_code),
  KEY idx_sys_role_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci ROW_FORMAT=DYNAMIC
COMMENT='角色表';

CREATE TABLE sys_menu (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '菜单ID',
  parent_id BIGINT UNSIGNED DEFAULT NULL COMMENT '父菜单ID，根节点为NULL',
  menu_type TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '类型：1目录，2菜单，3按钮/权限',
  name VARCHAR(100) NOT NULL COMMENT '路由名称或菜单标识',
  title VARCHAR(100) NOT NULL COMMENT '菜单标题',
  path VARCHAR(255) DEFAULT NULL COMMENT '前端路由路径',
  component VARCHAR(255) DEFAULT NULL COMMENT '前端组件路径',
  redirect VARCHAR(255) DEFAULT NULL COMMENT '重定向地址',
  icon VARCHAR(100) DEFAULT NULL COMMENT '菜单图标',
  permission VARCHAR(100) DEFAULT NULL COMMENT '后端权限标识，如 user:create',
  sort_order INT NOT NULL DEFAULT 0 COMMENT '排序值，越小越靠前',
  is_hidden TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否隐藏：1是，0否',
  is_enabled TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否启用：1是，0否',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_sys_menu_name (name),
  KEY idx_sys_menu_parent_sort (parent_id, sort_order),
  KEY idx_sys_menu_permission (permission),
  CONSTRAINT fk_sys_menu_parent FOREIGN KEY (parent_id) REFERENCES sys_menu(id)
    ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci ROW_FORMAT=DYNAMIC
COMMENT='菜单与权限资源表';

CREATE TABLE sys_user_role (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  user_id BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
  role_id BIGINT UNSIGNED NOT NULL COMMENT '角色ID',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '授权时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_sys_user_role (user_id, role_id),
  KEY idx_sys_user_role_role_id (role_id),
  CONSTRAINT fk_sys_user_role_user FOREIGN KEY (user_id) REFERENCES sys_user(id)
    ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT fk_sys_user_role_role FOREIGN KEY (role_id) REFERENCES sys_role(id)
    ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci ROW_FORMAT=DYNAMIC
COMMENT='用户角色关联表';

CREATE TABLE sys_role_menu (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  role_id BIGINT UNSIGNED NOT NULL COMMENT '角色ID',
  menu_id BIGINT UNSIGNED NOT NULL COMMENT '菜单/权限ID',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '授权时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_sys_role_menu (role_id, menu_id),
  KEY idx_sys_role_menu_menu_id (menu_id),
  CONSTRAINT fk_sys_role_menu_role FOREIGN KEY (role_id) REFERENCES sys_role(id)
    ON DELETE CASCADE ON UPDATE CASCADE,
  CONSTRAINT fk_sys_role_menu_menu FOREIGN KEY (menu_id) REFERENCES sys_menu(id)
    ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci ROW_FORMAT=DYNAMIC
COMMENT='角色菜单关联表';

-- ============================================================
-- 3. 健康档案与历史记录
-- 设计：health_profile 保存当前档案；health_record 保存每次测量快照。
-- 单位约定：身高cm、体重kg、血糖mmol/L、血脂mmol/L、
--          血压mmHg、心率次/分、饮水量ml、睡眠时长小时。
-- ============================================================

CREATE TABLE health_profile (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '健康档案ID',
  user_id BIGINT UNSIGNED NOT NULL COMMENT '所属用户ID，一个用户一份当前档案',
  real_name VARCHAR(50) DEFAULT NULL COMMENT '真实姓名或档案姓名',
  age TINYINT UNSIGNED DEFAULT NULL COMMENT '年龄（可选；也可由出生日期计算）',
  birth_date DATE DEFAULT NULL COMMENT '出生日期',
  gender TINYINT UNSIGNED DEFAULT NULL COMMENT '性别：0未知，1男，2女',
  height_cm DECIMAL(5,2) DEFAULT NULL COMMENT '身高，单位cm',
  weight_kg DECIMAL(5,2) DEFAULT NULL COMMENT '体重，单位kg',
  blood_sugar_mmol DECIMAL(6,2) DEFAULT NULL COMMENT '血糖，单位mmol/L',
  systolic_pressure SMALLINT UNSIGNED DEFAULT NULL COMMENT '收缩压，单位mmHg',
  diastolic_pressure SMALLINT UNSIGNED DEFAULT NULL COMMENT '舒张压，单位mmHg',
  total_cholesterol_mmol DECIMAL(6,2) DEFAULT NULL COMMENT '总胆固醇，单位mmol/L；原血脂字段需确认具体指标',
  heart_rate SMALLINT UNSIGNED DEFAULT NULL COMMENT '静息心率，单位次/分',
  vision_left DECIMAL(4,2) DEFAULT NULL COMMENT '左眼视力',
  vision_right DECIMAL(4,2) DEFAULT NULL COMMENT '右眼视力',
  sleep_duration_hours DECIMAL(4,2) DEFAULT NULL COMMENT '平均睡眠时长，单位小时',
  sleep_quality VARCHAR(30) DEFAULT NULL COMMENT '睡眠质量描述',
  smoking_status TINYINT UNSIGNED DEFAULT NULL COMMENT '吸烟：0否，1是，2已戒烟，NULL未知',
  drinking_status TINYINT UNSIGNED DEFAULT NULL COMMENT '饮酒：0否，1是，2已戒酒，NULL未知',
  exercise_habit TINYINT UNSIGNED DEFAULT NULL COMMENT '运动习惯：0无，1偶尔，2规律，NULL未知',
  food_preferences TEXT COMMENT '饮食类型/偏好',
  daily_water_ml INT UNSIGNED DEFAULT NULL COMMENT '每日饮水量，单位ml',
  medical_history TEXT COMMENT '既往病史（敏感健康信息，需严格授权访问）',
  allergy_history TEXT COMMENT '过敏史（敏感健康信息，需严格授权访问）',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_health_profile_user_id (user_id),
  CONSTRAINT fk_health_profile_user FOREIGN KEY (user_id) REFERENCES sys_user(id)
    ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci ROW_FORMAT=DYNAMIC
COMMENT='用户当前健康档案';

CREATE TABLE health_record (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '健康记录ID',
  user_id BIGINT UNSIGNED NOT NULL COMMENT '所属用户ID',
  height_cm DECIMAL(5,2) DEFAULT NULL COMMENT '本次记录身高，单位cm',
  weight_kg DECIMAL(5,2) DEFAULT NULL COMMENT '体重，单位kg',
  blood_sugar_mmol DECIMAL(6,2) DEFAULT NULL COMMENT '血糖，单位mmol/L',
  systolic_pressure SMALLINT UNSIGNED DEFAULT NULL COMMENT '收缩压，单位mmHg',
  diastolic_pressure SMALLINT UNSIGNED DEFAULT NULL COMMENT '舒张压，单位mmHg',
  total_cholesterol_mmol DECIMAL(6,2) DEFAULT NULL COMMENT '总胆固醇，单位mmol/L',
  heart_rate SMALLINT UNSIGNED DEFAULT NULL COMMENT '心率，单位次/分',
  vision_left DECIMAL(4,2) DEFAULT NULL COMMENT '左眼视力',
  vision_right DECIMAL(4,2) DEFAULT NULL COMMENT '右眼视力',
  sleep_duration_hours DECIMAL(4,2) DEFAULT NULL COMMENT '睡眠时长，单位小时',
  sleep_quality VARCHAR(30) DEFAULT NULL COMMENT '睡眠质量描述',
  smoking_status TINYINT UNSIGNED DEFAULT NULL COMMENT '吸烟状态',
  drinking_status TINYINT UNSIGNED DEFAULT NULL COMMENT '饮酒状态',
  exercise_habit TINYINT UNSIGNED DEFAULT NULL COMMENT '运动习惯',
  food_preferences TEXT COMMENT '饮食类型/偏好',
  daily_water_ml INT UNSIGNED DEFAULT NULL COMMENT '每日饮水量，单位ml',
  measured_at DATETIME(3) NOT NULL COMMENT '实际测量时间',
  source VARCHAR(30) NOT NULL DEFAULT 'manual' COMMENT '数据来源：manual/device/import等',
  remark VARCHAR(500) DEFAULT NULL COMMENT '备注',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '录入时间',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  KEY idx_health_record_user_measured (user_id, measured_at DESC),
  KEY idx_health_record_measured (measured_at),
  CONSTRAINT fk_health_record_user FOREIGN KEY (user_id) REFERENCES sys_user(id)
    ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci ROW_FORMAT=DYNAMIC
COMMENT='用户健康测量历史记录/趋势数据';

-- ============================================================
-- 4. 运动信息与疾病注意事项
-- sport_type 作为稳定的字典主表，其他表通过 sport_type_id 关联。
-- ============================================================

CREATE TABLE sport_type (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '运动类型ID',
  type_code VARCHAR(50) NOT NULL COMMENT '运动编码',
  type_name VARCHAR(50) NOT NULL COMMENT '运动名称',
  description VARCHAR(500) DEFAULT NULL COMMENT '运动介绍',
  status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态：1启用，0停用',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_sport_type_code (type_code),
  UNIQUE KEY uk_sport_type_name (type_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci ROW_FORMAT=DYNAMIC
COMMENT='运动类型字典表';

CREATE TABLE sport_info (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '运动建议ID',
  sport_type_id BIGINT UNSIGNED NOT NULL COMMENT '运动类型ID',
  suitable_time VARCHAR(100) DEFAULT NULL COMMENT '适宜运动时间',
  suitable_heart_rate VARCHAR(100) DEFAULT NULL COMMENT '建议心率范围，展示文本；个体化目标应由业务计算',
  suitable_frequency VARCHAR(100) DEFAULT NULL COMMENT '建议运动频率',
  recommended_speed VARCHAR(100) DEFAULT NULL COMMENT '建议速度/强度',
  duration_minutes SMALLINT UNSIGNED DEFAULT NULL COMMENT '建议单次时长，单位分钟',
  intensity VARCHAR(30) DEFAULT NULL COMMENT '运动强度：low/moderate/high等',
  description TEXT COMMENT '运动建议说明',
  status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态：1启用，0停用',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  KEY idx_sport_info_type_status (sport_type_id, status),
  CONSTRAINT fk_sport_info_type FOREIGN KEY (sport_type_id) REFERENCES sport_type(id)
    ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci ROW_FORMAT=DYNAMIC
COMMENT='运动建议信息表';

CREATE TABLE sport_disease_detail (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '注意事项ID',
  sport_type_id BIGINT UNSIGNED NOT NULL COMMENT '运动类型ID',
  disease_name VARCHAR(100) NOT NULL COMMENT '疾病/健康状况名称',
  recommended_method VARCHAR(1000) DEFAULT NULL COMMENT '运动方法或建议',
  precautions VARCHAR(1000) DEFAULT NULL COMMENT '注意事项',
  risk_level TINYINT UNSIGNED DEFAULT NULL COMMENT '提示级别：1一般，2谨慎，3需专业评估',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '更新时间',
  PRIMARY KEY (id),
  KEY idx_sport_disease_type (sport_type_id),
  KEY idx_sport_disease_name (disease_name),
  CONSTRAINT fk_sport_disease_type FOREIGN KEY (sport_type_id) REFERENCES sport_type(id)
    ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci ROW_FORMAT=DYNAMIC
COMMENT='运动与疾病注意事项关联表';

-- ============================================================
-- 5. AI 对话持久化
-- 实时通信由 Gin/ 层处理；消息内容持久化到 MySQL。
-- ============================================================

CREATE TABLE ai_conversation (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '会话ID',
  user_id BIGINT UNSIGNED NOT NULL COMMENT '所属用户ID',
  title VARCHAR(150) NOT NULL DEFAULT '新对话' COMMENT '会话标题',
  status TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态：1正常，0归档/关闭',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT '最后活动时间',
  deleted_at DATETIME(3) DEFAULT NULL COMMENT '软删除时间',
  PRIMARY KEY (id),
  KEY idx_ai_conversation_user_updated (user_id, updated_at DESC),
  KEY idx_ai_conversation_deleted (deleted_at),
  CONSTRAINT fk_ai_conversation_user FOREIGN KEY (user_id) REFERENCES sys_user(id)
    ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci ROW_FORMAT=DYNAMIC
COMMENT='AI问答会话表';

CREATE TABLE ai_message (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '消息ID',
  conversation_id BIGINT UNSIGNED NOT NULL COMMENT '所属会话ID',
  message_role VARCHAR(20) NOT NULL COMMENT '消息角色：system/user/assistant/tool',
  content MEDIUMTEXT NOT NULL COMMENT '消息正文',
  model_name VARCHAR(100) DEFAULT NULL COMMENT '模型名称',
  token_count INT UNSIGNED DEFAULT NULL COMMENT 'Token数量（可选）',
  sequence_no INT UNSIGNED NOT NULL COMMENT '会话内消息顺序号',
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT '创建时间',
  PRIMARY KEY (id),
  UNIQUE KEY uk_ai_message_conversation_sequence (conversation_id, sequence_no),
  KEY idx_ai_message_conversation_created (conversation_id, created_at),
  CONSTRAINT fk_ai_message_conversation FOREIGN KEY (conversation_id) REFERENCES ai_conversation(id)
    ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci ROW_FORMAT=DYNAMIC
COMMENT='AI问答消息表';
