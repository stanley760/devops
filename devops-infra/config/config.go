// Package config provides shared configuration types and a common loader.
//
// Responsibilities:
//   - This file only holds config structures shared by all services (logger, HTTP server)
//   - The loading mechanism (base + APP_ENV environment override merge) lives in viper.go of the same package
//   - Each service defines its own root Config and composes these types, loaded via LoadInto
package config

import "time"

// ServerConfig is the HTTP server config.
type ServerConfig struct {
	Port           int           `mapstructure:"port"`            // listen port
	ReadTimeout    time.Duration `mapstructure:"read_timeout"`    // read timeout; 0 means no limit
	WriteTimeout   time.Duration `mapstructure:"write_timeout"`   // write timeout; 0 means no limit
	TrustedProxies []string      `mapstructure:"trusted_proxies"` // trusted reverse proxy IPs or CIDRs
}
