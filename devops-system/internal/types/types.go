// Package types holds cross-layer transfer types (DTOs).
//
// The ID contract (stringified int64 primary keys) has been moved down to
// devops-common/types; this package re-exports them via type aliases, so the
// types.ID / types.ParamID usage in business code is decoupled from where the
// types are defined: migrating the implementation requires zero changes at
// call sites across the repo.
package types

import (
	"errors"
	"time"

	commontypes "github.com/stanley760/devops/devops-common/types"
	"github.com/stanley760/devops/devops-infra/errs"
)

// ID is the DTO form of an int64 primary key: serialized as a string in JSON (defined in devops-common/types).
type ID = commontypes.ID

// ParamID is the string-form ID in path/query (defined in devops-common/types).
type ParamID = commontypes.ParamID

// InvalidIDError indicates an ID parameter parse failure (defined in devops-common/types).
type InvalidIDError = commontypes.InvalidIDError

// DateTime is a timestamp serialized as "YYYY-MM-DD HH:mm:ss" (defined in devops-common/types).
//
// All **output** timestamps use this type so the wire format is uniform across
// every endpoint. Query/body timestamps keep using time.Time: they are parsed,
// not displayed, and RFC3339 remains accepted there for compatibility.
type DateTime = commontypes.DateTime

// NewDateTime wraps a time.Time into the unified output format.
func NewDateTime(t time.Time) DateTime { return commontypes.NewDateTime(t) }

// DateTimePtr wraps a *time.Time into the unified output format (nil stays nil).
func DateTimePtr(t *time.Time) *DateTime { return commontypes.DateTimePtr(t) }

// ParseID parses the ID (string form) in path/query parameters.
//
// Maps the zero-dependency layer's *InvalidIDError to this service's business
// error (400 parameter error), so errs.ToHumaError recognizes it correctly —
// the contract layer must not depend on infrastructure error codes in reverse.
func ParseID(s string) (ID, error) {
	id, err := commontypes.ParseID(s)
	if err != nil {
		return 0, asParamError(err)
	}
	return id, nil
}

// asParamError converts the zero-dependency layer's ID parse error into a CodeParam business error.
func asParamError(err error) error {
	var invalid *commontypes.InvalidIDError
	if errors.As(err, &invalid) {
		return errs.Wrap(err, errs.CodeParam, "invalid ID parameter")
	}
	return err
}

// Parse parses the ID in path/query and maps it to a business error.
//
// ParamID is a type alias, so methods cannot be added to it directly; this
// package-level function provides the same capability. Call sites uniformly
// use types.ParseID(in.ID) or this function.
func Parse(id ParamID) (ID, error) { return ParseID(string(id)) }
