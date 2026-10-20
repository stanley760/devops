-- t_system_user_role user-role association (tenant table).
-- Composite primary key, no standalone ID, no ID issuance; physical deletion (no deleted_at) —
-- assign/unassign are idempotent operations, and soft deletion would prevent rebuilding the same composite-key row.
CREATE TABLE IF NOT EXISTS t_system_user_role (
    tenant_id BIGINT NOT NULL,
    user_id   BIGINT NOT NULL,
    role_id   BIGINT NOT NULL,
    PRIMARY KEY (tenant_id, user_id, role_id),
    KEY idx_user_role_user (user_id),
    KEY idx_user_role_role (role_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
