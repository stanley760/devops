-- t_system_dict_data initial data: tenant 1 (default) dictionary items.
-- Explicit ID + tenant_id. ON DUPLICATE KEY UPDATE id = id makes repeated runs side-effect free.
INSERT INTO t_system_dict_data
    (id, tenant_id, creator, create_time, updater, update_time,
     dict_type, label, value, sort, status, css_class, remark)
VALUES
    -- sys_status
    (1, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, 'sys_status', '正常', '1', 1, 1, 'success', ''),
    (2, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, 'sys_status', '停用', '0', 2, 1, 'danger',  ''),
    -- sys_sex
    (3, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, 'sys_sex', '未知', '0', 1, 1, 'info',    ''),
    (4, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, 'sys_sex', '男',   '1', 2, 1, 'primary', ''),
    (5, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, 'sys_sex', '女',   '2', 3, 1, 'warning', ''),
    -- sys_user_type
    (6, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, 'sys_user_type', '普通用户', '1', 1, 1, '', ''),
    (7, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, 'sys_user_type', '管理员',   '2', 2, 1, '', ''),
    -- sys_menu_type
    (8, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, 'sys_menu_type', '目录', '1', 1, 1, '', ''),
    (9, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, 'sys_menu_type', '菜单', '2', 2, 1, '', ''),
    (10, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, 'sys_menu_type', '按钮', '3', 3, 1, '', ''),
    -- sys_data_scope
    (11, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, 'sys_data_scope', '全部数据',       '1', 1, 1, '', ''),
    (12, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, 'sys_data_scope', '自定义数据',     '2', 2, 1, '', ''),
    (13, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, 'sys_data_scope', '本部门数据',     '3', 3, 1, '', ''),
    (14, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, 'sys_data_scope', '本部门及以下数据', '4', 4, 1, '', ''),
    (15, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, 'sys_data_scope', '仅本人数据',     '5', 5, 1, '', '')
ON DUPLICATE KEY UPDATE id = id;
