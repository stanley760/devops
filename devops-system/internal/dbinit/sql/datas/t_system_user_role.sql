-- t_system_user_role initial data: admin(1) → super_admin(1).
-- ON DUPLICATE KEY UPDATE tenant_id = tenant_id makes repeated runs side-effect free.
INSERT INTO t_system_user_role (tenant_id, user_id, role_id)
VALUES (1, 1, 1)
ON DUPLICATE KEY UPDATE tenant_id = tenant_id;
