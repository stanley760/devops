package main

func logsSecurityPanels() []panel {
	var panels []panel
	logSelector := `{service_name=~"$service",deployment_environment=~"$env"}`

	panels = append(panels, lokiTimeseriesPanel(1,
		"Log volume (by service)",
		"Log lines per minute, grouped by service.",
		target{RefID: "A", Expr: `sum by (service_name) (count_over_time(` + logSelector + ` [1m]))`, LegendFormat: "{{service_name}}"}))
	panels = append(panels, lokiTimeseriesPanel(2,
		"Log level distribution",
		"Log lines per minute by detected level. Error series are empty while no error-level logs are emitted.",
		target{RefID: "A", Expr: `sum by (detected_level) (count_over_time(` + logSelector + ` | detected_level!="" [1m]))`, LegendFormat: "{{detected_level}}"}))
	panels = append(panels, lokiTimeseriesPanel(3,
		"ERROR log rate",
		"Error-level log lines per minute. An empty graph means no error-level logs were emitted.",
		target{RefID: "A", Expr: `sum(count_over_time(` + logSelector + ` | detected_level="error" [1m]))`, LegendFormat: "errors/min"}))
	panels = append(panels, lokiTimeseriesPanel(4,
		"Log status codes",
		"Log lines per minute grouped by the HTTP status code carried as structured metadata.",
		target{RefID: "A", Expr: `sum by (status) (count_over_time(` + logSelector + ` | status!="" [1m]))`, LegendFormat: "{{status}}"}))
	panels = append(panels, lokiTablePanel(5,
		"Top request paths",
		"The ten most frequently logged request paths.",
		target{RefID: "A", Expr: `topk(10, sum by (path) (count_over_time(` + logSelector + ` | path!="" [1m])))`, LegendFormat: "{{path}}"}))
	panels = append(panels, lokiTablePanel(6,
		"Slowest requests (by path)",
		"The ten paths with the highest observed request latency. Latency is aggregated, never used as a grouping label.",
		target{RefID: "A", Expr: `topk(10, max by (path) (max_over_time(` + logSelector + ` | unwrap duration(latency) | __error__="" [1m])))`, LegendFormat: "{{path}}"}))
	panels = append(panels, lokiTablePanel(7,
		"Top client IPs",
		"The ten client addresses producing the most log lines.",
		target{RefID: "A", Expr: `topk(10, sum by (client_ip) (count_over_time(` + logSelector + ` | client_ip!="" [1m])))`, LegendFormat: "{{client_ip}}"}))
	panels = append(panels, lokiLogsPanel(8,
		"Error log stream",
		"Live stream of error-level log lines; empty while the system is healthy.",
		target{RefID: "A", Expr: logSelector + ` | detected_level="error"`}))
	panels = append(panels, lokiLogsPanel(9,
		"Authentication failure stream",
		"Live stream of log lines carrying a 401 or 403 status.",
		target{RefID: "A", Expr: logSelector + ` | status=~"401|403"`}))

	panels = append(panels, timeseriesPanel(10,
		"Login failures",
		"Failed login attempts per second, per service. The counter is lazy, so an empty series means no login has been attempted.",
		"short", false, nil,
		target{RefID: "A", Expr: `sum by (service_name) (rate(auth_login_total{service_name=~"$service",deployment_environment=~"$env",result="failure"}[5m])) or vector(0)`, LegendFormat: "{{service_name}}"}))
	panels = append(panels, timeseriesPanel(11,
		"Login failure rate (%)",
		"Share of login attempts that failed; the business code distinguishes credential failures from server errors.",
		"percent", true, []float64{30, 50},
		target{RefID: "A", Expr: `100 * sum by (service_name) (rate(auth_login_total{service_name=~"$service",deployment_environment=~"$env",result="failure"}[5m])) / sum by (service_name) (rate(auth_login_total{service_name=~"$service",deployment_environment=~"$env"}[5m])) or vector(0)`, LegendFormat: "{{service_name}}"}))
	panels = append(panels, timeseriesPanel(12,
		"Login failures by business code",
		"Failed logins grouped by business error code.",
		"short", false, nil,
		target{RefID: "A", Expr: `sum by (code) (rate(auth_login_total{service_name=~"$service",deployment_environment=~"$env",result="failure"}[5m])) or vector(0)`, LegendFormat: "code {{code}}"}))
	panels = append(panels, timeseriesPanel(13,
		"Registration failures",
		"Failed registrations per second, per service.",
		"short", false, nil,
		target{RefID: "A", Expr: `sum by (service_name) (rate(auth_register_total{service_name=~"$service",deployment_environment=~"$env",result="failure"}[5m])) or vector(0)`, LegendFormat: "{{service_name}}"}))
	panels = append(panels, timeseriesPanel(14,
		"4xx responses (scanning / credential stuffing)",
		"Client-error responses per second, grouped by status code.",
		"reqps", false, nil,
		target{RefID: "A", Expr: `sum by (status) (rate(http_server_requests_total{job="devops-services",service_name=~"$service",deployment_environment=~"$env",status=~"4.."}[5m]))`, LegendFormat: "{{status}}"}))
	panels = append(panels, timeseriesPanel(15,
		"5xx responses (server faults)",
		"Server-error responses per second, grouped by status code.",
		"reqps", false, nil,
		target{RefID: "A", Expr: `sum by (status) (rate(http_server_requests_total{job="devops-services",service_name=~"$service",deployment_environment=~"$env",status=~"5.."}[5m]))`, LegendFormat: "{{status}}"}))
	panels = append(panels, timeseriesPanel(16,
		"Rate limiting (429)",
		"Requests rejected by the rate limiter.",
		"reqps", false, nil,
		target{RefID: "A", Expr: `sum(rate(http_server_requests_total{job="devops-services",service_name=~"$service",deployment_environment=~"$env",status="429"}[5m])) or vector(0)`, LegendFormat: "429/s"}))
	panels = append(panels, timeseriesPanel(17,
		"Gateway responses by class",
		"Higress/Envoy downstream responses by status class.",
		"reqps", false, nil,
		target{RefID: "A", Expr: `sum by (response_code_class) (rate(envoy_http_downstream_rq{job="higress-gateway",http_conn_manager_prefix=~"outbound.*"}[5m]))`, LegendFormat: "{{response_code_class}}"}))
	panels = append(panels, lokiTablePanel(18,
		"Top client IPs by error logs",
		"The ten client addresses producing the most error-level log lines.",
		target{RefID: "A", Expr: `topk(10, sum by (client_ip) (count_over_time(` + logSelector + ` | client_ip!="" | detected_level="error" [1m])))`, LegendFormat: "{{client_ip}}"}))

	g := &grid{}
	for i := range panels {
		panels[i].GridPos = g.next(12, 8)
	}
	return panels
}

func logsSecurityDashboard() dashboard {
	d := newDashboard(
		"devops-logs-security",
		"DevOps logs + security",
		"Log volume, level and status analysis from Loki, plus login/registration failures, client/server errors, rate limiting and gateway security signals.",
		[]string{"devops", "logs", "security"},
	)
	d.Panels = logsSecurityPanels()
	return d
}
