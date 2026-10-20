// Package grpc implements the IdGen gRPC service: transport-layer parameter
// validation, calls into the application service, and domain-error-to-gRPC-status mapping.
package grpc

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	idapp "github.com/stanley760/devops/devops-id/internal/application/idgen"
	iddomain "github.com/stanley760/devops/devops-id/internal/domain/idgen"
	"github.com/stanley760/devops/devops-id/pkg/idgenpb"
)

// maxBatchCount is the per-request batch limit (shared with the domain-layer constant to prevent drift).
const maxBatchCount = iddomain.DefaultMaxBatchCount

// IdGenServer is the IdGen gRPC implementation.
type IdGenServer struct {
	idgenpb.UnimplementedIdGenServer
	gen idapp.Generator
}

// NewIdGenServer creates the implementation.
func NewIdGenServer(gen idapp.Generator) *IdGenServer {
	return &IdGenServer{gen: gen}
}

// Next fetches a single ID.
func (s *IdGenServer) Next(ctx context.Context, req *idgenpb.NextRequest) (*idgenpb.NextReply, error) {
	if req.GetBizTag() == "" {
		return nil, status.Error(codes.InvalidArgument, "biz_tag must not be empty")
	}
	startedAt := time.Now()
	id, err := s.gen.Next(ctx, req.GetBizTag())
	logCall(ctx, "next", req.GetBizTag(), 1, id, id, err, time.Since(startedAt))
	if err != nil {
		return nil, mapErr(err)
	}
	return &idgenpb.NextReply{Id: id}, nil
}

// BatchNext allocates count monotonically increasing IDs (a monotonic exclusive batch:
// strictly increasing within the batch, no interleaving from concurrent calls, not
// contiguous across milliseconds).
func (s *IdGenServer) BatchNext(ctx context.Context, req *idgenpb.BatchNextRequest) (*idgenpb.BatchNextReply, error) {
	if req.GetBizTag() == "" {
		return nil, status.Error(codes.InvalidArgument, "biz_tag must not be empty")
	}
	if req.GetCount() < 1 {
		return nil, status.Error(codes.InvalidArgument, "count must be >= 1")
	}
	if req.GetCount() > maxBatchCount {
		return nil, status.Errorf(codes.InvalidArgument, "count exceeds the per-request maximum %d", maxBatchCount)
	}
	startedAt := time.Now()
	ids, err := s.gen.BatchNext(ctx, req.GetBizTag(), int(req.GetCount()))
	if err != nil {
		logCall(ctx, "batch_next", req.GetBizTag(), int(req.GetCount()), 0, 0, err, time.Since(startedAt))
		return nil, mapErr(err)
	}
	logCall(ctx, "batch_next", req.GetBizTag(), int(req.GetCount()), ids[0], ids[len(ids)-1], nil, time.Since(startedAt))
	return &idgenpb.BatchNextReply{Ids: ids}, nil
}

// logCall writes one line of ID generation call log, filling in the business semantics
// missing from the infra AccessLog: who (client) fetched how many IDs for which biz_tag,
// plus latency and the resulting ID range (batches are strictly increasing, so first/last
// are the range bounds). Division of labor with AccessLog -- AccessLog is the
// "RPC completed" marker (method/code/total latency); this line is the business context,
// and when investigating issues like "handler hang vs response lost in the transport
// layer" the two lines together form the complete chain. Debug on success (ID generation
// is high-frequency, so no info-level log spam); Warn on failure (mapErr only maps to a
// status code, and the original error is preserved only here).
// Carries ctx: observability's loghandler automatically injects trace_id/span_id,
// enabling trace-based correlation with the caller's (devops-system) logs.
func logCall(ctx context.Context, method, bizTag string, count int, firstID, lastID int64, err error, d time.Duration) {
	client := ""
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		client = p.Addr.String()
	}
	if err != nil {
		slog.WarnContext(ctx, "ID generation call failed",
			"method", method, "biz_tag", bizTag, "count", count,
			"client", client, "err", err, "latency", d)
		return
	}
	slog.DebugContext(ctx, "ID generation call",
		"method", method, "biz_tag", bizTag, "count", count,
		"first_id", firstID, "last_id", lastID, "client", client, "latency", d)
}

// mapErr maps ID generation layer errors to gRPC status codes:
//   - context.Canceled / DeadlineExceeded -> Canceled / DeadlineExceeded
//   - invalid parameters (ErrInvalidName, ErrBatchTooLarge) -> InvalidArgument
//   - retryable transient failures (clock rollback / lease expiry / slot taken /
//     infra unavailable / no cached slot / manager closed) -> Unavailable
//   - resource exhaustion (ErrSlotsExhausted, ErrTooManyTags) -> ResourceExhausted
//   - corrupted slot mapping (requires operator intervention) -> FailedPrecondition
//   - everything else -> Internal
func mapErr(err error) error {
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, "request canceled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "request timed out")
	}
	if errors.Is(err, iddomain.ErrInvalidName) {
		return status.Error(codes.InvalidArgument, "invalid biz_tag")
	}
	if errors.Is(err, iddomain.ErrBatchTooLarge) {
		return status.Error(codes.InvalidArgument, "batch size exceeds the maximum")
	}
	if errors.Is(err, iddomain.ErrClockBackward) {
		return status.Error(codes.Unavailable, "clock rollback detected, please retry later")
	}
	if errors.Is(err, iddomain.ErrLeaseLost) {
		return status.Error(codes.Unavailable, "generator lease expired (slot may have been taken), please retry later")
	}
	if errors.Is(err, iddomain.ErrManagerClosed) {
		return status.Error(codes.Unavailable, "generator closed, please retry later")
	}
	if errors.Is(err, iddomain.ErrSlotOccupied) {
		return status.Error(codes.Unavailable, "worker slot occupied, please retry later")
	}
	if errors.Is(err, iddomain.ErrInfraUnavailable) {
		return status.Error(codes.Unavailable, "ID generation infrastructure unavailable, please retry later")
	}
	if errors.Is(err, iddomain.ErrNoCachedSlot) {
		return status.Error(codes.Unavailable, "generator fallback unavailable (no cached slot), please retry later")
	}
	if errors.Is(err, iddomain.ErrSlotsExhausted) {
		return status.Error(codes.ResourceExhausted, "worker slots exhausted")
	}
	if errors.Is(err, iddomain.ErrTooManyTags) {
		return status.Error(codes.ResourceExhausted, "biz_tag count has reached the maximum")
	}
	if errors.Is(err, iddomain.ErrSlotMappingInvalid) {
		return status.Error(codes.FailedPrecondition, "slot mapping corrupted, please contact operations")
	}
	return status.Error(codes.Internal, "ID generation failed")
}
