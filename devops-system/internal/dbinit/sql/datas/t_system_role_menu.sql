-- t_system_role_menu initial data: tenant 1 super_admin(1) → all menus granted (49 rows).
-- Menu visibility goes through role_menu; the casbin p row is a domain-wide wildcard (super_admin, 1, *),
-- so new permissions need no extra policy. ON DUPLICATE KEY UPDATE tenant_id = tenant_id makes repeated runs side-effect free.
INSERT INTO t_system_role_menu (tenant_id, role_id, menu_id)
VALUES
    (1, 1, 1), (1, 1, 2), (1, 1, 3), (1, 1, 4), (1, 1, 5), (1, 1, 6), (1, 1, 7),
    (1, 1, 8), (1, 1, 9), (1, 1, 10), (1, 1, 11),
    (1, 1, 21), (1, 1, 22), (1, 1, 23), (1, 1, 24), (1, 1, 25),
    (1, 1, 31), (1, 1, 32), (1, 1, 33), (1, 1, 34), (1, 1, 35),
    (1, 1, 41), (1, 1, 42), (1, 1, 43), (1, 1, 44),
    (1, 1, 51), (1, 1, 52), (1, 1, 53), (1, 1, 54),
    (1, 1, 61), (1, 1, 62), (1, 1, 63), (1, 1, 64),
    (1, 1, 71), (1, 1, 72), (1, 1, 73), (1, 1, 74),
    (1, 1, 81), (1, 1, 82),
    (1, 1, 91), (1, 1, 92),
    (1, 1, 101), (1, 1, 102), (1, 1, 103), (1, 1, 104),
    (1, 1, 111), (1, 1, 112), (1, 1, 113), (1, 1, 114)
ON DUPLICATE KEY UPDATE tenant_id = tenant_id;
