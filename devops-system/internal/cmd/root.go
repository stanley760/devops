// Package cmd defines the command-line entrypoint for devops-system.
//
// Configuration management: the path is bound by -f/--config-file, with
// loading and reading encapsulated in devops-infra/config; loading happens
// in cobra.OnInitialize and component initialization in PersistentPreRunE.
package cmd

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"

	infracfg "github.com/stanley760/devops/devops-infra/config"
	grpcinfra "github.com/stanley760/devops/devops-infra/grpc"
	httpinfra "github.com/stanley760/devops/devops-infra/http"
	"github.com/stanley760/devops/devops-infra/logging"
	"github.com/stanley760/devops/devops-infra/nacos"
	"github.com/stanley760/devops/devops-infra/observability"
	"github.com/stanley760/devops/devops-infra/security/gateway"
	"github.com/stanley760/devops/devops-infra/security/humakit"
	securityjwt "github.com/stanley760/devops/devops-infra/security/jwt"
	securitysession "github.com/stanley760/devops/devops-infra/security/session"

	client "github.com/stanley760/devops/devops-infra/database/mysql"

	"github.com/stanley760/devops/devops-id/pkg/idclient"
	"github.com/stanley760/devops/devops-system/internal/app"
	sysaudit "github.com/stanley760/devops/devops-system/internal/audit"
	"github.com/stanley760/devops/devops-system/internal/auth"
	"github.com/stanley760/devops/devops-system/internal/authz"
	"github.com/stanley760/devops/devops-system/internal/config"
	"github.com/stanley760/devops/devops-system/internal/dbinit"
	"github.com/stanley760/devops/devops-system/internal/middleware"
	"github.com/stanley760/devops/devops-system/internal/model"
	"github.com/stanley760/devops/devops-system/internal/router"
	"github.com/stanley760/devops/devops-system/internal/store"
	"github.com/stanley760/devops/devops-system/internal/tenantdb"
)

var confFile string

var obsRuntime *observability.Runtime

var rootCmd = &cobra.Command{
	Use:   "devops-system",
	Short: "devops user and permission system",

	// Runs before any subcommand: initializes logging
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var cfg config.Config
		if err := infracfg.LoadInto(&cfg); err != nil {
			return err
		}
		if err := loadNacosConfig(&cfg); err != nil {
			return err
		}
		if err := logging.Init(cfg.Log); err != nil {
			return err
		}
		obs, err := observability.Init(cmd.Context(), observability.Config{
			ServiceName:    cfg.Nacos.Registry.ServiceName,
			Env:            infracfg.Env(),
			OTLPEndpoint:   cfg.Observability.OTLPEndpoint,
			SamplerRatio:   cfg.Observability.SamplerRatio,
			MetricsEnabled: cfg.Observability.MetricsEnabled,
		})
		if err != nil {
			return err
		}
		obsRuntime = obs

		slog.Debug("initialization complete",
			"service", "devops-system",
			"config", infracfg.Files(),
			"env", infracfg.Env(),
			"logLevel", cfg.Log.Level,
		)
		return nil
	},

	// Unified teardown after any subcommand, ensuring logs are not lost
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		_ = nacos.Close()
		if obsRuntime != nil {
			_ = obsRuntime.ShutdownWithTimeout(context.Background(), 5*time.Second)
			obsRuntime = nil
		}
		return logging.Close()
	},

	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

// loadNacosConfig fetches and merges remote Nacos configuration, remote
// first: a remote dataID hit takes precedence; only when the remote misses
// the profile dataID (<service>-<env>.yaml) is the local same-named env file
// merged as a fallback. bootstrap.yaml always serves as the common baseline.
func loadNacosConfig(cfg *config.Config) error {
	if !cfg.Nacos.Enabled {
		return nil
	}
	if err := nacos.Init(cfg.Nacos); err != nil {
		return err
	}
	dataIDs := infracfg.RemoteDataIDs(cfg.Nacos.ConfigCenter.DataIDs)
	profileDataID := infracfg.ProfileDataID()
	loaded := false
	profileLoaded := false
	for _, dataID := range dataIDs {
		content, err := nacos.Fetch(dataID)
		if err != nil {
			if cfg.Nacos.ConfigCenter.Required {
				return err
			}
			continue
		}
		if content == "" {
			continue
		}
		if err := infracfg.MergeRemoteYAML(dataID, content); err != nil {
			return err
		}
		loaded = true
		if dataID == profileDataID {
			profileLoaded = true
		}
	}
	if cfg.Nacos.ConfigCenter.Required && !loaded {
		return fmt.Errorf("nacos config center returned no configuration")
	}
	if !profileLoaded {
		// Remote missed the profile config: merge the local <service>-<env>.yaml fallback
		if err := infracfg.MergeEnvConfig(); err != nil {
			return err
		}
	}
	return infracfg.LoadInto(cfg)
}

func init() {
	rootCmd.PersistentFlags().StringVarP(
		&confFile, "config-file", "f", "config/bootstrap.yaml", "the bootstrap config file",
	)
}

// Execute runs the root command. version is used for the --version output.
func Execute(version string) {
	rootCmd.Version = version

	// cobra.OnInitialize runs before the Run of every command; configuration loading happens here
	cobra.OnInitialize(func() {
		if err := infracfg.Init(confFile); err != nil {
			panic(err)
		}
	})
	rootCmd.AddCommand(serverCmd, migrateCmd)
	if err := rootCmd.Execute(); err != nil {
		cobra.CheckErr(err)
	}
}

// serverCmd starts the HTTP service.
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the HTTP service",
	RunE: func(cmd *cobra.Command, args []string) (runErr error) {
		type cleanup struct {
			name string
			run  func() error
		}
		var cleanups []cleanup
		addCleanup := func(name string, run func() error) {
			cleanups = append(cleanups, cleanup{name: name, run: run})
		}
		defer func() {
			for i := len(cleanups) - 1; i >= 0; i-- {
				if err := cleanups[i].run(); err != nil {
					runErr = errors.Join(runErr, fmt.Errorf("failed to clean up %s: %w", cleanups[i].name, err))
				}
			}
		}()

		var cfg config.Config
		if err := infracfg.LoadInto(&cfg); err != nil {
			return err
		}

		// Assembly chain: db → TenantPlugin(fail-closed) → startup initialization
		//   (schema SQL + seed SQL, internal/dbinit) → authz → store (Redis sessions)
		//   → shared security (RS256 JWT + session) → idclient (ID issuance) → auth/service → httpx
		if err := client.Init(cfg.Mysql); err != nil {
			return err
		}
		addCleanup("mysql", client.Close)
		if err := obsRuntime.InstrumentGORM(client.Get(), cfg.Mysql.Database); err != nil {
			return err
		}
		if err := tenantdb.RegisterPlugin(client.Get()); err != nil {
			return fmt.Errorf("failed to register tenant filter plugin: %w", err)
		}
		// Startup initialization (see the internal/dbinit package docs): schema SQL
		// + seed SQL are skipped per table existence (existing tables are not
		// re-run) → casbin. Nacos registration only accepts traffic once everything
		// is ready; on failure it fails closed and refuses to start.
		if err := dbinit.Run(client.Get()); err != nil {
			return fmt.Errorf("startup schema/seed failed: %w", err)
		}
		if err := authz.Init(client.Get()); err != nil {
			return err
		}
		addCleanup("authz", func() error {
			authz.Close()
			return nil
		})
		if err := authz.SeedPolicies(1, 1); err != nil { // idempotent (AddPolicyIfAbsent)
			return fmt.Errorf("casbin policy seeding failed: %w", err)
		}
		if err := store.Init(cfg.Redis); err != nil {
			return err
		}
		addCleanup("redis", store.Close)
		if err := obsRuntime.InstrumentRedis(store.Get()); err != nil {
			return err
		}

		// Shared security wiring: configured private key → self-verifying KeySet
		// (single issuer; multi-public-key/JWKS can be added during rotation)
		pub, err := securityjwt.PublicKeyFromPrivatePEM([]byte(cfg.JWT.SigningKey))
		if err != nil {
			return fmt.Errorf("failed to parse JWT private key (jwt.signing_key must be an RSA private key PEM): %w", err)
		}
		keySet, err := securityjwt.NewStaticKeySet(map[string]*rsa.PublicKey{
			cfg.JWT.EffectiveKeyID(): pub,
		})
		if err != nil {
			return fmt.Errorf("failed to build JWT KeySet: %w", err)
		}
		tokenMgr, err := securityjwt.NewManager(cfg.JWT.SharedJWTConfig(keySet))
		if err != nil {
			return fmt.Errorf("failed to initialize shared JWT manager: %w", err)
		}
		sessionStore := securitysession.NewRedisClientStore(store.Get(), cfg.JWT.RefreshExpire)
		authM := auth.New(cfg.JWT, tokenMgr, sessionStore)
		conn, err := grpcinfra.Dial(cmd.Context(), cfg.IDClient.Endpoint, cfg.IDClient.DialTimeout)
		if err != nil {
			return err
		}
		addCleanup("devops-id connection", func() error { return grpcinfra.Close(conn) })
		idc := idclient.New(conn, idclient.BufferConfig{
			Batch:       cfg.IDClient.Batch,
			LowWater:    cfg.IDClient.LowWater,
			CallTimeout: cfg.IDClient.CallTimeout,
		})
		model.ConfigureModel(idc)

		// Business middleware suite (D7: business dependencies are injected via Deps; nil items get production defaults from NewKit).
		kit := middleware.NewKit(authM, middleware.Deps{})

		// Operation audit wiring: the Sink writes this tenant's t_system_operation_log,
		// and the collection scope is decided by the route whitelist in the router
		// package (write operations only, see audit_modules.go). Auditing is a
		// side path: write failures only log at slog.Warn and do not affect business
		// responses.
		kit.EnableAudit(humakit.AuditDeps{
			Sink:    sysaudit.NewSink(),
			Resolve: router.AuditModuleResolver(),
		})

		serviceName := cfg.Nacos.Registry.ServiceName
		if serviceName == "" {
			return fmt.Errorf("nacos.registry.service_name is not configured")
		}
		if err := nacos.Register(serviceName, cfg.Server.Port, cfg.Nacos.Registry.Meta); err != nil {
			return err
		}
		addCleanup("nacos service registration", func() error {
			return nacos.Deregister(serviceName, cfg.Server.Port)
		})

		readiness := []httpinfra.ReadinessCheck{
			{Name: "mysql", Check: client.Default().Ping},
			{Name: "redis", Check: store.Ping},
			{Name: "devops-id", Check: idc.Ping},
		}
		register := app.InitializeHTTP(cfg, kit, authM, store.Get())

		// Traffic-entry enforcement (third layer): requests must carry a Bearer JWT
		// that validates independently.
		//
		// The gateway side is already verified by the Higress jwt-auth plugin
		// (deploy/higress/wasmplugins/jwt-auth-2.0.0.yaml); this layer is
		// defense in depth: the compose topology has no NetworkPolicy, so the
		// gateway can be bypassed by containers on the same network, and a bypass
		// goes through no plugins at all — hence the application layer validates
		// once more. jwt-auth is mounted per Ingress resource and cannot express
		// per-path policies, so this layer takes on fine-grained enforcement.
		//
		// Endpoints on the allowlist bypass the gateway and can still be reached
		// directly:
		//   - Health checks and metrics: container orchestration probes and
		//     Prometheus scraping;
		//   - login / refresh / jwks.json: anonymous endpoints, where the user has
		//     no token yet (kept in line with devops-system-public.yaml, otherwise
		//     nobody could log in);
		//   - apidoc mount point: enabled in dev/test only (controlled by
		//     apidoc.enabled).
		guard := &httpinfra.GuardHooks{Guards: []gin.HandlerFunc{
			gateway.EnforceFromConfig(cfg.Gateway, authM,
				[]string{
					"/healthz", "/readyz", "/metrics",
					router.AuthLoginPath, router.AuthRefreshPath, router.JWKSPath,
					// Captcha issuance must be anonymous: the login page fetches the
					// challenge before it has any credential, so it sits in the same
					// anonymous set as login/refresh. Missing it makes the captcha
					// endpoint return 401 and the whole login page unusable.
					router.AuthCaptchaPath,
				},
				[]string{cfg.APIDoc.BasePath},
			),
		}}
		return httpinfra.RunWithOptions(cfg.Server, register, obsRuntime, guard, readiness...)
	},
}
