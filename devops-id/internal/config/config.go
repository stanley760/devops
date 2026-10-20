// Package config defines the devops-id service configuration structures.
//
// The loading mechanism reuses devops-infra/config (base + APP_ENV environment
// overrides merged); this package only defines its own root Config and
// service-specific config sections.
package config

import (
	"github.com/stanley760/devops/devops-infra/config"
	"time"
)

// Config is the devops-id service root config, mapping to the top-level structure of config/config.yaml
type Config struct {
	Grpc          GrpcConfig          `mapstructure:"grpc"`
	Log           config.LogConfig    `mapstructure:"log"`
	IDGen         IDGenConfig         `mapstructure:"idgen"`
	Etcd          config.EtcdConfig   `mapstructure:"etcd"`
	Nacos         config.NacosConfig  `mapstructure:"nacos"`
	Observability ObservabilityConfig `mapstructure:"observability"`
}

// GrpcConfig is the gRPC service config
type GrpcConfig struct {
	Listen     string `mapstructure:"listen"`     // listen address, e.g. ":9990"
	Reflection bool   `mapstructure:"reflection"` // whether to enable reflection (for dev debugging)
}

// IDGenConfig is the ID generator config
type IDGenConfig struct {
	LeafName    string `mapstructure:"leaf_name"`
	WorkerCache string `mapstructure:"worker_cache"`

	// AllowLocalFallback explicitly enables local fallback ID generation when etcd is unavailable.
	// Defaults to false (fail-closed): refuse ID generation when etcd is down; refusing is preferable to duplicates.
	AllowLocalFallback bool `mapstructure:"allow_local_fallback"`

	// MaxBizTagLen is the maximum byte length of biz_tag (default 64).
	MaxBizTagLen int `mapstructure:"max_biz_tag_len"`
	// MaxGenerators is the maximum number of concurrent generators (default 1024).
	MaxGenerators int `mapstructure:"max_generators"`
	// MaxBatchCount is the maximum batch size per request (default 100000).
	MaxBatchCount int `mapstructure:"max_batch_count"`

	// WatermarkSafetyMargin is the watermark safety margin (default 5s); must be greater than WatermarkPersistInterval.
	WatermarkSafetyMargin time.Duration `mapstructure:"watermark_safety_margin"`
	// WatermarkPersistInterval is the watermark persistence interval (default 3s).
	WatermarkPersistInterval time.Duration `mapstructure:"watermark_persist_interval"`
	// WatermarkMaxWait is the maximum time to wait for the watermark window on restart (default 30s).
	WatermarkMaxWait time.Duration `mapstructure:"watermark_max_wait"`
}

// ObservabilityConfig is the service-level observability config; ServiceName is derived from nacos.registry.service_name.
type ObservabilityConfig struct {
	OTLPEndpoint   string  `mapstructure:"otlp_endpoint"`
	SamplerRatio   float64 `mapstructure:"sampler_ratio"`
	MetricsEnabled bool    `mapstructure:"metrics_enabled"`
	MetricsListen  string  `mapstructure:"metrics_listen"`
}
