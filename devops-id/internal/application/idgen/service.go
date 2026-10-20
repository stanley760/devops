// Package idgen provides the application-layer entrypoint for ID generation use cases.
//
// It is currently a thin use-case seam over the domain Manager: the gRPC interface
// layer depends on this package rather than the domain package, and parameter
// validation plus domain-error-to-gRPC-status mapping stay in the interface layer
// (transport concerns). Use-case extensions (quotas, auditing, etc.) land in this
// layer without eroding the domain model.
package idgen

import (
	"context"
	iddomain "github.com/stanley760/devops/devops-id/internal/domain/idgen"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"time"
)

// Generator is the unified ID generator interface (application-layer view).
type Generator interface {
	Next(ctx context.Context, bizTag string) (int64, error)
	BatchNext(ctx context.Context, bizTag string, count int) ([]int64, error)
}

// Service is the ID generation use-case service.
type Service struct {
	mgr *iddomain.Manager

	batchLatency metric.Float64Histogram
}

// NewService creates the application service.
func NewService(mgr *iddomain.Manager) *Service {
	s := &Service{mgr: mgr}
	meter := otel.Meter("devops-id/idgen")
	h, err := meter.Float64Histogram(
		"idgen_batch_latency",
		metric.WithUnit("s"),
		metric.WithDescription("ID generation call latency (Next/BatchNext)"),
	)
	if err != nil {
		otel.Handle(err)
	}
	s.batchLatency = h
	return s
}

// Next fetches a single ID.
func (s *Service) Next(ctx context.Context, bizTag string) (int64, error) {
	startedAt := time.Now()
	id, err := s.mgr.Next(ctx, bizTag)
	s.recordLatency(ctx, bizTag, "next", time.Since(startedAt))
	return id, err
}

// BatchNext atomically fetches count strictly increasing IDs (a monotonic exclusive batch).
func (s *Service) BatchNext(ctx context.Context, bizTag string, count int) ([]int64, error) {
	startedAt := time.Now()
	ids, err := s.mgr.BatchNext(ctx, bizTag, count)
	s.recordLatency(ctx, bizTag, "batch_next", time.Since(startedAt))
	return ids, err
}

func (s *Service) recordLatency(ctx context.Context, bizTag, method string, d time.Duration) {
	if s.batchLatency == nil {
		return
	}
	s.batchLatency.Record(ctx, d.Seconds(), metric.WithAttributes(
		attribute.String("biz_tag", bizTag),
		attribute.String("method", method),
	))
}
