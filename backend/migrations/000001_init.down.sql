-- 000001_init.down.sql
-- 逆依赖顺序删除，先关外键检查避免删除顺序耦合。
SET FOREIGN_KEY_CHECKS = 0;

DROP TABLE IF EXISTS ai_message;
DROP TABLE IF EXISTS ai_conversation;
DROP TABLE IF EXISTS sport_disease_detail;
DROP TABLE IF EXISTS sport_info;
DROP TABLE IF EXISTS sport_type;
DROP TABLE IF EXISTS health_record;
DROP TABLE IF EXISTS health_profile;
DROP TABLE IF EXISTS sys_role_menu;
DROP TABLE IF EXISTS sys_user_role;
DROP TABLE IF EXISTS sys_menu;
DROP TABLE IF EXISTS sys_role;
DROP TABLE IF EXISTS sys_oauth_identity;
DROP TABLE IF EXISTS sys_user;

SET FOREIGN_KEY_CHECKS = 1;
