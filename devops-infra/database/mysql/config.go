package mysql

import (
	"fmt"
	"github.com/stanley760/devops/devops-infra/config"
	"gorm.io/gorm/logger"
	"time"
)

// Config is the MySQL connection config. The type alias is kept here
// so that service config continues to be loaded uniformly by devops-infra/config.
type Config = config.MysqlConfig

// normalize fills in defaults and validates required fields, avoiding deferring connection errors to the dialing stage.
func normalizeConfig(cfg Config) (Config, error) {
	if cfg.Host == "" {
		return cfg, fmt.Errorf("mysql.host must not be empty")
	}
	if cfg.Port <= 0 {
		return cfg, fmt.Errorf("mysql.port must be greater than 0")
	}
	if cfg.Username == "" {
		return cfg, fmt.Errorf("mysql.username must not be empty")
	}
	if cfg.Database == "" {
		return cfg, fmt.Errorf("mysql.database must not be empty")
	}
	if cfg.Charset == "" {
		cfg.Charset = "utf8mb4"
	}
	if cfg.MaxIdleConns == 0 {
		cfg.MaxIdleConns = 2
	}
	if cfg.ConnMaxLifetime == 0 {
		cfg.ConnMaxLifetime = time.Hour
	}
	if cfg.ConnMaxIdleTime == 0 {
		cfg.ConnMaxIdleTime = time.Minute
	}
	return cfg, nil
}

// dsn builds the Go MySQL Driver DSN.
func configDSN(cfg Config) string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database, cfg.Charset,
	)
}

// gormLogLevel parses the GORM log level; empty defaults to warn.
func configGormLogLevel(cfg Config) logger.LogLevel {
	switch cfg.LogLevel {
	case "silent":
		return logger.Silent
	case "error":
		return logger.Error
	case "info":
		return logger.Info
	default:
		return logger.Warn
	}
}
