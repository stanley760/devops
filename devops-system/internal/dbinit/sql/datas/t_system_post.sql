-- t_system_post initial data: tenant 1 (default) posts 1=ceo, 2=se, 3=ops, 4=user.
-- Explicit ID + tenant_id. ON DUPLICATE KEY UPDATE id = id makes repeated runs side-effect free.
INSERT INTO t_system_post
    (id, tenant_id, creator, create_time, updater, update_time, code, name, sort, status)
VALUES
    (1, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, 'ceo',  '董事长',     1, 1),
    (2, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, 'se',   '架构师',     2, 1),
    (3, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, 'ops',  '运维工程师', 3, 1),
    (4, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, 'user', '普通用户',   4, 1)
ON DUPLICATE KEY UPDATE id = id;
