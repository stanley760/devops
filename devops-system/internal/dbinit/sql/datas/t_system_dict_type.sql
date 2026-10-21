-- t_system_dict_type initial data: tenant 1 (default) dictionary types.
-- sys_captcha_type is the captcha type value set: the login captcha type is read from
-- t_system_config (sys.captcha.type) and must be one of these values.
-- Explicit ID + tenant_id. ON DUPLICATE KEY UPDATE id = id makes repeated runs side-effect free.
INSERT INTO t_system_dict_type
    (id, tenant_id, creator, create_time, updater, update_time, name, type, status, remark)
VALUES
    (1, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '系统状态',   'sys_status',   1, '通用状态字典'),
    (2, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '性别',       'sys_sex',      1, '性别字典'),
    (3, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '用户类型',   'sys_user_type', 1, '用户类型字典'),
    (4, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '菜单类型',   'sys_menu_type',  1, '菜单类型字典'),
    (5, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '数据范围',   'sys_data_scope', 1, '角色数据范围字典'),
    (6, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '验证码类型', 'sys_captcha_type', 1, '登录/注册验证码类型：取值范围字典')
ON DUPLICATE KEY UPDATE id = id;
