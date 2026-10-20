// Package dbinit handles startup table creation and initial data initialization (pure SQL scripts, zero bookkeeping).
//
// Responsibility boundary: table creation and initial data are written as .sql scripts embedded
// in the binary and executed once before startup completes (in the server assembly chain /
// the migrate subcommand).
//
// Script layout (file name stem = table name, must match ^t_[a-z0-9_]+$):
//   - sql/schema/<table>.sql   one CREATE TABLE IF NOT EXISTS per table;
//   - sql/datas/<table>.sql   initial data per table (tables without seeds get no file).
//
// Skip rules:
//   - table creation: skipped entirely if the table already exists (checked via information_schema);
//   - seeds: executed only if the corresponding table was just created by this run (table exists = data already initialized).
//
// Note: DDL script changes do not apply to existing tables — schema changes on an existing DB require manual ALTER.
// Concurrent first-start of multiple instances relies on CREATE TABLE IF NOT EXISTS and the seed
// ON DUPLICATE KEY UPDATE id = id fallback, with no distributed lock introduced.
package dbinit

import (
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"path"
	"regexp"
	"sort"
	"strings"

	"gorm.io/gorm"
)

//go:embed sql/schema sql/datas
var scriptFS embed.FS

// tablePattern is the file name stem convention: the table name.
var tablePattern = regexp.MustCompile(`^t_[a-z0-9_]+$`)

// createTableRE matches the start of a CREATE TABLE statement; capture group 1 is the created table name.
var createTableRE = regexp.MustCompile("(?i)^CREATE TABLE (?:IF NOT EXISTS )?`?([a-z0-9_]+)`?\\s*\\(")

// script is a single SQL script (already split into a statement sequence).
type script struct {
	Table string // file name stem (table name)
	Name  string // full file name
	Stmts []string
}

// Run executes the table creation and initial data scripts against the target DB (see the package doc for details).
func Run(gdb *gorm.DB) error {
	return runFS(gdb, scriptFS)
}

// runFS executes the table creation and seed scripts against the given file system (separate, so tests can inject a MapFS).
func runFS(gdb *gorm.DB, fsys fs.FS) error {
	ddls, err := loadScripts(fsys, "sql/schema", true)
	if err != nil {
		return fmt.Errorf("failed to load table creation scripts: %w", err)
	}
	datas, err := loadScripts(fsys, "sql/datas", false)
	if err != nil {
		return fmt.Errorf("failed to load seed scripts: %w", err)
	}
	ddlTables := make(map[string]bool, len(ddls))
	for _, s := range ddls {
		ddlTables[s.Table] = true
	}
	for _, s := range datas {
		if !ddlTables[s.Table] {
			return fmt.Errorf("seed script %s has no corresponding table creation script sql/schema/%s", s.Name, s.Name)
		}
	}

	created := make(map[string]bool)
	skipped := 0
	for _, s := range ddls {
		exists, err := tableExists(gdb, s.Table)
		if err != nil {
			return err
		}
		if exists {
			skipped++
			slog.Info("table already exists, skipping creation", "table", s.Table, "file", s.Name)
			continue
		}
		for _, stmt := range s.Stmts {
			if err := gdb.Exec(stmt).Error; err != nil {
				return fmt.Errorf("failed to execute table creation script %s: %w", s.Name, err)
			}
		}
		created[s.Table] = true
		slog.Info("table creation executed", "table", s.Table, "file", s.Name)
	}

	seeded := 0
	for _, s := range datas {
		if !created[s.Table] {
			slog.Info("seed skipped (table not created by this run)", "table", s.Table, "file", s.Name)
			continue
		}
		if err := gdb.Transaction(func(tx *gorm.DB) error {
			for _, stmt := range s.Stmts {
				if err := tx.Exec(stmt).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return fmt.Errorf("failed to execute seed script %s: %w", s.Name, err)
		}
		seeded++
		slog.Info("seed data executed", "table", s.Table, "file", s.Name)
	}

	slog.Info("dbinit completed", "created", len(created), "seeded", seeded, "skipped", skipped)
	return nil
}

// loadScripts loads and validates all .sql files under dir (file name stem = table name, sorted by name).
// When isDDL, it validates that the file contains exactly 1 CREATE TABLE and that the table name matches the file name stem.
func loadScripts(fsys fs.FS, dir string, isDDL bool) ([]script, error) {
	paths, err := fs.Glob(fsys, dir+"/*.sql")
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	out := make([]script, 0, len(paths))
	for _, p := range paths {
		base := path.Base(p)
		stem := strings.TrimSuffix(base, ".sql")
		if !tablePattern.MatchString(stem) {
			return nil, fmt.Errorf("file name %s is invalid (must be <table>.sql, with the table name matching ^t_[a-z0-9_]+$)", base)
		}
		content, err := fs.ReadFile(fsys, p)
		if err != nil {
			return nil, err
		}
		stmts := splitStatements(string(content))
		if isDDL {
			if len(stmts) != 1 {
				return nil, fmt.Errorf("table creation script %s must contain exactly 1 CREATE TABLE statement, got %d", base, len(stmts))
			}
			m := createTableRE.FindStringSubmatch(stmts[0])
			if m == nil {
				return nil, fmt.Errorf("the statement in table creation script %s is not CREATE TABLE", base)
			}
			if m[1] != stem {
				return nil, fmt.Errorf("the table %s created by table creation script %s does not match the file name stem", m[1], base)
			}
		} else if len(stmts) == 0 {
			return nil, fmt.Errorf("seed script %s has no valid statements", base)
		}
		out = append(out, script{Table: stem, Name: base, Stmts: stmts})
	}
	return out, nil
}

// tableExists checks whether a table exists: MySQL queries information_schema (works for
// MySQL/MariaDB alike); other dialects (sqlite in tests) query sqlite_master.
func tableExists(gdb *gorm.DB, table string) (bool, error) {
	var cnt int64
	var err error
	if gdb.Dialector.Name() == "mysql" {
		err = gdb.Raw(
			"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?",
			table,
		).Scan(&cnt).Error
	} else {
		err = gdb.Raw(
			"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?",
			table,
		).Scan(&cnt).Error
	}
	if err != nil {
		return false, fmt.Errorf("failed to check whether table %s exists: %w", table, err)
	}
	return cnt > 0, nil
}
