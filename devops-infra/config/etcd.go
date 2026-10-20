package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
	"time"
)

// EtcdTLSConfig defines optional mutual TLS settings for an etcd client.
type EtcdTLSConfig struct {
	CAFile             string `mapstructure:"ca_file"`
	CertFile           string `mapstructure:"cert_file"`
	KeyFile            string `mapstructure:"key_file"`
	InsecureSkipVerify bool   `mapstructure:"insecure_skip_verify"`
}

type EtcdConfig struct {
	Endpoints   []string       `mapstructure:"endpoints"`
	Username    string         `mapstructure:"username"`
	Password    string         `mapstructure:"password"`
	DialTimeout time.Duration  `mapstructure:"dial_timeout"`
	TLS         *EtcdTLSConfig `mapstructure:"tls"`
	LeaseTTL    time.Duration  `mapstructure:"lease_ttl"`
}

func NormalizeEtcdConfig(cfg EtcdConfig) (EtcdConfig, error) {
	endpoints := make([]string, 0, len(cfg.Endpoints))
	for _, endpoint := range cfg.Endpoints {
		endpoint = strings.TrimSpace(endpoint)
		if endpoint != "" {
			endpoints = append(endpoints, endpoint)
		}
	}
	if len(endpoints) == 0 {
		return cfg, fmt.Errorf("etcd.endpoints must not be empty")
	}
	if cfg.DialTimeout < 0 {
		return cfg, fmt.Errorf("etcd.dial_timeout cannot be negative")
	}
	if cfg.LeaseTTL < 0 {
		return cfg, fmt.Errorf("etcd.lease_ttl cannot be negative")
	}
	if (cfg.Username == "") != (cfg.Password == "") {
		return cfg, fmt.Errorf("etcd.username and etcd.password must be configured together")
	}
	if err := cfg.ValidateTLS(); err != nil {
		return cfg, err
	}

	cfg.Endpoints = endpoints
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 5 * time.Second
	}
	if cfg.LeaseTTL == 0 {
		cfg.LeaseTTL = 10 * time.Second
	}
	return cfg, nil
}

func (cfg EtcdConfig) ValidateTLS() error {
	if cfg.TLS == nil {
		return nil
	}
	if (cfg.TLS.CertFile == "") != (cfg.TLS.KeyFile == "") {
		return fmt.Errorf("etcd.tls.cert_file and etcd.tls.key_file must be configured together")
	}
	return nil
}

func (cfg EtcdConfig) TLSConfig() (*tls.Config, error) {
	if err := cfg.ValidateTLS(); err != nil {
		return nil, err
	}
	if cfg.TLS == nil {
		return nil, nil
	}

	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: cfg.TLS.InsecureSkipVerify,
	}
	if cfg.TLS.CAFile != "" {
		caPEM, err := os.ReadFile(cfg.TLS.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read etcd CA certificate: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("failed to parse etcd CA certificate %s", cfg.TLS.CAFile)
		}
		tlsConfig.RootCAs = pool
	}
	if cfg.TLS.CertFile != "" {
		certificate, err := tls.LoadX509KeyPair(cfg.TLS.CertFile, cfg.TLS.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load etcd client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{certificate}
	}
	return tlsConfig, nil
}
