package mysql

import (
	"context"
	"fmt"
)

// Ping checks the availability of the underlying connection pool; the timeout is controlled by the caller via context.
func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.db == nil {
		return fmt.Errorf("MySQL client not initialized")
	}
	pool, err := c.db.DB()
	if err != nil {
		return fmt.Errorf("failed to obtain the underlying connection pool: %w", err)
	}
	if err := pool.PingContext(ctx); err != nil {
		return fmt.Errorf("MySQL health check failed: %w", err)
	}
	return nil
}
