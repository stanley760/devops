package main

type grid struct {
	x    int
	y    int
	rowH int
}

func (g *grid) next(w, h int) gridPos {
	if g.x+w > 24 {
		g.x = 0
		g.y += g.rowH
		g.rowH = 0
	}
	pos := gridPos{H: h, W: w, X: g.x, Y: g.y}
	g.x += w
	if h > g.rowH {
		g.rowH = h
	}
	return pos
}

func percentField(minValue, maxValue float64) map[string]any {
	return map[string]any{
		"unit": "percent",
		"min":  minValue,
		"max":  maxValue,
		"custom": map[string]any{
			"fillOpacity": 10,
			"lineWidth":   2,
		},
	}
}

func plainField(unit string) map[string]any {
	return map[string]any{
		"unit": unit,
		"custom": map[string]any{
			"fillOpacity": 10,
			"lineWidth":   2,
		},
	}
}

func timeseriesPanel(id int, title, description, unit string, isPercent bool, thresholds []float64, targets ...target) panel {
	defaults := plainField(unit)
	if isPercent {
		defaults = percentField(0, 100)
	}
	var overrides []override
	if len(thresholds) > 0 {
		steps := []thresholdStep{{Color: "green", Value: nil}}
		colors := []string{"yellow", "red", "dark-red"}
		for i, value := range thresholds {
			color := "red"
			if i < len(colors) {
				color = colors[i]
			}
			v := value
			steps = append(steps, thresholdStep{Color: color, Value: &v})
		}
		overrides = append(overrides, override{
			Matcher:    map[string]string{"id": "byName", "options": "Value"},
			Properties: map[string]any{"thresholds": map[string]any{"mode": "absolute", "steps": steps}},
		})
	}
	return panel{
		ID:          id,
		Type:        "timeseries",
		Title:       title,
		Description: description,
		Datasource:  &promDatasource,
		FieldConfig: fieldConfig{Defaults: defaults, Overrides: overrides},
		Targets:     targets,
		Options: map[string]any{
			"legend":  map[string]any{"displayMode": "table", "placement": "bottom", "calcs": []string{"lastNotNull", "max"}},
			"tooltip": map[string]any{"mode": "multi"},
		},
	}
}

func statPanel(id int, title, description, unit string, thresholds []float64, targets ...target) panel {
	defaults := plainField(unit)
	var overrides []override
	if len(thresholds) > 0 {
		steps := []thresholdStep{{Color: "green", Value: nil}}
		colors := []string{"yellow", "red", "dark-red"}
		for i, value := range thresholds {
			color := "red"
			if i < len(colors) {
				color = colors[i]
			}
			v := value
			steps = append(steps, thresholdStep{Color: color, Value: &v})
		}
		overrides = append(overrides, override{
			Matcher:    map[string]string{"id": "byName", "options": "Value"},
			Properties: map[string]any{"thresholds": map[string]any{"mode": "absolute", "steps": steps}},
		})
	}
	return panel{
		ID:          id,
		Type:        "stat",
		Title:       title,
		Description: description,
		Datasource:  &promDatasource,
		FieldConfig: fieldConfig{Defaults: defaults, Overrides: overrides},
		Targets:     targets,
		Options: map[string]any{
			"colorMode":     "value",
			"graphMode":     "area",
			"justifyMode":   "auto",
			"orientation":   "auto",
			"reduceOptions": map[string]any{"calcs": []string{"lastNotNull"}, "fields": "", "values": false},
			"textMode":      "auto",
			"wideLayout":    true,
		},
	}
}

func tablePanel(id int, title, description string, targets ...target) panel {
	return panel{
		ID:          id,
		Type:        "table",
		Title:       title,
		Description: description,
		Datasource:  &promDatasource,
		FieldConfig: fieldConfig{Defaults: map[string]any{}},
		Targets:     targets,
		Options:     map[string]any{"showHeader": true, "sortBy": []map[string]any{{"desc": true, "displayName": "Value"}}},
	}
}

func lokiTimeseriesPanel(id int, title, description string, targets ...target) panel {
	return panel{
		ID:          id,
		Type:        "timeseries",
		Title:       title,
		Description: description,
		Datasource:  &lokiDatasource,
		FieldConfig: fieldConfig{Defaults: map[string]any{"custom": map[string]any{"fillOpacity": 10, "lineWidth": 2}}},
		Targets:     targets,
		Options: map[string]any{
			"legend":  map[string]any{"displayMode": "table", "placement": "bottom", "calcs": []string{"lastNotNull"}},
			"tooltip": map[string]any{"mode": "multi"},
		},
	}
}

func lokiTablePanel(id int, title, description string, targets ...target) panel {
	return panel{
		ID:          id,
		Type:        "table",
		Title:       title,
		Description: description,
		Datasource:  &lokiDatasource,
		FieldConfig: fieldConfig{Defaults: map[string]any{}},
		Targets:     targets,
		Options:     map[string]any{"showHeader": true, "sortBy": []map[string]any{{"desc": true, "displayName": "Value"}}},
	}
}

func lokiLogsPanel(id int, title, description string, targets ...target) panel {
	return panel{
		ID:          id,
		Type:        "logs",
		Title:       title,
		Description: description,
		Datasource:  &lokiDatasource,
		FieldConfig: fieldConfig{Defaults: map[string]any{}},
		Targets:     targets,
		Options: map[string]any{
			"dedupStrategy":      "none",
			"enableLogDetails":   true,
			"prettifyLogMessage": false,
			"showCommonLabels":   false,
			"showLabels":         false,
			"showTime":           true,
			"sortOrder":          "Descending",
			"wrapLogMessage":     true,
		},
	}
}

func serviceVariables() templating {
	// Loki rejects stream selectors whose regex matchers can match empty labels.
	return templating{List: []templateVariable{
		{
			Name:       "service",
			Label:      "Service",
			Type:       "query",
			Datasource: &promDatasource,
			Definition: `label_values({service_name=~".+"}, service_name)`,
			Query:      map[string]string{"query": `label_values({service_name=~".+"}, service_name)`, "refId": "PrometheusVariableQueryEditor-VariableQuery"},
			IncludeAll: true,
			AllValue:   ".+",
			Multi:      true,
			Sort:       1,
			Refresh:    2,
			Current:    map[string]any{"selected": true, "text": []string{"All"}, "value": []string{"$__all"}},
		},
		{
			Name:       "env",
			Label:      "Environment",
			Type:       "query",
			Datasource: &promDatasource,
			Definition: `label_values({deployment_environment=~".+"}, deployment_environment)`,
			Query:      map[string]string{"query": `label_values({deployment_environment=~".+"}, deployment_environment)`, "refId": "PrometheusVariableQueryEditor-VariableQuery"},
			IncludeAll: true,
			AllValue:   ".+",
			Multi:      true,
			Sort:       1,
			Refresh:    2,
			Current:    map[string]any{"selected": true, "text": []string{"All"}, "value": []string{"$__all"}},
		},
		{
			Name:       "route",
			Label:      "Route",
			Type:       "query",
			Datasource: &promDatasource,
			Definition: `label_values(http_server_requests_total, route)`,
			Query:      map[string]string{"query": `label_values(http_server_requests_total, route)`, "refId": "PrometheusVariableQueryEditor-VariableQuery"},
			IncludeAll: true,
			AllValue:   ".*",
			Multi:      true,
			Sort:       1,
			Refresh:    2,
			Current:    map[string]any{"selected": true, "text": []string{"All"}, "value": []string{"$__all"}},
		},
		{
			Name:       "instance",
			Label:      "Instance",
			Type:       "query",
			Datasource: &promDatasource,
			Definition: `label_values(up, instance)`,
			Query:      map[string]string{"query": `label_values(up, instance)`, "refId": "PrometheusVariableQueryEditor-VariableQuery"},
			IncludeAll: true,
			AllValue:   ".*",
			Multi:      true,
			Sort:       1,
			Refresh:    2,
			Current:    map[string]any{"selected": true, "text": []string{"All"}, "value": []string{"$__all"}},
		},
	}}
}

func annotations() map[string]any {
	return map[string]any{
		"list": []map[string]any{
			{
				"builtIn":    1,
				"datasource": map[string]any{"type": "grafana", "uid": "-- Grafana --"},
				"enable":     true,
				"hide":       true,
				"iconColor":  "rgba(0, 211, 255, 1)",
				"name":       "Annotations & Alerts",
				"type":       "dashboard",
			},
		},
	}
}

func newDashboard(uid, title, description string, tags []string) dashboard {
	return dashboard{
		Annotations:   annotations(),
		Editable:      true,
		GraphTooltip:  1,
		Links:         []map[string]any{},
		Panels:        []panel{},
		Refresh:       "30s",
		SchemaVersion: 39,
		Tags:          tags,
		Templating:    serviceVariables(),
		Time:          map[string]string{"from": "now-1h", "to": "now"},
		Timepicker:    map[string]any{},
		Title:         title,
		UID:           uid,
		Version:       1,
	}
}
