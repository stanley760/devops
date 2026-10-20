// The migrate subcommand: schema SQL + seed data SQL (internal/dbinit).
//
// It shares the same initialization as the server startup chain (dbinit.Run);
// manual migrate is only an operational fallback entry point — normally it
// completes automatically at service startup. Existing tables and their
// seeds are skipped automatically.
//
// Single-database mode: all tables and seed data are written to the
// configured MySQL database.
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	infracfg "github.com/stanley760/devops/devops-infra/config"
	"github.com/stanley760/devops/devops-infra/database/mysql"

	"github.com/stanley760/devops/devops-system/internal/authz"
	"github.com/stanley760/devops/devops-system/internal/config"
	"github.com/stanley760/devops/devops-system/internal/dbinit"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Schema SQL + seed data SQL (existing tables are skipped, idempotent, consistent with server startup)",
	RunE: func(cmd *cobra.Command, args []string) error {
		var cfg config.Config
		if err := infracfg.LoadInto(&cfg); err != nil {
			return err
		}
		if err := mysql.Init(cfg.Mysql); err != nil {
			return err
		}
		gdb := mysql.Get()

		if err := dbinit.Run(gdb); err != nil {
			return fmt.Errorf("schema/seed failed: %w", err)
		}
		if err := authz.Init(gdb); err != nil { // creates the casbin_rule table and loads existing policies
			return err
		}
		defer authz.Close()
		if err := authz.SeedPolicies(1, 1); err != nil { // admin = platform admin + tenant 1 super admin
			return fmt.Errorf("casbin policy seeding failed: %w", err)
		}
		fmt.Println("migrate complete: schema + seed SQL ready (existing tables skipped automatically); casbin_rule + policy seeds ready")
		return nil
	},
}
