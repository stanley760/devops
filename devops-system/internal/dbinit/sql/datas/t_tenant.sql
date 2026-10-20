-- t_tenant initial data: default tenant tenant default(1).
-- Explicit ID, no ID issuance. ON DUPLICATE KEY UPDATE id = id makes repeated runs side-effect free.
INSERT INTO t_tenant (id, creator, create_time, updater, update_time, name, code, status)
VALUES (1, 'seed', CURRENT_TIMESTAMP, 'seed', CURRENT_TIMESTAMP, 'devops', 'default', 1)
ON DUPLICATE KEY UPDATE id = id;
