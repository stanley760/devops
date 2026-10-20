// Package config defines application config structures and wraps viper's initialization and reading.
//
// Responsibilities:
//   - Init(path) first loads .env, then bootstrap.yaml, merges the service environment override file, and enables hot reload
//   - LoadInto() deserializes the config into a struct
//
// Configuration layout (.env + bootstrap + remote first + local fallback):
//
//	config/.env                    Local environment variables (APP_ENV, NACOS_REGISTER_IP, etc.), not committed
//	config/bootstrap.yaml          Bootstrap config: nacos connection info + common baseline, always loaded
//	remote dataID                  Loaded first; once hit, the remote value wins
//	config/<service>-<env>.yaml    Local fallback: merged by the caller via MergeEnvConfig only when
//	                              the remote side does not hit the same-named dataID
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
	baseFile       string
	envFile        string
	dotEnvFile     string
	remoteFiles    []string
	bootstrapNacos map[string]any
)

// Init loads .env (if present), then reads the bootstrap config.
// Local environment override files are not merged at this point — remote Nacos config wins; only when the remote misses
// does the caller load the fallback via MergeEnvConfig.
func Init(path string) error {
	if path == "" {
		path = filepath.Join("config", "bootstrap.yaml")
	}
	// Load .env first: inject local variables such as APP_ENV and NACOS_REGISTER_IP into the process environment.
	// OS environment variables already set take precedence; .env never overwrites them (safe for CI/container injection).
	loadDotEnv(path)
	viper.Reset()
	viper.SetConfigFile(path)
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read base config %s: %w", path, err)
	}
	baseFile = path
	envFile = ""
	remoteFiles = nil
	bootstrapNacos = viper.GetStringMap("nacos")
	viper.WatchConfig()
	return nil
}

// MergeEnvConfig merges the local environment override file <service>-<env>.yaml (when present).
// It serves as the local fallback when the remote config center misses; a missing file is skipped silently.
func MergeEnvConfig() error {
	return mergeEnvConfig(baseFile)
}

// mergeEnvConfig loads the service environment config per APP_ENV; file name format: <service>-<env>.yaml.
func mergeEnvConfig(basePath string) error {
	env := Env()
	serviceName := ServiceName()
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

// ServiceName returns the service name used to derive Spring Boot style
// configuration names from nacos.registry.service_name.
func ServiceName() string {
	return strings.TrimSpace(viper.GetString("nacos.registry.service_name"))
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
	for _, dataID := range remoteFiles {
		sources = append(sources, "nacos:"+dataID)
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

// MergeRemoteYAML merges one remote configuration document and records its
// data ID as a configuration source.
func MergeRemoteYAML(dataID, content string) error {
	localNacos := bootstrapNacos
	if localNacos == nil {
		localNacos = map[string]any{}
	}
	if err := MergeWithYAML(content); err != nil {
		return err
	}
	viper.Set("nacos", localNacos)
	remoteFiles = append(remoteFiles, dataID)
	return nil
}

// RemoteDataIDs returns configured shared data IDs followed by the Spring
// Boot style service and profile data IDs.
func RemoteDataIDs(configured []string) []string {
	dataIDs := make([]string, 0, len(configured)+2)
	for _, dataID := range configured {
		if trimmed := strings.TrimSpace(dataID); trimmed != "" && !containsString(dataIDs, trimmed) {
			dataIDs = append(dataIDs, trimmed)
		}
	}
	serviceName := ServiceName()
	if serviceName == "" {
		return dataIDs
	}
	mainDataID := serviceName + ".yaml"
	if !containsString(dataIDs, mainDataID) {
		dataIDs = append(dataIDs, mainDataID)
	}
	profileDataID := ProfileDataID()
	if profileDataID != "" && !containsString(dataIDs, profileDataID) {
		dataIDs = append(dataIDs, profileDataID)
	}
	return dataIDs
}

// ProfileDataID returns the profile dataID (<service>-<env>.yaml), sharing its name with the local fallback
// file. Callers use it to decide whether the remote side hit: merge the local environment override file only on a miss.
// Returns an empty string when service_name is not configured.
func ProfileDataID() string {
	serviceName := ServiceName()
	if serviceName == "" {
		return ""
	}
	return serviceName + "-" + Env() + ".yaml"
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
