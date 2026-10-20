package dao

import (
	"errors"
	"regexp"
	"strings"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

// MySQL unique-constraint violation error codes.
const (
	// errDupEntry is the duplicate-key error (ER_DUP_ENTRY), used by single-column unique indexes.
	errDupEntry = 1062
	// errDupKey is the unique-key conflict error (ER_DUP_KEY), used by composite unique indexes.
	errDupKey = 1586
)

// dupKeyIndexRE extracts the conflicting index name from the MySQL error text.
//
// The Message for MySQL 1062 looks like:
//
//	Duplicate entry 'shared@example.com' for key 't_user.uk_email'
//
// so take the identifier inside `for key '...'` and split on `.` to get the
// last segment (the index name).
var dupKeyIndexRE = regexp.MustCompile("for key '([^']+)'")

// duplicateField reports whether the error is a unique-constraint violation and
// returns the conflicting **index name** (uk_username / uk_email / uk_mobile)
// so callers can give an accurate message. It returns an empty string when the
// index name cannot be parsed.
//
// Why not gorm.ErrDuplicatedKey: devops-infra/database/mysql's gorm.Open does
// **not enable TranslateError**, so that error is never translated out. This
// check is kept as forward compatibility (if infra enables translation in the
// future, this branch still hits first).
func duplicateField(err error) (dup bool, indexName string) {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true, ""
	}
	var myErr *mysql.MySQLError
	if !errors.As(err, &myErr) {
		return false, ""
	}
	if myErr.Number != errDupEntry && myErr.Number != errDupKey {
		return false, ""
	}
	if m := dupKeyIndexRE.FindStringSubmatch(myErr.Message); len(m) == 2 {
		if i := strings.LastIndex(m[1], "."); i >= 0 {
			return true, m[1][i+1:]
		}
		return true, m[1]
	}
	return true, ""
}
