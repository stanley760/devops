package grpc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
)

// Dial dials a gRPC service, returning a ready connection (non-blocking, reconnectable).
// It uses the built-in Unavailable exponential backoff retry strategy: ID-generation calls are idempotent-safe (retries may at most burn IDs, never reuse one).
func Dial(ctx context.Context, endpoint string, timeout time.Duration) (*grpc.ClientConn, error) {
	return DialWithOptions(ctx, endpoint, timeout)
}

// DialWithOptions dials a gRPC service and injects the OTel client stats handler by default.
// Appended DialOptions come after the defaults and can override default behavior such as transport credentials.
func DialWithOptions(ctx context.Context, endpoint string, timeout time.Duration, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	dialOptions := append([]grpc.DialOption{
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}, opts...)
	conn, err := grpc.NewClient(
		endpoint,
		dialOptions...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial gRPC %s: %w", endpoint, err)
	}
	return conn, nil
}

// IsUnavailable reports whether the error is Unavailable (connection dropped/service unreachable),
// for callers to decide whether to retry.
func IsUnavailable(err error) bool {
	return status.Code(err) == codes.Unavailable
}

// Close closes the client connection.
func Close(conn *grpc.ClientConn) error {
	if conn == nil {
		return nil
	}
	if err := conn.Close(); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("failed to close gRPC connection: %w", err)
	}
	return nil
}
