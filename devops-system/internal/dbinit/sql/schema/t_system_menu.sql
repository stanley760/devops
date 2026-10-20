-- t_system_menu menu table (global table, no tenant_id, aligned with yudao).
-- type: 1 directory 2 menu 3 button. API authorization follows the "permission identifier" model (Casbin decides by permission),
-- so api_path/api_method are not attached.
CREATE TABLE IF NOT EXISTS t_system_menu (
    id             BIGINT       NOT NULL,
    creator        VARCHAR(64)  NULL,
    create_time    DATETIME(3)  NULL,
    updater        VARCHAR(64)  NULL,
    update_time    DATETIME(3)  NULL,
    deleted_at     DATETIME(3)  NULL,
    name           VARCHAR(50)  NOT NULL,
    permission     VARCHAR(100) NULL,
    type           TINYINT      NOT NULL,
    sort           BIGINT       NULL,
    parent_id      BIGINT       NULL DEFAULT 0,
    path           VARCHAR(200) NULL,
    icon           VARCHAR(100) NULL,
    component      VARCHAR(255) NULL,
    component_name VARCHAR(255) NULL,
    status         TINYINT      NOT NULL DEFAULT 1,
    visible        BOOLEAN      NOT NULL DEFAULT TRUE,
    keep_alive     BOOLEAN      NOT NULL DEFAULT FALSE,
    always_show    BOOLEAN      NOT NULL DEFAULT TRUE,
    PRIMARY KEY (id),
    KEY idx_t_system_menu_deleted_at (deleted_at),
    KEY idx_t_system_menu_parent_id (parent_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
