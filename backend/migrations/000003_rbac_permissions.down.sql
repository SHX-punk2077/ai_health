-- 000003_rbac_permissions.down.sql
DELETE FROM sys_role_menu WHERE menu_id BETWEEN 18 AND 23;
DELETE FROM sys_menu WHERE id BETWEEN 18 AND 23;
