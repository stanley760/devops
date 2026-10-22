-- t_system_menu initial data (global table): the full menu master set, 44 rows.
-- Directories/menus 1-9, buttons = parent menu ID*10 + sequence number; buttons have always_show=FALSE.
-- Explicit ID, no ID issuance. ON DUPLICATE KEY UPDATE id = id makes repeated runs side-effect free.
INSERT INTO t_system_menu
    (id, creator, create_time, updater, update_time,
     name, permission, type, sort, parent_id, path, icon, component,
     status, visible, always_show)
VALUES
    (1,  'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '系统管理', '', 1, 1, 0, '/system', 'ep:setting', '', 1, TRUE, TRUE),
    (2,  'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '用户管理', 'system:user:list',   2, 1, 1, 'user',  'ep:user',            'system/user/index',  1, TRUE, TRUE),
    (3,  'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '角色管理', 'system:role:list',   2, 2, 1, 'role',  'ep:key',             'system/role/index',  1, TRUE, TRUE),
    (4,  'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '菜单管理', 'system:menu:list',   2, 3, 1, 'menu',  'ep:menu',            'system/menu/index',  1, TRUE, TRUE),
    (5,  'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '部门管理', 'system:dept:list',   2, 4, 1, 'dept',  'ep:office-building', 'system/dept/index',  1, TRUE, TRUE),
    (6,  'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '岗位管理', 'system:post:list',   2, 5, 1, 'post',  'ep:postcard',        'system/post/index',  1, TRUE, TRUE),
    (7,  'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '租户管理', '',                   2, 6, 1, 'tenant', 'ep:coin',            'platform/tenant/index', 1, TRUE, TRUE),
    (8,  'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '登录日志', 'system:loginlog:list',   2, 7, 1, 'loginlog', 'ep:document', 'system/loginlog/index', 1, TRUE, TRUE),
    (9,  'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '操作日志', 'system:operationlog:list',   2, 8, 1, 'operationlog', 'ep:document', 'system/operationlog/index', 1, TRUE, TRUE),
    (10, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '字典管理', 'system:dict:list',   2, 9, 1, 'dict',  'ep:collection',           'system/dict/index',  1, TRUE, TRUE),
    (11, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '系统配置', 'system:config:list', 2, 10, 1, 'config', 'ep:sliders',            'system/config/index', 1, TRUE, TRUE),
    (21, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '用户查询', 'system:user:query',          3, 1, 2, '', '', '', 1, TRUE, FALSE),
    (22, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '用户新增', 'system:user:create',         3, 2, 2, '', '', '', 1, TRUE, FALSE),
    (23, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '用户修改', 'system:user:update',         3, 3, 2, '', '', '', 1, TRUE, FALSE),
    (24, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '用户删除', 'system:user:delete',         3, 4, 2, '', '', '', 1, TRUE, FALSE),
    (25, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '重置密码', 'system:user:update-password', 3, 5, 2, '', '', '', 1, TRUE, FALSE),
    (31, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '角色查询', 'system:role:query',          3, 1, 3, '', '', '', 1, TRUE, FALSE),
    (32, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '角色新增', 'system:role:create',        3, 2, 3, '', '', '', 1, TRUE, FALSE),
    (33, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '角色修改', 'system:role:update',         3, 3, 3, '', '', '', 1, TRUE, FALSE),
    (34, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '角色删除', 'system:role:delete',         3, 4, 3, '', '', '', 1, TRUE, FALSE),
    (41, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '菜单查询', 'system:menu:query',          3, 1, 4, '', '', '', 1, TRUE, FALSE),
    (42, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '菜单新增', 'system:menu:create',         3, 2, 4, '', '', '', 1, TRUE, FALSE),
    (43, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '菜单修改', 'system:menu:update',         3, 3, 4, '', '', '', 1, TRUE, FALSE),
    (44, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '菜单删除', 'system:menu:delete',         3, 4, 4, '', '', '', 1, TRUE, FALSE),
    (51, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '部门查询', 'system:dept:query',          3, 1, 5, '', '', '', 1, TRUE, FALSE),
    (52, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '部门新增', 'system:dept:create',         3, 2, 5, '', '', '', 1, TRUE, FALSE),
    (53, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '部门修改', 'system:dept:update',         3, 3, 5, '', '', '', 1, TRUE, FALSE),
    (54, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '部门删除', 'system:dept:delete',         3, 4, 5, '', '', '', 1, TRUE, FALSE),
    (61, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '岗位查询', 'system:post:query',          3, 1, 6, '', '', '', 1, TRUE, FALSE),
    (62, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '岗位新增', 'system:post:create',         3, 2, 6, '', '', '', 1, TRUE, FALSE),
    (63, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '岗位修改', 'system:post:update',         3, 3, 6, '', '', '', 1, TRUE, FALSE),
    (64, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '岗位删除', 'system:post:delete',         3, 4, 6, '', '', '', 1, TRUE, FALSE),
    (81, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '日志删除', 'system:loginlog:delete', 3, 1, 8, '', '', '', 1, TRUE, FALSE),
    (82, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '日志清空', 'system:loginlog:clean',  3, 2, 8, '', '', '', 1, TRUE, FALSE),
    (91, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '日志删除', 'system:operationlog:delete', 3, 1, 9, '', '', '', 1, TRUE, FALSE),
    (92, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '日志清空', 'system:operationlog:clean',  3, 2, 9, '', '', '', 1, TRUE, FALSE),
    (101, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '字典查询', 'system:dict:query',   3, 1, 10, '', '', '', 1, TRUE, FALSE),
    (102, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '字典新增', 'system:dict:create',  3, 2, 10, '', '', '', 1, TRUE, FALSE),
    (103, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '字典修改', 'system:dict:update',  3, 3, 10, '', '', '', 1, TRUE, FALSE),
    (104, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '字典删除', 'system:dict:delete',  3, 4, 10, '', '', '', 1, TRUE, FALSE),
    (111, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '配置查询', 'system:config:query',   3, 1, 11, '', '', '', 1, TRUE, FALSE),
    (112, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '配置新增', 'system:config:create',  3, 2, 11, '', '', '', 1, TRUE, FALSE),
    (113, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '配置修改', 'system:config:update',  3, 3, 11, '', '', '', 1, TRUE, FALSE),
    (114, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '配置删除', 'system:config:delete',  3, 4, 11, '', '', '', 1, TRUE, FALSE)
ON DUPLICATE KEY UPDATE id = id;

-- 菜单重命名：ON DUPLICATE KEY UPDATE id=id 不会更新已有行，
-- 单独用 UPDATE 让「参数配置 → 系统配置」的改名在存量库上也能生效（按 id 定位，可重复执行）。
UPDATE t_system_menu SET name = '系统配置', icon = 'ep:sliders' WHERE id = 11;
