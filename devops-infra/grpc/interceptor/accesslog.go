package interceptor

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// AccessLog records access information after each gRPC request completes (Debug level).
func AccessLog() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (any, error) {
		startedAt := time.Now()
		resp, err := handler(ctx, req)

		slog.Debug("gRPC request",
			"method", info.FullMethod,
			"code", status.Code(err).String(),
			"latency", time.Since(startedAt),
		)
		return resp, err
	}
}
