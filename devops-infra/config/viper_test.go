package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestInitSkipsEnvFileUntilMergeEnvConfig(t *testing.T) {
	t.Setenv(EnvAppEnv, "test")

	dir := t.TempDir()
	bootstrap := filepath.Join(dir, "bootstrap.yaml")
	writeTestFile(t, bootstrap, "service:\n  name: devops-id\nlog:\n  level: info\n")
	writeTestFile(t, filepath.Join(dir, "devops-id-test.yaml"), "log:\n  level: debug\n")

	if err := Init(bootstrap); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(viper.Reset)

	// Init loads bootstrap only; the environment overlay is merged by the caller
	if got := viper.GetString("log.level"); got != "info" {
		t.Fatalf("log.level = %q, want %q", got, "info")
	}

	// MergeEnvConfig merges the APP_ENV overlay
	if err := MergeEnvConfig(); err != nil {
		t.Fatalf("MergeEnvConfig() error = %v", err)
	}
	if got := viper.GetString("log.level"); got != "debug" {
		t.Fatalf("log.level = %q, want %q", got, "debug")
	}
}

func TestMergeWithYAMLOverridesWithoutReset(t *testing.T) {
	dir := t.TempDir()
	bootstrap := filepath.Join(dir, "bootstrap.yaml")
	writeTestFile(t, bootstrap, "server:\n  port: 9000\nlog:\n  level: info\n")

	if err := Init(bootstrap); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(viper.Reset)

	if err := MergeWithYAML("log:\n  level: warn\n"); err != nil {
		t.Fatalf("MergeWithYAML() error = %v", err)
	}
	if got := viper.GetString("log.level"); got != "warn" {
		t.Fatalf("log.level = %q, want %q", got, "warn")
	}
	if got := viper.GetInt("server.port"); got != 9000 {
		t.Fatalf("server.port = %d, want 9000", got)
	}
}

func TestConfigServiceNameFallsBackToServiceName(t *testing.T) {
	t.Setenv(EnvAppEnv, "prod")

	dir := t.TempDir()
	bootstrap := filepath.Join(dir, "bootstrap.yaml")
	writeTestFile(t, bootstrap, "service:\n  name: devops-system\n")

	if err := Init(bootstrap); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(viper.Reset)

	if got := ConfigServiceName(); got != "devops-system" {
		t.Fatalf("ConfigServiceName() = %q, want %q", got, "devops-system")
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// The gray instance runs under the identity "devops-system-gray" but shares the main
// instance's business config, so service.config_name overrides the config-side name;
// without it the config file name is derived from the gray name and the instance
// cannot find any configuration.
func TestConfigServiceNameOverridesServiceName(t *testing.T) {
	t.Setenv(EnvAppEnv, "dev")

	dir := t.TempDir()
	bootstrap := filepath.Join(dir, "bootstrap.yaml")
	writeTestFile(t, bootstrap, "service:\n  name: devops-system-gray\n  config_name: devops-system\n")

	if err := Init(bootstrap); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(viper.Reset)

	if got := ServiceName(); got != "devops-system-gray" {
		t.Fatalf("ServiceName() = %q, want %q", got, "devops-system-gray")
	}
	if got := ConfigServiceName(); got != "devops-system" {
		t.Fatalf("ConfigServiceName() = %q, want %q", got, "devops-system")
	}
}
