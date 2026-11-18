// Package main renders Grafana dashboards for the DevOps observability stack.
package main

type datasource struct {
	Type string `json:"type"`
	UID  string `json:"uid"`
}

var (
	promDatasource = datasource{Type: "prometheus", UID: "prometheus"}
	lokiDatasource = datasource{Type: "loki", UID: "loki"}
)

type target struct {
	RefID        string `json:"refId"`
	Expr         string `json:"expr"`
	LegendFormat string `json:"legendFormat,omitempty"`
}

type gridPos struct {
	H int `json:"h"`
	W int `json:"w"`
	X int `json:"x"`
	Y int `json:"y"`
}

type thresholdStep struct {
	Color string   `json:"color"`
	Value *float64 `json:"value"`
}

type override struct {
	Matcher    map[string]string `json:"matcher"`
	Properties map[string]any    `json:"properties"`
}

type fieldConfig struct {
	Defaults  map[string]any `json:"defaults"`
	Overrides []override     `json:"overrides"`
}

type panel struct {
	ID          int            `json:"id"`
	Type        string         `json:"type"`
	Title       string         `json:"title"`
	Description string         `json:"description,omitempty"`
	Datasource  *datasource    `json:"datasource,omitempty"`
	GridPos     gridPos        `json:"gridPos"`
	FieldConfig fieldConfig    `json:"fieldConfig"`
	Targets     []target       `json:"targets,omitempty"`
	Options     map[string]any `json:"options"`
}

type templateVariable struct {
	Name       string            `json:"name"`
	Label      string            `json:"label"`
	Type       string            `json:"type"`
	Datasource *datasource       `json:"datasource,omitempty"`
	Definition string            `json:"definition"`
	Query      map[string]string `json:"query"`
	Regex      string            `json:"regex"`
	IncludeAll bool              `json:"includeAll"`
	AllValue   string            `json:"allValue"`
	Multi      bool              `json:"multi"`
	Sort       int               `json:"sort"`
	Refresh    int               `json:"refresh"`
	Current    map[string]any    `json:"current"`
}

type templating struct {
	List []templateVariable `json:"list"`
}

type dashboard struct {
	Annotations   map[string]any    `json:"annotations"`
	Editable      bool              `json:"editable"`
	GraphTooltip  int               `json:"graphTooltip"`
	Links         []map[string]any  `json:"links"`
	Panels        []panel           `json:"panels"`
	Refresh       string            `json:"refresh"`
	SchemaVersion int               `json:"schemaVersion"`
	Tags          []string          `json:"tags"`
	Templating    templating        `json:"templating"`
	Time          map[string]string `json:"time"`
	Timepicker    map[string]any    `json:"timepicker"`
	Timezone      string            `json:"timezone"`
	Title         string            `json:"title"`
	UID           string            `json:"uid"`
	Version       int               `json:"version"`
	WeekStart     string            `json:"weekStart"`
}
