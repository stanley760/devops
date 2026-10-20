-- t_system_role initial data: tenant 1 (default) roles 1=super admin (builtin), 2=common role.
-- Explicit ID + tenant_id. ON DUPLICATE KEY UPDATE id = id makes repeated runs side-effect free.
INSERT INTO t_system_role
    (id, tenant_id, creator, create_time, updater, update_time, name, code, sort, data_scope, status, type)
VALUES
    (1, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '超级管理员', 'super_admin', 1, 1, 1, 1),
    (2, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '普通角色',   'common',      2, 2, 1, 2)
ON DUPLICATE KEY UPDATE id = id;
