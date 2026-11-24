// Package cmd defines the devops-id command-line entrypoint.
//
// Configuration management: the path is bound via -f/--config-file, and loading and
// reading are encapsulated in devops-infra/config -- loading happens in
// cobra.OnInitialize, component initialization in PersistentPreRunE.
package cmd

import (
	"context"
	"log/slog"
	"time"

	"github.com/spf13/cobra"
	"github.com/stanley760/devops/devops-id/internal/config"
	infracfg "github.com/stanley760/devops/devops-infra/config"
	"github.com/stanley760/devops/devops-infra/logging"
	"github.com/stanley760/devops/devops-infra/observability"
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
		if err := loadEnvConfig(&cfg); err != nil {
			return err
		}
		if err := logging.Init(cfg.Log); err != nil {
			return err
		}
		obs, err := observability.Init(cmd.Context(), observability.Config{
			ServiceName:     cfg.Service.Name,
			Env:             infracfg.Env(),
			OTLPEndpoint:    cfg.Observability.OTLPEndpoint,
			SamplerRatio:    cfg.Observability.SamplerRatio,
			MetricsEnabled:  cfg.Observability.MetricsEnabled,
			OTLPLogEndpoint: cfg.Observability.OTLPLogEndpoint,
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

// loadEnvConfig merges the local environment override file <service>-<env>.yaml
// (selected by APP_ENV) and reloads the config struct. bootstrap.yaml always
// serves as the shared baseline; the overlay carries environment-specific
// differences (dev / test / prod).
func loadEnvConfig(cfg *config.Config) error {
	if err := infracfg.MergeEnvConfig(); err != nil {
		return err
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
