-- t_system_domain_event domain event table (added by the DDD/CQRS refactor).
--
-- Purpose: carries the facts that, after an aggregate state change, "need to be projected
-- into an external read model" (the current projection target is
-- casbin policies). Events and business rows are written in the same transaction, guaranteeing the state change and the pending facts land atomically.
--
-- Projection semantics:
--   - projected_at IS NULL means pending projection; a timestamp is written after a successful projection;
--   - a projection failure does not block the main flow; it only increments attempts and records last_error, compensated by a retry job.
--
-- No seed data (a pure runtime table), so it is not placed in sql/datas.
CREATE TABLE IF NOT EXISTS t_system_domain_event (
    id             BIGINT       NOT NULL,
    creator        VARCHAR(64)  NULL,
    create_time    DATETIME(3)  NULL,
    updater        VARCHAR(64)  NULL,
    update_time    DATETIME(3)  NULL,
    deleted_at     DATETIME(3)  NULL,
    event_name     VARCHAR(64)  NOT NULL,
    aggregate_type VARCHAR(32)  NOT NULL,
    aggregate_id   BIGINT       NOT NULL,
    tenant_id      BIGINT       NOT NULL,
    payload        JSON         NULL,
    occurred_at    DATETIME(3)  NOT NULL,
    projected_at   DATETIME(3)  NULL,
    attempts       INT          NOT NULL DEFAULT 0,
    last_error     VARCHAR(500) NULL,
    PRIMARY KEY (id),
    KEY idx_t_system_domain_event_event_name (event_name),
    KEY idx_t_system_domain_event_aggregate (aggregate_type, aggregate_id),
    KEY idx_t_system_domain_event_tenant_id (tenant_id),
    KEY idx_t_system_domain_event_projected_at (projected_at),
    KEY idx_t_system_domain_event_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
