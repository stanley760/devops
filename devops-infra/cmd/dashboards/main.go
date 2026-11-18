package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	out := flag.String("out", "deploy/grafana/provisioning/dashboards", "dashboard output directory")
	flag.Parse()

	boards := buildAll()
	if len(boards) == 0 {
		fail("no dashboards to render")
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fail("create output directory: %v", err)
	}
	for _, d := range boards {
		data, err := json.MarshalIndent(d, "", "  ")
		if err != nil {
			fail("marshal %s: %v", d.UID, err)
		}
		data = append(data, '\n')
		path := filepath.Join(*out, d.UID+".json")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			fail("write %s: %v", path, err)
		}
		fmt.Printf("==> %s (%d panels)\n", path, len(d.Panels))
	}
}

func buildAll() []dashboard {
	return []dashboard{systemAppDashboard(), dataMiddlewareDashboard(), logsSecurityDashboard()}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
