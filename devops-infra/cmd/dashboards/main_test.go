package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSystemAppDashboardIsWellFormed(t *testing.T) {
	for _, d := range buildAll() {
		t.Run(d.UID, func(t *testing.T) {
			if d.UID == "" || d.Title == "" {
				t.Fatalf("dashboard missing uid/title: %+v", d)
			}
			if len(d.Panels) == 0 {
				t.Fatal("dashboard has no panels")
			}
			seen := map[int]bool{}
			for _, p := range d.Panels {
				if p.ID == 0 || seen[p.ID] {
					t.Fatalf("panel %q has missing or duplicate id %d", p.Title, p.ID)
				}
				seen[p.ID] = true
				if p.Title == "" {
					t.Fatalf("panel %d has no title", p.ID)
				}
				if p.Datasource == nil || p.Datasource.UID == "" {
					t.Fatalf("panel %q has no datasource", p.Title)
				}
				if p.GridPos.W == 0 || p.GridPos.H == 0 {
					t.Fatalf("panel %q has no grid position", p.Title)
				}
				for _, target := range p.Targets {
					if strings.TrimSpace(target.Expr) == "" {
						t.Fatalf("panel %q has a target with an empty expression", p.Title)
					}
				}
			}
		})
	}
}

func TestSystemAppDashboardLinksToTraceDrilldown(t *testing.T) {
	for _, link := range systemAppDashboard().Links {
		if link["title"] == "Traces Drilldown (Tempo)" {
			if link["url"] != "/a/grafana-exploretraces-app" {
				t.Fatalf("trace link URL = %v, want /a/grafana-exploretraces-app", link["url"])
			}
			return
		}
	}
	t.Fatal("system/app dashboard has no Traces Drilldown link")
}

func TestDashboardsRenderValidJSON(t *testing.T) {
	for _, d := range buildAll() {
		data, err := json.MarshalIndent(d, "", "  ")
		if err != nil {
			t.Fatalf("marshal %s: %v", d.UID, err)
		}
		if !json.Valid(data) {
			t.Fatalf("%s produced invalid JSON", d.UID)
		}
		var probe map[string]any
		if err := json.Unmarshal(data, &probe); err != nil {
			t.Fatalf("unmarshal %s: %v", d.UID, err)
		}
		for _, key := range []string{"uid", "title", "panels", "templating", "schemaVersion", "time"} {
			if _, ok := probe[key]; !ok {
				t.Fatalf("%s is missing the %q key", d.UID, key)
			}
		}
		if probe["schemaVersion"].(float64) < 39 {
			t.Fatalf("%s uses schemaVersion %v, expected >= 39", d.UID, probe["schemaVersion"])
		}
	}
}

func TestTemplateVariablesResolve(t *testing.T) {
	for _, d := range buildAll() {
		names := map[string]bool{}
		for _, variable := range d.Templating.List {
			names[variable.Name] = true
		}
		for _, want := range []string{"service", "env", "route", "instance"} {
			if !names[want] {
				t.Fatalf("%s is missing the %q template variable", d.UID, want)
			}
		}
	}
}

func TestLogsDashboardAllFiltersMatchNonEmptyLabels(t *testing.T) {
	for _, variable := range logsSecurityDashboard().Templating.List {
		if variable.Name == "service" || variable.Name == "env" {
			if variable.AllValue != ".+" {
				t.Errorf("%s all-value regex must exclude empty labels for Loki selectors, got %q", variable.Name, variable.AllValue)
			}
		}
	}
}

func TestDataMiddlewareDashboardCoversBothDomains(t *testing.T) {
	d := dataMiddlewareDashboard()

	titles := make([]string, 0, len(d.Panels))
	exprs := make([]string, 0)
	for _, p := range d.Panels {
		titles = append(titles, p.Title)
		for _, target := range p.Targets {
			exprs = append(exprs, target.Expr)
		}
	}
	joined := strings.Join(titles, " | ")
	joinedExprs := strings.Join(exprs, " | ")

	for _, want := range []string{
		"MySQL connections",
		"MySQL query rate",
		"MySQL slow queries",
		"SQL pool",
		"Cache hit rate",
		"Redis keyspace hit rate",
		"Redis memory",
		"Redis evictions",
		"Redis connection pool",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("dashboard is missing a panel family %q; titles: %s", want, joined)
		}
	}
	if !strings.Contains(joinedExprs, "devops_cache_operations_total") {
		t.Error("no panel queries devops_cache_operations_total")
	}
	if !strings.Contains(joinedExprs, "db_client_connections_hits") {
		t.Error("no panel shows the Redis connection pool reuse rate")
	}
}

func TestCacheHitRatePreservesCacheLabel(t *testing.T) {
	for _, panel := range dataMiddlewareDashboard().Panels {
		if panel.Title != "Cache hit rate (%)" {
			continue
		}
		if len(panel.Targets) != 1 || strings.Count(panel.Targets[0].Expr, "sum by (cache)") != 3 {
			t.Fatalf("cache hit-rate query must group hit and miss rates by cache: %+v", panel.Targets)
		}
		return
	}
	t.Fatal("cache hit-rate panel not found")
}

func TestLogsSecurityDashboardUsesLokiForLogPanels(t *testing.T) {
	d := logsSecurityDashboard()

	lokiPanels := 0
	for _, p := range d.Panels {
		for _, target := range p.Targets {
			expr := target.Expr
			if p.Datasource != nil && p.Datasource.UID == "loki" {
				lokiPanels++
				for _, forbidden := range []string{"| json", "$__auto", "$__interval", "$__range"} {
					if strings.Contains(expr, forbidden) {
						t.Errorf("panel %q uses %q, which this Loki version rejects: %s", p.Title, forbidden, expr)
					}
				}
				if strings.Contains(expr, "count_over_time") && strings.Contains(expr, "unwrap") {
					t.Errorf("panel %q combines count_over_time with unwrap: %s", p.Title, expr)
				}
			}
		}
	}
	if lokiPanels == 0 {
		t.Fatal("no panel queries the Loki datasource")
	}
}

func TestLogsSecurityDashboardCoversBothDomains(t *testing.T) {
	d := logsSecurityDashboard()

	titles := make([]string, 0, len(d.Panels))
	for _, p := range d.Panels {
		titles = append(titles, p.Title)
	}
	joined := strings.Join(titles, " | ")
	for _, want := range []string{
		"Log volume",
		"Log level",
		"Log status codes",
		"Top request paths",
		"Login failures",
		"Registration failures",
		"4xx",
		"429",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("dashboard is missing a panel family %q; titles: %s", want, joined)
		}
	}
}
