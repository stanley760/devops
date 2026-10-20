-- t_system_dict_type dictionary type table (tenant table).
-- The composite unique index uk_tenant_type(tenant_id,type) is declared inline.
CREATE TABLE IF NOT EXISTS t_system_dict_type (
    id          BIGINT       NOT NULL,
    creator     VARCHAR(64)  NULL,
    create_time DATETIME(3)  NULL,
    updater     VARCHAR(64)  NULL,
    update_time DATETIME(3)  NULL,
    deleted_at  DATETIME(3)  NULL,
    tenant_id   BIGINT       NOT NULL,
    name        VARCHAR(100) NOT NULL,
    type        VARCHAR(100) NOT NULL,
    status      TINYINT      NOT NULL DEFAULT 1,
    remark      VARCHAR(500) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_tenant_type (tenant_id, type),
    KEY idx_t_system_dict_type_deleted_at (deleted_at),
    KEY idx_t_system_dict_type_tenant_id (tenant_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
