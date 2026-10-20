package redis

import (
	"context"
	"fmt"
)

// Ping checks Redis connectivity; the timeout is controlled by the caller via context.
func (client *Client) Ping(ctx context.Context) error {
	if client == nil || client.client == nil {
		return fmt.Errorf("Redis client not initialized")
	}
	if err := client.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("Redis health check failed: %w", err)
	}
	return nil
}
