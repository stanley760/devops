// Package errs defines unified error codes and the business error type.
//
// The data foundation of unified error handling:
//   - Error codes share the HTTP status space: 200 success, 400/401/403/404/500 failures
//   - Business errors uniformly use *BizError to carry a code and a user-readable msg, and can wrap an underlying error
//   - handler / service report failures by returning error,
//     which response.FailErr recognizes as BizError and converts into a unified response
package errs

import (
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// Error code constants, aligned with HTTP status codes.
const (
	CodeOK               = 200 // success
	CodeParam            = 400 // invalid parameter
	CodeUnauthorized     = 401 // not logged in or session expired
	CodeForbidden        = 403 // no operation permission
	CodeNotFound         = 404 // resource not found
	CodeMethodNotAllowed = 405 // request method not allowed
	CodeTooManyRequests  = 429 // too many requests (rate limited)
	CodeInternal         = 500 // internal server error
)

// BizError is a business error.
// Code targets the response code and frontend branching, Msg targets user display, Err keeps the underlying cause for log triage,
// and stack is the call stack at the error's creation point (captured in New/Wrap); logs use Stack() to locate where the error occurred.
type BizError struct {
	Code  int
	Msg   string
	Err   error
	stack []uintptr
}

// Error implements the error interface.
func (e *BizError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("biz error: code=%d msg=%q: %v", e.Code, e.Msg, e.Err)
	}
	return fmt.Sprintf("biz error: code=%d msg=%q", e.Code, e.Msg)
}

// Unwrap supports errors.Is / errors.As unwrapping down to the underlying error.
func (e *BizError) Unwrap() error { return e.Err }

// Stack renders the call stack at the error's creation point (two lines per frame: function name + file:line).
// It returns an empty string for a zero-value construction or when not captured (theoretically should not happen); callers omit the log field accordingly.
func (e *BizError) Stack() string {
	return formatFrames(e.stack)
}

// New creates a business error.
func New(code int, msg string) *BizError {
	return &BizError{Code: code, Msg: msg, stack: callers()}
}

// Wrap attaches business semantics on top of an underlying error, keeping the original error for logs.
func Wrap(err error, code int, msg string) *BizError {
	return &BizError{Code: code, Msg: msg, Err: err, stack: callers()}
}

// callers captures the call stack, skipping the errs package's own frames (runtime.Callers/callers/New/Wrap),
// so the stack top lands on the business code's error creation point.
func callers() []uintptr {
	pcs := make([]uintptr, 32)
	n := runtime.Callers(3, pcs)
	return pcs[:n]
}

// formatFrames renders the call stack (two lines per frame: function name + file:line); an empty stack returns an empty string.
func formatFrames(pcs []uintptr) string {
	if len(pcs) == 0 {
		return ""
	}
	var b strings.Builder
	frames := runtime.CallersFrames(pcs)
	for {
		frame, more := frames.Next()
		fmt.Fprintf(&b, "%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line)
		if !more {
			break
		}
	}
	return b.String()
}

// ToHumaError converts a unified business error into a Huma Problem Details error.
//
// Only the user-facing BizError.Msg is exposed; the underlying Err never enters the response, avoiding leaks of sensitive
// information such as database credentials, SQL, and internal addresses; non-BizError is always treated as an internal error.
//
// While the response hides details, server logs must keep the full error chain and the occurrence-point stack:
// non-BizError and 5xx BizError are logged at Error level here (the only error log outlet on the huma path);
// 4xx are expected business rejections and are not logged (the access log already has Debug records).
func ToHumaError(err error) error {
	if err == nil {
		return nil
	}

	var bizErr *BizError
	if !errors.As(err, &bizErr) {
		slog.Error("unclassified error (non-BizError, responding 500)",
			"err", err, "stack", formatFrames(callers()))
		return huma.Error500InternalServerError("internal server error")
	}

	if bizErr.Code >= 500 {
		slog.Error("business error (5xx)",
			"code", bizErr.Code, "msg", bizErr.Msg, "err", err, "stack", bizErr.Stack())
	}

	status := bizErr.Code
	if status < 400 || status > 599 {
		status = CodeInternal
	}
	message := bizErr.Msg
	if message == "" {
		message = "internal server error"
	}
	return huma.NewError(status, message)
}

// Common predefined errors, returned directly or compared with errors.Is.
var (
	ErrParam            = New(CodeParam, "invalid parameter")
	ErrUnauthorized     = New(CodeUnauthorized, "not logged in or session expired")
	ErrForbidden        = New(CodeForbidden, "no operation permission")
	ErrNotFound         = New(CodeNotFound, "resource not found")
	ErrMethodNotAllowed = New(CodeMethodNotAllowed, "request method not allowed")
	ErrTooManyRequests  = New(CodeTooManyRequests, "too many requests, please try again later")
	ErrInternal         = New(CodeInternal, "internal server error")
)
