-- 000003_rbac_permissions.up.sql
-- RBAC 管理端按钮权限：角色 / 菜单的增删改。
-- 沿用 000002 的约定：只补充 sys_menu 数据，不修改任何表结构。
-- super_admin 与 admin 均获得这 6 个权限位；role:assign 仍按 000002 的决策只授予 super_admin。

INSERT INTO sys_menu
(id, parent_id, menu_type, name, title, path, component, redirect, icon, permission, sort_order, is_hidden, is_enabled)
VALUES
  (18, NULL, 3, 'RoleCreate', '新增角色', NULL, NULL, NULL, NULL, 'role:create', 110, 1, 1),
  (19, NULL, 3, 'RoleUpdate', '修改角色', NULL, NULL, NULL, NULL, 'role:update', 111, 1, 1),
  (20, NULL, 3, 'RoleDelete', '删除角色', NULL, NULL, NULL, NULL, 'role:delete', 112, 1, 1),
  (21, NULL, 3, 'MenuCreate', '新增菜单', NULL, NULL, NULL, NULL, 'menu:create', 113, 1, 1),
  (22, NULL, 3, 'MenuUpdate', '修改菜单', NULL, NULL, NULL, NULL, 'menu:update', 114, 1, 1),
  (23, NULL, 3, 'MenuDelete', '删除菜单', NULL, NULL, NULL, NULL, 'menu:delete', 115, 1, 1);

INSERT INTO sys_role_menu (role_id, menu_id)
SELECT 1, id FROM sys_menu WHERE id BETWEEN 18 AND 23;

INSERT INTO sys_role_menu (role_id, menu_id)
SELECT 2, id FROM sys_menu WHERE id BETWEEN 18 AND 23;
