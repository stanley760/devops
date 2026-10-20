-- t_system_dept initial data: tenant 1 (default) departments 100=devops(root) → 101 tech, 102 ops.
-- Explicit ID + tenant_id. ON DUPLICATE KEY UPDATE id = id makes repeated runs side-effect free.
INSERT INTO t_system_dept
    (id, tenant_id, creator, create_time, updater, update_time, name, parent_id, sort, status)
VALUES
    (100, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, 'devops', 0,   0, 1),
    (101, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '技术部', 100,  1, 1),
    (102, 1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, '运维部', 100,  2, 1)
ON DUPLICATE KEY UPDATE id = id;
