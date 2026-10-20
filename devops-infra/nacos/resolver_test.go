package nacos

import (
	"net/url"
	"testing"

	"google.golang.org/grpc/resolver"
)

// TestServiceNameFromTarget verifies both target forms extract the service name.
// Historical defect: nacos://devops-id (two slashes) parsed the service name as authority,
// so grpc resolver.Target.Endpoint() returned empty → Build errored → the connection never had addresses.
func TestServiceNameFromTarget(t *testing.T) {
	cases := []struct {
		target string
		want   string
	}{
		{"nacos:///devops-id", "devops-id"},            // standard three slashes: service name in the path position
		{"nacos://devops-id", "devops-id"},             // mistyped two slashes: service name in the authority position, Host as fallback
		{"nacos:///devops-id?timeout=5s", "devops-id"}, // with query parameters
		{"nacos:///", ""},                              // no service name → Build should error
	}

	for _, c := range cases {
		u, err := url.Parse(c.target)
		if err != nil {
			t.Fatalf("parse %q: %v", c.target, err)
		}
		got := serviceNameFromTarget(resolver.Target{URL: *u})
		if got != c.want {
			t.Errorf("serviceNameFromTarget(%q) = %q, want %q", c.target, got, c.want)
		}
	}
}
