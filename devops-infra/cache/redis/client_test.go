package redis

import (
	"context"
	"testing"
)

func TestNilClientReturnsError(t *testing.T) {
	var client *Client
	ctx := context.Background()

	if _, err := client.Get(ctx, "key"); err == nil {
		t.Fatal("Get() should return an initialization error")
	}
	if err := client.Set(ctx, "key", "value", 0); err == nil {
		t.Fatal("Set() should return an initialization error")
	}
	if err := client.Delete(ctx, "key"); err == nil {
		t.Fatal("Delete() should return an initialization error")
	}
	if _, err := client.Exists(ctx, "key"); err == nil {
		t.Fatal("Exists() should return an initialization error")
	}
	if err := client.Ping(ctx); err == nil {
		t.Fatal("Ping() should return an initialization error")
	}
}
