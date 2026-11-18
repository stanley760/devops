package main

func systemAppPanels() []panel {
	var panels []panel

	panels = append(panels, timeseriesPanel(1,
		"Host CPU utilization (%)",
		"Per-CPU utilization of the WSL2/Docker VM, not the Windows host.",
		"percent", true, []float64{80, 95},
		target{RefID: "A", Expr: `100 * (1 - avg(rate(node_cpu_seconds_total{job="node",mode="idle"}[5m])) by (cpu))`, LegendFormat: "cpu{{cpu}}"}))
	panels = append(panels, timeseriesPanel(2,
		"Host memory utilization (%)",
		"MemAvailable share of MemTotal for the WSL2/Docker VM.",
		"percent", true, []float64{80, 95},
		target{RefID: "A", Expr: `100 * (1 - node_memory_MemAvailable_bytes{job="node"} / node_memory_MemTotal_bytes{job="node"})`, LegendFormat: "memory"}))
	panels = append(panels, timeseriesPanel(3,
		"Disk I/O throughput (B/s)",
		"Read and write throughput per device of the WSL2/Docker VM.",
		"Bps", false, nil,
		target{RefID: "A", Expr: `sum by (device) (rate(node_disk_read_bytes_total{job="node"}[5m]))`, LegendFormat: "read {{device}}"},
		target{RefID: "B", Expr: `sum by (device) (rate(node_disk_written_bytes_total{job="node"}[5m]))`, LegendFormat: "write {{device}}"}))
	panels = append(panels, timeseriesPanel(4,
		"Disk I/O saturation (%)",
		"Share of wall-clock time each device spent doing I/O.",
		"percent", true, []float64{70, 90},
		target{RefID: "A", Expr: `100 * rate(node_disk_io_time_seconds_total{job="node"}[5m])`, LegendFormat: "{{device}}"}))
	panels = append(panels, timeseriesPanel(5,
		"Filesystem usage (%)",
		"Used share per mounted filesystem of the WSL2/Docker VM. Read-only and virtual mounts are excluded.",
		"percent", true, []float64{80, 90},
		target{RefID: "A", Expr: `100 * (1 - node_filesystem_avail_bytes{job="node",mountpoint!~"^/(var/lib/kiri|dev|proc|sys|run|tmp)(/|$)"} / node_filesystem_size_bytes{job="node",mountpoint!~"^/(var/lib/kiri|dev|proc|sys|run|tmp)(/|$)"})`, LegendFormat: "{{mountpoint}}"}))
	panels = append(panels, timeseriesPanel(6,
		"Network throughput (B/s)",
		"Receive and transmit throughput per interface of the WSL2/Docker VM; loopback is excluded.",
		"Bps", false, nil,
		target{RefID: "A", Expr: `sum by (device) (rate(node_network_receive_bytes_total{job="node",device!="lo"}[5m]))`, LegendFormat: "rx {{device}}"},
		target{RefID: "B", Expr: `sum by (device) (rate(node_network_transmit_bytes_total{job="node",device!="lo"}[5m]))`, LegendFormat: "tx {{device}}"}))
	panels = append(panels, statPanel(7,
		"Host load (1m)",
		"1-minute load average of the WSL2/Docker VM. Compare against the CPU count.",
		"short", nil,
		target{RefID: "A", Expr: `node_load1{job="node"}`, LegendFormat: "load1"}))
	panels = append(panels, statPanel(8,
		"File descriptor usage (%)",
		"Allocated file descriptors against the system maximum.",
		"percent", []float64{70, 90},
		target{RefID: "A", Expr: `100 * node_filefd_allocated{job="node"} / node_filefd_maximum{job="node"}`, LegendFormat: "fd"}))
	panels = append(panels, statPanel(9,
		"Host uptime",
		"Uptime of the WSL2/Docker VM, derived from node_time_seconds and node_boot_time_seconds.",
		"dtdurations", nil,
		target{RefID: "A", Expr: `node_time_seconds{job="node"} - node_boot_time_seconds{job="node"}`, LegendFormat: "uptime"}))

	panels = append(panels, timeseriesPanel(10,
		"Service process CPU (cores)",
		"Per-service process CPU usage. Metrics carry native service and environment labels.",
		"short", false, nil,
		target{RefID: "A", Expr: `rate(process_cpu_seconds_total{job="devops-services",service_name=~"$service",deployment_environment=~"$env",instance=~"$instance"}[5m])`, LegendFormat: "{{service_name}}"}))
	panels = append(panels, timeseriesPanel(11,
		"Service process memory (RSS)",
		"Resident set size per service.",
		"bytes", false, nil,
		target{RefID: "A", Expr: `process_resident_memory_bytes{job="devops-services",service_name=~"$service",deployment_environment=~"$env",instance=~"$instance"}`, LegendFormat: "{{service_name}}"}))
	panels = append(panels, timeseriesPanel(12,
		"Goroutines per service",
		"Live goroutine count per service. A sustained climb can indicate a leak or blocked dependency.",
		"short", false, nil,
		target{RefID: "A", Expr: `go_goroutines{job="devops-services",service_name=~"$service",deployment_environment=~"$env",instance=~"$instance"}`, LegendFormat: "{{service_name}}"}))
	panels = append(panels, timeseriesPanel(13,
		"Go GC pause (s/s)",
		"Cumulative GC stop-the-world time per second, per service.",
		"s", false, nil,
		target{RefID: "A", Expr: `rate(go_gc_duration_seconds_sum{job="devops-services",service_name=~"$service",deployment_environment=~"$env",instance=~"$instance"}[5m])`, LegendFormat: "{{service_name}}"}))
	panels = append(panels, timeseriesPanel(14,
		"Open file descriptors per service",
		"File descriptors held open by each service process.",
		"short", false, nil,
		target{RefID: "A", Expr: `process_open_fds{job="devops-services",service_name=~"$service",deployment_environment=~"$env",instance=~"$instance"}`, LegendFormat: "{{service_name}}"}))

	panels = append(panels, timeseriesPanel(15,
		"HTTP QPS (by route)",
		"Request rate per business route. The route label uses Gin's route template to keep cardinality bounded.",
		"reqps", false, nil,
		target{RefID: "A", Expr: `sum by (route) (rate(http_server_requests_total{job="devops-services",service_name=~"$service",deployment_environment=~"$env",route=~"$route"}[1m]))`, LegendFormat: "{{route}}"}))
	panels = append(panels, timeseriesPanel(16,
		"HTTP 5xx error rate (%)",
		"Share of responses with a 5xx status. Thresholds: 1% warning, 5% critical.",
		"percent", true, []float64{1, 5},
		target{RefID: "A", Expr: `100 * sum(rate(http_server_requests_total{job="devops-services",service_name=~"$service",deployment_environment=~"$env",route=~"$route",status=~"5.."}[5m])) / sum(rate(http_server_requests_total{job="devops-services",service_name=~"$service",deployment_environment=~"$env",route=~"$route"}[5m]))`, LegendFormat: "5xx rate"}))
	panels = append(panels, timeseriesPanel(17,
		"HTTP status code distribution",
		"Request rate grouped by HTTP status code.",
		"reqps", false, nil,
		target{RefID: "A", Expr: `sum by (status) (rate(http_server_requests_total{job="devops-services",service_name=~"$service",deployment_environment=~"$env",route=~"$route"}[5m]))`, LegendFormat: "{{status}}"}))
	panels = append(panels, timeseriesPanel(18,
		"HTTP latency quantiles (in-house histogram)",
		"P50/P95/P99 by business route from devops_http_request_duration_seconds.",
		"s", false, nil,
		target{RefID: "A", Expr: `histogram_quantile(0.50, sum by (le, route) (rate(devops_http_request_duration_seconds_bucket{job="devops-services",service_name=~"$service",deployment_environment=~"$env",route=~"$route"}[5m])))`, LegendFormat: "p50 {{route}}"},
		target{RefID: "B", Expr: `histogram_quantile(0.95, sum by (le, route) (rate(devops_http_request_duration_seconds_bucket{job="devops-services",service_name=~"$service",deployment_environment=~"$env",route=~"$route"}[5m])))`, LegendFormat: "p95 {{route}}"},
		target{RefID: "C", Expr: `histogram_quantile(0.99, sum by (le, route) (rate(devops_http_request_duration_seconds_bucket{job="devops-services",service_name=~"$service",deployment_environment=~"$env",route=~"$route"}[5m])))`, LegendFormat: "p99 {{route}}"}))
	panels = append(panels, timeseriesPanel(19,
		"HTTP latency quantiles (otelgin semconv)",
		"P50/P95/P99 from otelgin's semconv histogram, kept separate from the in-house histogram. It uses http_route, not route.",
		"s", false, nil,
		target{RefID: "A", Expr: `histogram_quantile(0.50, sum by (le, http_route) (rate(http_server_request_duration_seconds_bucket{job="devops-services",service_name=~"$service",deployment_environment=~"$env",http_route=~"$route"}[5m])))`, LegendFormat: "p50 {{http_route}}"},
		target{RefID: "B", Expr: `histogram_quantile(0.95, sum by (le, http_route) (rate(http_server_request_duration_seconds_bucket{job="devops-services",service_name=~"$service",deployment_environment=~"$env",http_route=~"$route"}[5m])))`, LegendFormat: "p95 {{http_route}}"},
		target{RefID: "C", Expr: `histogram_quantile(0.99, sum by (le, http_route) (rate(http_server_request_duration_seconds_bucket{job="devops-services",service_name=~"$service",deployment_environment=~"$env",http_route=~"$route"}[5m])))`, LegendFormat: "p99 {{http_route}}"}))
	panels = append(panels, timeseriesPanel(20,
		"HTTP in-flight requests",
		"Requests currently being processed.",
		"short", false, nil,
		target{RefID: "A", Expr: `http_server_in_flight{job="devops-services",service_name=~"$service",deployment_environment=~"$env",instance=~"$instance"}`, LegendFormat: "{{service_name}}"}))

	panels = append(panels, timeseriesPanel(21,
		"gRPC client QPS (by method)",
		"Outbound gRPC call rate per method.",
		"reqps", false, nil,
		target{RefID: "A", Expr: `sum by (rpc_method, service_name) (rate(rpc_client_call_duration_seconds_count{job="devops-services",service_name=~"$service",deployment_environment=~"$env"}[5m]))`, LegendFormat: "{{service_name}} {{rpc_method}}"}))
	panels = append(panels, timeseriesPanel(22,
		"gRPC client error rate (%)",
		"Share of outbound gRPC calls that did not return OK.",
		"percent", true, []float64{1, 5},
		target{RefID: "A", Expr: `100 * sum(rate(rpc_client_call_duration_seconds_count{job="devops-services",service_name=~"$service",deployment_environment=~"$env",rpc_response_status_code!="OK"}[5m])) / sum(rate(rpc_client_call_duration_seconds_count{job="devops-services",service_name=~"$service",deployment_environment=~"$env"}[5m]))`, LegendFormat: "gRPC error rate"}))
	panels = append(panels, timeseriesPanel(23,
		"gRPC server QPS (by method)",
		"Inbound gRPC call rate per method.",
		"reqps", false, nil,
		target{RefID: "A", Expr: `sum by (rpc_method, service_name) (rate(rpc_server_call_duration_seconds_count{job="devops-services",service_name=~"$service",deployment_environment=~"$env"}[5m]))`, LegendFormat: "{{service_name}} {{rpc_method}}"}))

	panels = append(panels, timeseriesPanel(24,
		"Gateway requests (by response class)",
		"Higress/Envoy downstream requests by status class.",
		"reqps", false, nil,
		target{RefID: "A", Expr: `sum by (response_code_class) (rate(envoy_http_downstream_rq{job="higress-gateway",http_conn_manager_prefix=~"outbound.*"}[5m]))`, LegendFormat: "{{response_code_class}}"}))
	panels = append(panels, timeseriesPanel(25,
		"Gateway upstream requests (by cluster)",
		"Envoy upstream requests per cluster.",
		"reqps", false, nil,
		target{RefID: "A", Expr: `sum by (cluster_name) (rate(envoy_cluster_upstream_rq{job="higress-gateway"}[5m]))`, LegendFormat: "{{cluster_name}}"}))
	panels = append(panels, timeseriesPanel(26,
		"Gateway circuit breakers (open connections)",
		"Open connections per Envoy cluster. Non-zero means the circuit breaker is shedding traffic.",
		"short", false, nil,
		target{RefID: "A", Expr: `envoy_cluster_circuit_breakers_default_cx_open{job="higress-gateway"}`, LegendFormat: "{{cluster_name}}"}))

	panels = append(panels, statPanel(27,
		"Dependency readiness",
		"devops_readiness per dependency: 1 = reachable, 0 = unreachable. This matches the condition used by /readyz.",
		"short", []float64{0.5},
		target{RefID: "A", Expr: `devops_readiness{job="devops-services",service_name=~"$service",deployment_environment=~"$env"}`, LegendFormat: "{{service_name}}/{{dependency}}"}))
	panels = append(panels, statPanel(28,
		"Scrape target up",
		"1 = Prometheus can scrape the service's /metrics endpoint.",
		"short", []float64{0.5},
		target{RefID: "A", Expr: `up{job="devops-services",instance=~"$instance"}`, LegendFormat: "{{instance}}"}))
	panels = append(panels, statPanel(29,
		"Service uptime",
		"Seconds since the service process started.",
		"s", nil,
		target{RefID: "A", Expr: `time() - process_start_time_seconds{job="devops-services",service_name=~"$service",deployment_environment=~"$env",instance=~"$instance"}`, LegendFormat: "{{service_name}}"}))
	panels = append(panels, timeseriesPanel(30,
		"Trace span calls (from Tempo)",
		"Span call rate by status code, remote-written by Tempo's metrics-generator. The service label is provided by Tempo.",
		"reqps", false, nil,
		target{RefID: "A", Expr: `sum by (service, status_code) (rate(traces_spanmetrics_calls_total[5m]))`, LegendFormat: "{{service}} {{status_code}}"}))

	g := &grid{}
	for i := range panels {
		panels[i].GridPos = g.next(12, 8)
	}
	return panels
}

func systemAppDashboard() dashboard {
	d := newDashboard(
		"devops-system-app",
		"DevOps system resources + application performance",
		"Host resources (WSL2/Docker VM view via node-exporter), per-service runtime, HTTP/gRPC RED, the Higress gateway and dependency readiness.",
		[]string{"devops", "system", "red"},
	)
	d.Links = []map[string]any{
		{
			"asDropdown":  false,
			"icon":        "external link",
			"includeVars": false,
			"keepTime":    true,
			"tags":        []string{},
			"targetBlank": true,
			"title":       "Traces Drilldown (Tempo)",
			"type":        "link",
			"url":         "/a/grafana-exploretraces-app",
		},
	}
	d.Panels = systemAppPanels()
	return d
}
