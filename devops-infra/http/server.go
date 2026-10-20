package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	commonResp "github.com/stanley760/devops/devops-common/response"
	"github.com/stanley760/devops/devops-infra/config"
	"github.com/stanley760/devops/devops-infra/errs"
	"github.com/stanley760/devops/devops-infra/http/middleware"
	"github.com/stanley760/devops/devops-infra/http/response"
	"github.com/stanley760/devops/devops-infra/observability"
)

type Register func(r *gin.Engine)

type ReadinessCheck struct {
	Name  string
	Check func(context.Context) error
}

func Run(cfg config.ServerConfig, register Register, readiness ...ReadinessCheck) error {
	return RunWithOptions(cfg, register, nil, nil, readiness...)
}

// GuardHooks holds business-side global middleware injected after the ops
// endpoints are mounted and before business routes are registered.
//
// This is an extension point rather than a hard-coded traffic guard in infra:
// the guard (rejecting direct requests that bypass the gateway) is a policy
// only required by services exposing HTTP externally, and the Secret is owned
// by business configuration; infra should not know about concrete business
// security switches.
type GuardHooks struct {
	// Guards are registered in order after the ops endpoints
	// (/healthz /readyz /metrics) are mounted.
	// These endpoints do not go through the gateway, so they must be mounted
	// before the guards; otherwise health checks and Prometheus scraping would be blocked.
	Guards []gin.HandlerFunc
}

// RunWithOptions wires the OTel trace/RED middleware and /metrics into the
// existing HTTP assembly.
// The observability Runtime may be nil, keeping existing callers unchanged;
// a nil guard registers no business guards, likewise keeping existing callers unchanged.
func RunWithOptions(cfg config.ServerConfig, register Register, obs *observability.Runtime, guard *GuardHooks, readiness ...ReadinessCheck) error {
	gin.SetMode(ginMode())
	r, err := newRouter(cfg, register, readiness, obs, guard)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      r,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	errCh := make(chan error, 1)
	go func() {
		slog.Info("HTTP server started", "addr", srv.Addr, "mode", gin.Mode())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	select {
	case err := <-errCh:
		return fmt.Errorf("HTTP server error: %w", err)
	case <-ctx.Done():
	}
	slog.Info("shutdown signal received, starting graceful shutdown")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}
	slog.Info("HTTP server stopped")
	return nil
}

func newRouter(cfg config.ServerConfig, register Register, readiness []ReadinessCheck, obs *observability.Runtime, guard ...*GuardHooks) (*gin.Engine, error) {
	r := gin.New()
	if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		return nil, fmt.Errorf("failed to configure trusted proxies: %w", err)
	}
	if obs != nil {
		r.Use(obs.Middleware())
		r.Use(obs.MetricsMiddleware())
	}
	r.Use(middleware.Recovery())
	r.Use(middleware.AccessLog())
	r.HandleMethodNotAllowed = true
	r.NoRoute(func(c *gin.Context) { response.Fail(c, errs.CodeNotFound, "endpoint does not exist") })
	r.NoMethod(func(c *gin.Context) { response.Fail(c, errs.CodeMethodNotAllowed, errs.ErrMethodNotAllowed.Msg) })
	r.GET("/healthz", func(c *gin.Context) { response.OK(c, gin.H{"status": "ok"}) })
	if obs != nil && obs.MetricsHandler != nil {
		r.GET("/metrics", gin.WrapH(obs.MetricsHandler))
	}
	r.GET("/readyz", func(c *gin.Context) {
		var ready, unready []string
		for _, check := range readiness {
			if check.Check == nil {
				ready = append(ready, check.Name)
				continue
			}
			checkCtx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
			err := check.Check(checkCtx)
			cancel()
			if err != nil {
				unready = append(unready, check.Name)
				continue
			}
			ready = append(ready, check.Name)
		}
		if len(unready) == 0 {
			response.OK(c, gin.H{"status": "ready", "ready": ready})
			return
		}
		c.JSON(http.StatusServiceUnavailable, commonResp.BaseResponse[any]{
			Code: http.StatusServiceUnavailable,
			Msg:  "service not ready",
			Data: gin.H{"status": "unready", "ready": ready, "unready": unready},
		})
	})
	// Business guards must run after the ops endpoints but before business routes:
	// the former serve direct liveness probes, the latter force traffic through the gateway.
	if len(guard) > 0 && guard[0] != nil {
		for _, g := range guard[0].Guards {
			if g != nil {
				r.Use(g)
			}
		}
	}
	if register != nil {
		register(r)
	}
	return r, nil
}

func ginMode() string {
	switch config.Env() {
	case "prod", "PROD":
		return gin.ReleaseMode
	default:
		return gin.DebugMode
	}
}
