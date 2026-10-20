// Package dbinit creates tables at startup (pure SQL scripts, zero
// bookkeeping).
//
// Differences from devops-system/internal/dbinit:
//   - **no seed data**: C-side users register themselves, so no accounts are
//     pre-created; only sql/schema is needed. There is no sql/datas directory,
//     and hence no "seeds run only for newly created tables" logic.
//   - Likewise it does not reuse system's implementation across modules —
//     that is an internal package and cannot be referenced by devops-user.
//
// Skip rule: if the table already exists (checked via information_schema), the
// whole script is skipped.
//
// ⚠️ DDL script changes do not apply to existing tables — schema changes on
// existing databases require manual ALTER. Concurrent first-start of multiple
// instances is covered by CREATE TABLE IF NOT EXISTS, with no distributed
// lock.
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

//go:embed sql/schema
var scriptFS embed.FS

// tablePattern is the file stem convention: the table name.
var tablePattern = regexp.MustCompile(`^t_[a-z0-9_]+$`)

// createTableRE matches the start of a CREATE TABLE statement: capture group 1
// is the created table name.
var createTableRE = regexp.MustCompile("(?i)^CREATE TABLE (?:IF NOT EXISTS )?`?([a-z0-9_]+)`?\\s*\\(")

// script is a single table-creation script (a sequence of split statements).
type script struct {
	Table string // file stem (table name)
	Name  string // full file name
	Stmts []string
}

// Run executes the table-creation scripts against the target database.
func Run(gdb *gorm.DB) error {
	return runFS(gdb, scriptFS)
}

// runFS executes the table-creation scripts against a given file system
// (separated so unit tests can inject a MapFS).
func runFS(gdb *gorm.DB, fsys fs.FS) error {
	ddls, err := loadScripts(fsys)
	if err != nil {
		return err
	}

	created := 0
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
				return fmt.Errorf("executing schema script %s failed: %w", s.Name, err)
			}
		}
		created++
		slog.Info("schema script executed", "table", s.Table, "file", s.Name)
	}

	slog.Info("dbinit complete", "created", created, "skipped", skipped)
	return nil
}

// loadScripts loads and validates all .sql files under sql/schema (file stem =
// table name, sorted by name). It validates that each script contains exactly
// 1 CREATE TABLE and that the table name matches the file stem.
func loadScripts(fsys fs.FS) ([]script, error) {
	paths, err := fs.Glob(fsys, "sql/schema/*.sql")
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	out := make([]script, 0, len(paths))
	for _, p := range paths {
		base := path.Base(p)
		stem := strings.TrimSuffix(base, ".sql")
		if !tablePattern.MatchString(stem) {
			return nil, fmt.Errorf("invalid file name %s (must be <table>.sql with the table matching ^t_[a-z0-9_]+$)", base)
		}
		content, err := fs.ReadFile(fsys, p)
		if err != nil {
			return nil, err
		}
		stmts := splitStatements(string(content))
		if len(stmts) != 1 {
			return nil, fmt.Errorf("schema script %s must contain exactly 1 CREATE TABLE statement, got %d", base, len(stmts))
		}
		m := createTableRE.FindStringSubmatch(stmts[0])
		if m == nil {
			return nil, fmt.Errorf("statement in schema script %s is not a CREATE TABLE", base)
		}
		if m[1] != stem {
			return nil, fmt.Errorf("table %s created by schema script %s does not match the file stem", m[1], base)
		}
		out = append(out, script{Table: stem, Name: base, Stmts: stmts})
	}
	return out, nil
}

// tableExists checks whether a table already exists: MySQL queries
// information_schema; other dialects (unit-test sqlite) query sqlite_master.
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
		return false, fmt.Errorf("checking whether table %s exists failed: %w", table, err)
	}
	return cnt > 0, nil
}

// splitStatements splits SQL text into an executable statement sequence on
// semicolons:
//   - semicolons inside single-quoted '…', double-quoted "…", and backtick `…`
//     literals are not statement boundaries;
//   - MySQL backslash escapes ('\') and adjacent doubled quotes ('it”s') are
//     preserved verbatim inside literals;
//   - -- line comments and /* */ block comments are stripped wholesale
//     (including commented-out semicolons);
//   - pure comment/whitespace segments produce no statements.
//
// Dialect boundary: targets MySQL (production) and sqlite (unit tests); PG
// dollar-quoted strings are not handled.
func splitStatements(sql string) []string {
	var stmts []string
	var b strings.Builder
	flush := func() {
		if s := strings.TrimSpace(b.String()); s != "" {
			stmts = append(stmts, s)
		}
		b.Reset()
	}
	i, n := 0, len(sql)
	for i < n {
		switch c := sql[i]; {
		case c == '\'' || c == '"' || c == '`':
			i = copyQuoted(sql, i, &b)
		case c == '-' && i+1 < n && sql[i+1] == '-':
			for i < n && sql[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && sql[i+1] == '*':
			i += 2
			for i+1 < n && !(sql[i] == '*' && sql[i+1] == '/') {
				i++
			}
			i += 2 // skip "*/"; when unclosed, i going out of range is caught by the outer condition
		case c == ';':
			flush()
			i++
		default:
			b.WriteByte(c)
			i++
		}
	}
	flush()
	return stmts
}

// copyQuoted copies a whole quoted literal starting at sql[i] into b,
// returning the index after the literal ends. When the quote is unclosed it
// returns len(sql), letting the database report the syntax error.
func copyQuoted(sql string, i int, b *strings.Builder) int {
	q := sql[i]
	b.WriteByte(q)
	i++
	for i < len(sql) {
		c := sql[i]
		b.WriteByte(c)
		i++
		switch {
		case c == '\\' && q != '`' && i < len(sql):
			b.WriteByte(sql[i]) // MySQL backslash escape: take the next character too
			i++
		case c == q && i < len(sql) && sql[i] == q:
			b.WriteByte(q) // adjacent doubled quotes ('') are an in-literal escape
			i++
		case c == q:
			return i
		}
	}
	return i
}
