-- t_system_dict_data dictionary item table (tenant table).
-- Items reference their dictionary type by type code (not by id), so the type code
-- is immutable after creation.
CREATE TABLE IF NOT EXISTS t_system_dict_data (
    id          BIGINT       NOT NULL,
    creator     VARCHAR(64)  NULL,
    create_time DATETIME(3)  NULL,
    updater     VARCHAR(64)  NULL,
    update_time DATETIME(3)  NULL,
    deleted_at  DATETIME(3)  NULL,
    tenant_id   BIGINT       NOT NULL,
    dict_type   VARCHAR(100) NOT NULL,
    label       VARCHAR(100) NOT NULL,
    value       VARCHAR(100) NOT NULL,
    sort        BIGINT       NULL,
    status      TINYINT      NOT NULL DEFAULT 1,
    css_class   VARCHAR(100) NULL,
    remark      VARCHAR(500) NULL,
    PRIMARY KEY (id),
    KEY idx_t_system_dict_data_deleted_at (deleted_at),
    KEY idx_t_system_dict_data_tenant_id (tenant_id),
    KEY idx_t_system_dict_data_dict_type (dict_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
