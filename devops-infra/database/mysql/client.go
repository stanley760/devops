// Package mysql provides a MySQL database adapter: connection pool, transactions, and health checks.
//
// Prefer Client in new code with explicit injection at the cmd layer;
// Init/Get/Close are transitional interfaces kept for existing callers.
package mysql

import (
	"context"
	"fmt"
	"log/slog"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	database "github.com/stanley760/devops/devops-infra/database"
)

// Client wraps a GORM MySQL connection pool.
type Client struct {
	db *gorm.DB
}

// New establishes a connection and applies connection pool and GORM log settings.
func New(cfg Config) (*Client, error) {
	cfg, err := normalizeConfig(cfg)
	if err != nil {
		return nil, err
	}

	db, err := gorm.Open(mysql.Open(configDSN(cfg)), &gorm.Config{
		Logger: gormlogger.Default.LogMode(configGormLogLevel(cfg)),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MySQL: %w", err)
	}

	pool, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to obtain the underlying connection pool: %w", err)
	}
	pool.SetMaxIdleConns(cfg.MaxIdleConns)
	pool.SetMaxOpenConns(cfg.MaxOpenConns)
	pool.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	pool.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	slog.Info("connected to MySQL",
		"addr", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		"database", cfg.Database)
	return &Client{db: db}, nil
}

// DB returns the underlying GORM connection for existing Repositories or migration tools.
func (c *Client) DB() *gorm.DB {
	if c == nil {
		return nil
	}
	return c.db
}

// Session returns the transaction in the current context; when there is no transaction, returns a connection pool session.
func (c *Client) Session(ctx context.Context) *gorm.DB {
	if tx, ok := database.ExecutorFromContext(ctx); ok {
		return tx
	}
	return c.db.WithContext(ctx)
}

// Transaction starts a transaction and injects tx into fn's context, which Repositories reuse via Session.
func (c *Client) Transaction(ctx context.Context, fn database.TxFunc) error {
	if c == nil || c.db == nil {
		return fmt.Errorf("MySQL client not initialized")
	}
	return database.Transaction(ctx, c.db.WithContext(ctx), fn)
}

// Close closes the connection pool.
func (c *Client) Close() error {
	if c == nil || c.db == nil {
		return nil
	}
	pool, err := c.db.DB()
	if err != nil {
		return err
	}
	return pool.Close()
}

var current *Client

// Init establishes the global connection, compatible with existing commands and legacy callers.
func Init(cfg Config) error {
	client, err := New(cfg)
	if err != nil {
		return err
	}
	current = client
	return nil
}

// Default returns the current global client.
func Default() *Client { return current }

// Get returns the current underlying GORM connection; nil when Init has not been called.
func Get() *gorm.DB {
	return current.DB()
}

// Close closes the global connection pool.
func Close() error {
	client := current
	current = nil
	return client.Close()
}
