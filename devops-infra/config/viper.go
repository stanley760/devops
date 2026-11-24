// Package config defines application config structures and wraps viper's initialization and reading.
//
// Responsibilities:
//   - Init(path) first loads .env, then bootstrap.yaml, and enables hot reload
//   - MergeEnvConfig() merges the APP_ENV environment override file <service>-<env>.yaml
//   - LoadInto() deserializes the config into a struct
//
// Configuration layout (.env + bootstrap + environment overlay):
//
//	config/.env                    Local environment variables (APP_ENV, HOST_IP, etc.), not committed
//	config/bootstrap.yaml          Bootstrap config: service identity + common baseline, always loaded
//	config/<service>-<env>.yaml    Environment overlay, merged by the caller via MergeEnvConfig
//	Example: devops-id-dev.yaml, devops-system-prod.yaml
//
// The .env file ranks below OS environment variables: variables already set are not overwritten by .env,
// preventing externally injected values in CI/container environments from being tampered with by local files.
//
// Usage (cmd layer):
//
//	cobra.OnInitialize(func() { config.Init(confFile) })
//	cfg, err := config.LoadInto(&cfg)
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// EnvAppEnv is the environment variable name used to select the environment; values such as dev / prod / test.
const EnvAppEnv = "APP_ENV"

// loadDotEnv loads the .env file next to bootstrap (when present), injecting its variables
// into os.Environ if not already set in the OS environment. A missing file is skipped silently — production environments usually do not rely on .env.
func loadDotEnv(basePath string) {
	configDir := filepath.Dir(basePath)
	dotEnv := filepath.Join(configDir, ".env")
	if _, err := os.Stat(dotEnv); err != nil {
		return // .env missing, skip
	}
	// Overload=false: do not overwrite OS environment variables already set (CI/container injection wins)
	if err := godotenv.Load(dotEnv); err == nil {
		dotEnvFile = ".env"
	}
}

var (
	baseFile   string
	envFile    string
	dotEnvFile string
)

// Init loads .env (if present), then reads the bootstrap config.
// Local environment override files are not merged at this point — the caller
// merges them via MergeEnvConfig after Init.
func Init(path string) error {
	if path == "" {
		path = filepath.Join("config", "bootstrap.yaml")
	}
	// Load .env first: inject local variables such as APP_ENV and HOST_IP into the process environment.
	// OS environment variables already set take precedence; .env never overwrites them (safe for CI/container injection).
	loadDotEnv(path)
	viper.Reset()
	viper.SetConfigFile(path)
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read base config %s: %w", path, err)
	}
	baseFile = path
	envFile = ""
	viper.WatchConfig()
	return nil
}

// MergeEnvConfig merges the local environment override file <service>-<env>.yaml (when present).
// A missing file is skipped silently — the bootstrap config is then the whole config.
func MergeEnvConfig() error {
	return mergeEnvConfig(baseFile)
}

// mergeEnvConfig loads the service environment config per APP_ENV; file name format: <config_name>-<env>.yaml.
func mergeEnvConfig(basePath string) error {
	env := Env()
	serviceName := ConfigServiceName()
	if serviceName == "" {
		return nil
	}
	configDir := filepath.Dir(basePath)
	candidate := filepath.Join(configDir, serviceName+"-"+env+".yaml")
	if _, err := os.Stat(candidate); err != nil {
		return nil
	}
	envViper := viper.New()
	envViper.SetConfigFile(candidate)
	if err := envViper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read module environment config %s: %w", candidate, err)
	}
	if err := viper.MergeConfigMap(envViper.AllSettings()); err != nil {
		return fmt.Errorf("failed to merge module environment config %s: %w", candidate, err)
	}
	envFile = candidate
	return nil
}

// ServiceName returns the service identity from service.name. It feeds the
// observability service.name (cross-service trace and Grafana filtering depend
// on it).
func ServiceName() string {
	return strings.TrimSpace(viper.GetString("service.name"))
}

// ConfigServiceName returns the name used to derive the environment override
// file name (<name>-<env>.yaml). It defaults to service.name and is overridden
// by service.config_name when set. The gray instance runs under the identity
// "devops-system-gray" but shares the main instance's business config
// ("devops-system" / "devops-system-dev.yaml"), so it sets this to
// "devops-system" explicitly.
func ConfigServiceName() string {
	if name := strings.TrimSpace(viper.GetString("service.config_name")); name != "" {
		return name
	}
	return ServiceName()
}

// Env returns the currently effective environment name, defaulting to dev when unset.
func Env() string {
	env := strings.TrimSpace(os.Getenv(EnvAppEnv))
	if env == "" {
		return "dev"
	}
	return env
}

// Files returns the sources of the configuration files actually loaded.
func Files() string {
	sources := make([]string, 0, 4)
	if dotEnvFile != "" {
		sources = append(sources, dotEnvFile)
	}
	if baseFile != "" {
		sources = append(sources, baseFile)
	}
	if envFile != "" {
		sources = append(sources, filepath.Base(envFile))
	}
	return strings.Join(sources, " + ")
}

// LoadInto deserializes the merged config into a struct defined by the caller.
func LoadInto(target any) error {
	if err := viper.Unmarshal(target); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}
	return nil
}

// MergeWithYAML incrementally merges a YAML document into the active local
// configuration. Existing values are preserved unless the document overrides
// them, matching Spring Boot's higher-priority remote configuration behavior.
func MergeWithYAML(content string) error {
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("external config content is empty")
	}
	remote := viper.New()
	remote.SetConfigType("yaml")
	if err := remote.ReadConfig(strings.NewReader(content)); err != nil {
		return fmt.Errorf("failed to parse external YAML config: %w", err)
	}
	if err := viper.MergeConfigMap(remote.AllSettings()); err != nil {
		return fmt.Errorf("failed to merge external YAML config: %w", err)
	}
	return nil
}
