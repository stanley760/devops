-- t_system_role_menu role-menu association (tenant table; menus are global, but grants happen per tenant role).
-- Composite primary key, no standalone ID, no ID issuance; physical deletion (no deleted_at), for the same reason as t_system_user_role.
CREATE TABLE IF NOT EXISTS t_system_role_menu (
    tenant_id BIGINT NOT NULL,
    role_id   BIGINT NOT NULL,
    menu_id   BIGINT NOT NULL,
    PRIMARY KEY (tenant_id, role_id, menu_id),
    KEY idx_role_menu_role (role_id),
    KEY idx_role_menu_menu (menu_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
