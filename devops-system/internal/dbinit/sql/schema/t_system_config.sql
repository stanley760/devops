-- t_system_config system configuration table (tenant table).
-- The composite unique index uk_tenant_config_key(tenant_id,config_key) is declared inline.
CREATE TABLE IF NOT EXISTS t_system_config (
    id           BIGINT        NOT NULL,
    creator      VARCHAR(64)   NULL,
    create_time  DATETIME(3)   NULL,
    updater      VARCHAR(64)   NULL,
    update_time  DATETIME(3)   NULL,
    deleted_at   DATETIME(3)   NULL,
    tenant_id    BIGINT        NOT NULL,
    config_key   VARCHAR(100)  NOT NULL,
    config_name  VARCHAR(100)  NOT NULL,
    config_value VARCHAR(1000) NULL,
    type         TINYINT       NOT NULL DEFAULT 2,
    visible      TINYINT(1)    NOT NULL DEFAULT TRUE,
    remark       VARCHAR(500)  NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_tenant_config_key (tenant_id, config_key),
    KEY idx_t_system_config_deleted_at (deleted_at),
    KEY idx_t_system_config_tenant_id (tenant_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
