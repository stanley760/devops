package main

func dataMiddlewarePanels() []panel {
	var panels []panel

	panels = append(panels, timeseriesPanel(1,
		"MySQL connections",
		"Total, running and cached threads on the MySQL server.",
		"short", false, nil,
		target{RefID: "A", Expr: `mysql_global_status_threads_connected{job="mysqld"}`, LegendFormat: "connected"},
		target{RefID: "B", Expr: `mysql_global_status_threads_running{job="mysqld"}`, LegendFormat: "running"},
		target{RefID: "C", Expr: `mysql_global_status_threads_cached{job="mysqld"}`, LegendFormat: "cached"}))
	panels = append(panels, timeseriesPanel(2,
		"MySQL connection usage (%)",
		"Connected threads as a share of max_connections.",
		"percent", true, []float64{70, 85},
		target{RefID: "A", Expr: `100 * mysql_global_status_threads_connected{job="mysqld"} / mysql_global_variables_max_connections{job="mysqld"}`, LegendFormat: "usage"}))
	panels = append(panels, timeseriesPanel(3,
		"MySQL query rate",
		"Queries executed per second on the MySQL server.",
		"reqps", false, nil,
		target{RefID: "A", Expr: `rate(mysql_global_status_queries{job="mysqld"}[5m])`, LegendFormat: "queries"}))
	panels = append(panels, timeseriesPanel(4,
		"MySQL slow queries",
		"Rate of queries that exceeded long_query_time.",
		"short", false, nil,
		target{RefID: "A", Expr: `rate(mysql_global_status_slow_queries{job="mysqld"}[5m])`, LegendFormat: "slow queries"}))
	panels = append(panels, timeseriesPanel(5,
		"MySQL aborted connections",
		"Rates of failed connection attempts and aborted client connections.",
		"short", false, nil,
		target{RefID: "A", Expr: `rate(mysql_global_status_aborted_connects{job="mysqld"}[5m])`, LegendFormat: "aborted connects"},
		target{RefID: "B", Expr: `rate(mysql_global_status_aborted_clients{job="mysqld"}[5m])`, LegendFormat: "aborted clients"}))

	panels = append(panels, timeseriesPanel(6,
		"SQL pool saturation (%)",
		"In-use connections as a share of MaxOpenConns per database.",
		"percent", true, []float64{70, 90},
		target{RefID: "A", Expr: `100 * go_sql_in_use_connections{job="devops-services",service_name=~"$service",deployment_environment=~"$env"} / go_sql_max_open_connections{job="devops-services",service_name=~"$service",deployment_environment=~"$env"}`, LegendFormat: "{{service_name}} {{db_name}}"}))
	panels = append(panels, timeseriesPanel(7,
		"SQL pool connections",
		"Open, in-use and idle connections in the application's SQL pool.",
		"short", false, nil,
		target{RefID: "A", Expr: `go_sql_open_connections{job="devops-services",service_name=~"$service",deployment_environment=~"$env"}`, LegendFormat: "{{service_name}} open"},
		target{RefID: "B", Expr: `go_sql_in_use_connections{job="devops-services",service_name=~"$service",deployment_environment=~"$env"}`, LegendFormat: "{{service_name}} in use"},
		target{RefID: "C", Expr: `go_sql_idle_connections{job="devops-services",service_name=~"$service",deployment_environment=~"$env"}`, LegendFormat: "{{service_name}} idle"}))
	panels = append(panels, timeseriesPanel(8,
		"SQL pool wait rate",
		"Rate of goroutines waiting for a SQL connection and the time they spend waiting.",
		"short", false, nil,
		target{RefID: "A", Expr: `rate(go_sql_wait_count_total{job="devops-services",service_name=~"$service",deployment_environment=~"$env"}[5m])`, LegendFormat: "waits/s"},
		target{RefID: "B", Expr: `rate(go_sql_wait_duration_seconds_total{job="devops-services",service_name=~"$service",deployment_environment=~"$env"}[5m])`, LegendFormat: "wait seconds/s"}))

	panels = append(panels, timeseriesPanel(9,
		"Cache hit rate (%)",
		"Application-level cache hit rate from devops_cache_operations_total. The cache label is a logical name, never a raw key.",
		"percent", true, []float64{30, 50},
		target{RefID: "A", Expr: `100 * sum by (cache) (rate(devops_cache_operations_total{job="devops-services",service_name=~"$service",deployment_environment=~"$env",result="hit"}[5m])) / (sum by (cache) (rate(devops_cache_operations_total{job="devops-services",service_name=~"$service",deployment_environment=~"$env",result="hit"}[5m])) + sum by (cache) (rate(devops_cache_operations_total{job="devops-services",service_name=~"$service",deployment_environment=~"$env",result="miss"}[5m])))`, LegendFormat: "{{cache}}"}))
	panels = append(panels, timeseriesPanel(10,
		"Cache operations (hit / miss)",
		"Application cache lookup rate split by result.",
		"reqps", false, nil,
		target{RefID: "A", Expr: `sum by (cache, result) (rate(devops_cache_operations_total{job="devops-services",service_name=~"$service",deployment_environment=~"$env"}[5m]))`, LegendFormat: "{{cache}} {{result}}"}))

	panels = append(panels, timeseriesPanel(11,
		"Redis keyspace hit rate (%)",
		"Redis server keyspace hit rate, distinct from the application cache hit rate.",
		"percent", true, []float64{50, 80},
		target{RefID: "A", Expr: `100 * rate(redis_keyspace_hits_total{job="redis"}[5m]) / (rate(redis_keyspace_hits_total{job="redis"}[5m]) + rate(redis_keyspace_misses_total{job="redis"}[5m]))`, LegendFormat: "keyspace hit rate"}))
	panels = append(panels, timeseriesPanel(12,
		"Redis memory usage",
		"Memory used by the Redis server, with configured maxmemory for reference.",
		"bytes", false, nil,
		target{RefID: "A", Expr: `redis_memory_used_bytes{job="redis"}`, LegendFormat: "used"},
		target{RefID: "B", Expr: `redis_config_maxmemory{job="redis"}`, LegendFormat: "maxmemory"}))
	panels = append(panels, timeseriesPanel(13,
		"Redis evictions and expirations",
		"Rate of keys evicted for lack of memory and keys expired by TTL.",
		"short", false, nil,
		target{RefID: "A", Expr: `rate(redis_evicted_keys_total{job="redis"}[5m])`, LegendFormat: "evicted/s"},
		target{RefID: "B", Expr: `rate(redis_expired_keys_total{job="redis"}[5m])`, LegendFormat: "expired/s"}))
	panels = append(panels, timeseriesPanel(14,
		"Redis clients",
		"Connected and blocked clients on the Redis server.",
		"short", false, nil,
		target{RefID: "A", Expr: `redis_connected_clients{job="redis"}`, LegendFormat: "connected"},
		target{RefID: "B", Expr: `redis_blocked_clients{job="redis"}`, LegendFormat: "blocked"}))
	panels = append(panels, statPanel(15,
		"Redis uptime",
		"Seconds since the Redis server started.",
		"s", nil,
		target{RefID: "A", Expr: `redis_uptime_in_seconds{job="redis"}`, LegendFormat: "uptime"}))

	panels = append(panels, timeseriesPanel(16,
		"Redis connection pool usage",
		"Used and idle connections in the application's Redis pool, against the pool maximum.",
		"short", false, nil,
		target{RefID: "A", Expr: `db_client_connections_usage{job="devops-services",service_name=~"$service",deployment_environment=~"$env",state="used"}`, LegendFormat: "used"},
		target{RefID: "B", Expr: `db_client_connections_usage{job="devops-services",service_name=~"$service",deployment_environment=~"$env",state="idle"}`, LegendFormat: "idle"},
		target{RefID: "C", Expr: `db_client_connections_max{job="devops-services",service_name=~"$service",deployment_environment=~"$env"}`, LegendFormat: "max"}))
	panels = append(panels, timeseriesPanel(17,
		"Redis pool wait and timeout rate",
		"Rate of pool acquisitions that waited and those that timed out.",
		"short", false, nil,
		target{RefID: "A", Expr: `rate(db_client_connections_waits{job="devops-services",service_name=~"$service",deployment_environment=~"$env"}[5m])`, LegendFormat: "waits/s"},
		target{RefID: "B", Expr: `rate(db_client_connections_timeouts{job="devops-services",service_name=~"$service",deployment_environment=~"$env"}[5m])`, LegendFormat: "timeouts/s"}))
	panels = append(panels, timeseriesPanel(18,
		"Redis connection pool reuse rate (%)",
		"Pool connection reuse rate, not application cache hit rate. Those metrics describe different behaviors.",
		"percent", true, []float64{30, 50},
		target{RefID: "A", Expr: `100 * rate(db_client_connections_hits{job="devops-services",service_name=~"$service",deployment_environment=~"$env"}[5m]) / (rate(db_client_connections_hits{job="devops-services",service_name=~"$service",deployment_environment=~"$env"}[5m]) + rate(db_client_connections_misses{job="devops-services",service_name=~"$service",deployment_environment=~"$env"}[5m]))`, LegendFormat: "{{pool_name}}"}))

	g := &grid{}
	for i := range panels {
		panels[i].GridPos = g.next(12, 8)
	}
	return panels
}

func dataMiddlewareDashboard() dashboard {
	d := newDashboard(
		"devops-data-middleware",
		"DevOps database + cache / middleware",
		"MySQL server metrics, application SQL pool, application cache hit rate, Redis server metrics and Redis connection pool.",
		[]string{"devops", "database", "cache"},
	)
	d.Panels = dataMiddlewarePanels()
	return d
}
