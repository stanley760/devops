package dbinit

import "strings"

// splitStatements splits SQL text into an executable statement sequence by semicolons:
//   - semicolons inside single-quoted '…', double-quoted "…", and backtick-quoted `…` literals are not statement boundaries;
//   - MySQL backslash escapes ('\") and adjacent doubled quotes ('it"s') are preserved verbatim inside literals;
//   - -- line comments and /* */ block comments are stripped entirely (including commented-out semicolons);
//   - comment-only/whitespace segments produce no statement.
//
// Dialect boundary: this package targets MySQL (production) and sqlite (tests); PG's
// dollar-quoted strings are not handled — the current scripts contain no such syntax.
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
			i += 2 // skip "*/"; when unclosed, an out-of-range i is caught by the outer condition
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

// copyQuoted copies one quoted literal starting at sql[i] as a whole into b, returning the
// index just after the literal ends. If the quote is unclosed it returns len(sql), and the
// syntax error is left for the database to report.
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
			b.WriteByte(sql[i]) // MySQL backslash escape: include the next character
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
