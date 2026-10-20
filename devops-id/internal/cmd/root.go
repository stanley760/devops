// Package cmd defines the devops-id command-line entrypoint.
//
// Configuration management: the path is bound via -f/--config-file, and loading and
// reading are encapsulated in devops-infra/config -- loading happens in
// cobra.OnInitialize, component initialization in PersistentPreRunE.
package cmd

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/stanley760/devops/devops-id/internal/config"
	infracfg "github.com/stanley760/devops/devops-infra/config"
	"github.com/stanley760/devops/devops-infra/logging"
	"github.com/stanley760/devops/devops-infra/nacos"
	"github.com/stanley760/devops/devops-infra/observability"
	"log/slog"
	"time"
)

var confFile string

var obsRuntime *observability.Runtime

var rootCmd = &cobra.Command{
	Use:   "devops-id",
	Short: "devops distributed ID generation service",

	// Runs before every subcommand: initializes logging
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
			"service", "devops-id",
			"config", infracfg.Files(),
			"env", infracfg.Env(),
			"logLevel", cfg.Log.Level,
		)
		return nil
	},

	// Unified cleanup after every subcommand, ensuring no logs are lost
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

// loadNacosConfig fetches remote Nacos config and merges it, remote taking precedence:
// when a remote dataID hits, the remote value wins; the local same-named environment
// file is only merged as a fallback when the remote profile dataID
// (<service>-<env>.yaml) misses.
// bootstrap.yaml always serves as the shared baseline.
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
		// Remote profile config not found: merge the local <service>-<env>.yaml as a fallback
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

	// cobra.OnInitialize runs before each command's Run; configuration loading happens here
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
