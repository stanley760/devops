// Package cmd defines the devops-user command-line entrypoint.
//
// Config management: the path is bound by -f/--config-file, loading and
// reading are encapsulated in devops-infra/config, loading happens in
// cobra.OnInitialize, and component initialization in PersistentPreRunE.
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

	"github.com/stanley760/devops/devops-infra/captcha"
	infracfg "github.com/stanley760/devops/devops-infra/config"
	grpcinfra "github.com/stanley760/devops/devops-infra/grpc"
	httpinfra "github.com/stanley760/devops/devops-infra/http"
	"github.com/stanley760/devops/devops-infra/logging"
	"github.com/stanley760/devops/devops-infra/nacos"
	"github.com/stanley760/devops/devops-infra/observability"
	"github.com/stanley760/devops/devops-infra/security/gateway"
	securityjwt "github.com/stanley760/devops/devops-infra/security/jwt"
	securitysession "github.com/stanley760/devops/devops-infra/security/session"

	client "github.com/stanley760/devops/devops-infra/database/mysql"

	"github.com/stanley760/devops/devops-id/pkg/idclient"
	"github.com/stanley760/devops/devops-user/internal/auth"
	"github.com/stanley760/devops/devops-user/internal/config"
	"github.com/stanley760/devops/devops-user/internal/dbinit"
	"github.com/stanley760/devops/devops-user/internal/loginlock"
	"github.com/stanley760/devops/devops-user/internal/middleware"
	"github.com/stanley760/devops/devops-user/internal/model"
	cendoauth2 "github.com/stanley760/devops/devops-user/internal/oauth2"
	"github.com/stanley760/devops/devops-user/internal/refreshtoken"
	"github.com/stanley760/devops/devops-user/internal/router"
	"github.com/stanley760/devops/devops-user/internal/service"
	"github.com/stanley760/devops/devops-user/internal/store"
)

var confFile string

var obsRuntime *observability.Runtime

// refreshKeyPrefix is the Redis key prefix for refresh tokens and login
// rate-limit counters. It keeps the same style as devops-infra/security/session's
// `t{tid}:auth:*` namespace; the C side is single-tenant, so it is fixed at
// t1 (matching auth.DefaultTenantID).
const refreshKeyPrefix = "t1"

var rootCmd = &cobra.Command{
	Use:   "devops-user",
	Short: "devops C-side user service",

	// This runs before every subcommand: load config, initialize logging and
	// observability.
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
			"service", "devops-user",
			"config", infracfg.Files(),
			"env", infracfg.Env(),
			"logLevel", cfg.Log.Level,
		)
		return nil
	},

	// Unified teardown after every subcommand, ensuring logs are not lost.
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

// loadNacosConfig fetches remote Nacos config and merges it, remote first:
// when a remote dataID hits, the remote value wins; only when the remote
// profile dataID (<service>-<env>.yaml) misses is the local same-named
// environment file merged as a fallback. bootstrap.yaml always serves as the
// common baseline.
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
		// Remote profile config missed: merge the local <service>-<env>.yaml
		// as a fallback
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

// Execute runs the root command. version is used for --version output.
func Execute(version string) {
	rootCmd.Version = version

	// cobra.OnInitialize runs before each command's Run; config loading
	// happens here.
	cobra.OnInitialize(func() {
		if err := infracfg.Init(confFile); err != nil {
			panic(err)
		}
	})
	rootCmd.AddCommand(serverCmd)
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
					runErr = errors.Join(runErr, fmt.Errorf("cleanup of %s failed: %w", cleanups[i].name, err))
				}
			}
		}()

		var cfg config.Config
		if err := infracfg.LoadInto(&cfg); err != nil {
			return err
		}

		// Assembly chain: db → dbinit (create tables) → idclient (ID generation)
		//   → store (Redis sessions) → shared security (RS256 JWT + session) →
		//   business middleware kit → service → router.
		if err := client.Init(cfg.Mysql); err != nil {
			return err
		}
		addCleanup("mysql", client.Close)
		if obsRuntime != nil {
			if err := obsRuntime.InstrumentGORM(client.Get(), cfg.Mysql.Database); err != nil {
				return err
			}
		}

		// Create tables (no seed data): skipped based on table existence; existing
		// tables are not executed again. Done before Nacos registration — traffic
		// should not be served before the tables exist.
		if err := dbinit.Run(client.Get()); err != nil {
			return fmt.Errorf("table creation at startup failed: %w", err)
		}

		// ID generation service (primary keys are allocated by devops-id, not
		// auto-increment). Wired after dbinit: table creation needs no IDs (no
		// seed data), but runtime inserts do.
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

		if err := store.Init(cfg.Redis); err != nil {
			return err
		}
		addCleanup("redis", store.Close)
		if obsRuntime != nil {
			if err := obsRuntime.InstrumentRedis(store.Get()); err != nil {
				return err
			}
		}

		// Shared security wiring: configured private key → self-verifying KeySet
		// (single issuer; can extend to multiple public keys/JWKS during rotation)
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
			return fmt.Errorf("failed to initialize the shared JWT manager: %w", err)
		}
		sessionStore := securitysession.NewRedisClientStore(store.Get(), cfg.JWT.RefreshExpire)

		// The refresh token store (opaque random string, not JWT) and the login
		// rate limiter both reuse the same Redis with the t1 prefix.
		refreshStore := refreshtoken.NewStore(store.Get(), refreshKeyPrefix, cfg.JWT.RefreshExpire)

		// 身份来源。默认是网关模式：JWT 只在网关验一次——网关验签后把已验证的声明
		// 放进请求头传下来，本服务不再验签，只查会话是否仍然有效（所以「踢下线」
		// 依然立即生效）。jwt 模式是旧路径，供前面没有网关的环境使用。
		var authM *auth.Manager
		var guards []gin.HandlerFunc
		switch gateway.ResolveIdentitySource(cfg.Gateway) {
		case gateway.IdentitySourceGateway:
			authM = auth.NewGatewayIdentity(cfg.JWT, tokenMgr, sessionStore, refreshStore)
			// 只有来自可信网关的请求才带身份；其余请求没有身份，认证环节直接 401。
			guards = []gin.HandlerFunc{gateway.IdentityMiddleware(cfg.Server.TrustedProxies)}
		default:
			authM = auth.New(cfg.JWT, tokenMgr, sessionStore, refreshStore)
			if cfg.Gateway.RequireJWT {
				guards = []gin.HandlerFunc{gateway.EnforceFromConfig(
					cfg.Gateway, authM, gateway.ExemptPathsFor(cfg.Gateway), []string{cfg.APIDoc.BasePath},
				)}
			}
		}

		// Business middleware kit (the C side has no RBAC/tenants; it only
		// injects envelope messages).
		kit := middleware.NewKit(authM, middleware.Deps{})

		// Business use cases
		limiter := loginlock.New(store.Get(), refreshKeyPrefix, loginlock.Config{
			Enabled:      cfg.LoginLock.Enabled,
			MaxIPFails:   cfg.LoginLock.MaxIPFails,
			IPWindow:     cfg.LoginLock.IPWindow,
			MaxUserFails: cfg.LoginLock.MaxUserFails,
			UserWindow:   cfg.LoginLock.UserWindow,
			LockTTL:      cfg.LoginLock.LockTTL,
		})
		// Captcha: the answer store shares the Redis connection with the session
		// store. The key prefix is namespaced per side so a captcha issued by the
		// C-side can never be replayed against the B-side (both share one Redis
		// and therefore one keyspace).
		captchaSvc := service.NewCaptcha(captcha.NewRedisStore(store.Get(), refreshKeyPrefix+":captcha"))
		authSvc := service.NewAuth(authM, limiter, captchaSvc)
		profileSvc := service.NewProfile(authM, limiter)

		// Third-party login (GitHub / Google / WeChat).
		// Providers with incomplete credentials are still registered but
		// Enabled()=false, and the frontend hides their entries accordingly.
		oauthProviders := map[string]cendoauth2.Provider{}
		autoReg := map[string]bool{}
		prefixes := map[string]string{}
		for name, pc := range map[string]config.ProviderConfig{
			"github": cfg.OAuth.Github,
			"google": cfg.OAuth.Google,
			"wechat": cfg.OAuth.Wechat,
		} {
			oc := cendoauth2.Config{
				ClientID:       pc.ClientID,
				ClientSecret:   pc.ClientSecret,
				RedirectURL:    pc.RedirectURL,
				Scopes:         pc.Scopes,
				AutoRegister:   pc.AutoRegister,
				NicknamePrefix: pc.NicknamePrefix,
			}
			switch name {
			case "github":
				oauthProviders[name] = cendoauth2.NewGitHubProvider(oc)
			case "google":
				oauthProviders[name] = cendoauth2.NewGoogleProvider(oc)
			case "wechat":
				oauthProviders[name] = cendoauth2.NewWeChatProvider(oc)
			}
			autoReg[name] = pc.AutoRegister
			prefixes[name] = pc.NicknamePrefix
		}
		stateStore := cendoauth2.NewRedisStateStore(store.Get(), refreshKeyPrefix, cfg.OAuth.StateTTL)
		oauthSvc := service.NewOAuth(service.OAuthDeps{
			AuthM:             authM,
			Providers:         oauthProviders,
			States:            stateStore,
			StateTTL:          cfg.OAuth.StateTTL,
			AutoRegister:      autoReg,
			NicknamePrefix:    prefixes,
			RedirectWhitelist: cfg.OAuth.RedirectWhitelist,
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
			// The ID generation service is a hard dependency of the write path
			// (primary keys are not auto-increment): when devops-id is
			// unavailable, registration and other inserts fail fast, so readyz
			// must reflect that and the orchestrator can pull traffic early.
			{Name: "devops-id", Check: idc.Ping},
		}
		register := router.NewRegister(cfg.APIDoc, authM, router.Modules{
			Auth:    router.NewAuthRoutes(authSvc, captchaSvc, kit),
			Profile: router.NewProfileRoutes(profileSvc, kit),
			OAuth:   router.NewOAuthRoutes(oauthSvc, kit),
		})

		// 流量入口守卫。两种模式：
		//   - 网关模式（默认）：JWT 只在网关验一次——网关验签后把已验证的声明放进请求头
		//     传下来，本服务从请求头取身份，不再自己验签（应用层只查会话存活，所以
		//     「踢下线」仍然立即生效）。这里的守卫只负责提取身份，且只对来自可信网关
		//     的请求生效；没经过网关的请求没有身份，认证环节直接 401。
		//   - jwt 模式（旧路径）：本服务自己验 Bearer JWT；require_jwt 决定是否再叠一层
		//     粗粒度守卫，拦掉绕过网关的直连请求。
		// 豁免清单（login / register / refresh / captcha / oauth 回调）来自 infra 的
		// 暴露端点策略（devops-infra/security/gateway）——同一份策略也用来生成 Higress
		// 的 public ingress，两边不可能再各改一半。
		guard := &httpinfra.GuardHooks{Guards: guards}
		return httpinfra.RunWithOptions(cfg.Server, register, obsRuntime, guard, readiness...)
	},
}
